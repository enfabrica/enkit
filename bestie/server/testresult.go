package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tpb "github.com/enfabrica/enkit/bestie/proto"
	"github.com/enfabrica/enkit/lib/kbuildbarn"
	"github.com/enfabrica/enkit/lib/multierror"
	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages
	"github.com/xenking/zipstream"
	"google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/protobuf/proto"
)

// Base URL to use for reading bytestream:// artifacts from cluster builds.
var deploymentBaseUrl string

// Pertinent fields to uniquely identify a Bazel event stream.
type bazelStream struct {
	buildId       string
	invocationId  string
	testTarget    string
	run           string
	invocationSha string // derived
}

// Derive a unique invocation SHA value.
func deriveInvocationSha(items []string) string {
	// The SHA value allows combining one or more items
	// (e.g. invocationId, buildId, run) to uniquely identify
	// a bazel test stream without impacting the database schema.
	hash := sha256.Sum256([]byte(strings.Join(items[:], ".")))
	return hex.EncodeToString(hash[:])
}

// Store fields that help identify this Bazel stream.
func identifyStream(bazelBuildEvent bes.BuildEvent, streamId *build.StreamId) *bazelStream {
	// Extract the stream identifier fields of interest.
	stream := bazelStream{
		buildId:      streamId.GetBuildId(),
		invocationId: streamId.GetInvocationId(),
		run:          strconv.Itoa(int(bazelBuildEvent.GetId().GetTestResult().GetRun())),
		testTarget:   bazelBuildEvent.GetId().GetTestResult().GetLabel(),
	}
	// Calculate a SHA256 hash using the following fields to uniquely identify this stream.
	stream.invocationSha = deriveInvocationSha([]string{stream.invocationId, stream.buildId, stream.run})
	return &stream
}

// Open a bytestream file.
func openBytestreamFile(fileName, bytestreamUri string) (io.ReadCloser, error) {
	if len(deploymentBaseUrl) == 0 {
		return nil, fmt.Errorf("base URL not specified")
	}

	hash, size, err := kbuildbarn.ParseByteStreamUrl(bytestreamUri)
	if err != nil {
		return nil, err
	}
	fileUrl := kbuildbarn.Url(deploymentBaseUrl, hash, size, kbuildbarn.WithFileName(fileName))

	client := http.DefaultClient
	resp, err := client.Get(fileUrl)
	if err != nil {
		return nil, err
	}
	respStatus := resp.StatusCode
	if respStatus != http.StatusOK {
		return nil, fmt.Errorf("HTTP error status %d", respStatus)
	}
	return resp.Body, nil
}

// Open a local file.
func openLocalFile(file string) (io.ReadCloser, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("Error opening %s: %w", file, err)
	}
	return ioutil.NopCloser(f), nil
}

// Handle metrics extraction from the TestResult event.
func handleTestResultEvent(bazelBuildEvent bes.BuildEvent, streamId *build.StreamId) error {
	stream := identifyStream(bazelBuildEvent, streamId)
	m := bazelBuildEvent.GetTestResult()
	if m == nil {
		return fmt.Errorf("Error extracting TestResult data from event message")
	}

	var sbuf strings.Builder
	sbuf.WriteString(fmt.Sprintf("\nTestResult for %s: %s\n", stream.testTarget, m.GetStatus()))
	sbuf.WriteString(fmt.Sprintf("\trun: %s\n", stream.run))
	sbuf.WriteString(fmt.Sprintf("\tbuildId: %s\n", stream.buildId))
	sbuf.WriteString(fmt.Sprintf("\tinvocationId: %s\n", stream.invocationId))
	sbuf.WriteString(fmt.Sprintf("\tinvocationSha: %s\n", stream.invocationSha))
	debugPrintln(sbuf.String())

	var outFileName, outFileUri string
	for _, of := range m.GetTestActionOutput() {
		if strings.HasSuffix(of.GetName(), "outputs.zip") {
			outFileName = of.GetName()
			outFileUri = of.GetUri()
			break
		}
	}
	if len(outFileName) == 0 {
		// The outputs.zip file was not found.
		return nil
	}

	var zipFileCloser io.ReadCloser
	var readErr error = nil
	urlParts := strings.Split(outFileUri, "://")
	if len(urlParts) != 2 {
		return fmt.Errorf("Error reading %s file: malformed URL: %s", outFileName, outFileUri)
	}
	scheme, fileRef := urlParts[0], urlParts[1]
	switch scheme {
	case "bytestream":
		// Handle cluster build scenario, translating bytestream:// URL to file URL for http.Get().
		zipFileCloser, readErr = openBytestreamFile(outFileName, outFileUri)
	case "file":
		// Use URI without file:// prefix to access the local file system path.
		zipFileCloser, readErr = openLocalFile(fileRef)
	default:
		// log and ignore this file: not a supported URL scheme prefix.
		logger.Printf("Unexpected URI scheme when processing %s file: %s", outFileName, outFileUri)
		return nil
	}
	if readErr != nil {
		// Attempt to read the zip file failed.
		return fmt.Errorf("Error reading %s file: %w", outFileName, readErr)
	}
	defer zipFileCloser.Close()

	// Process test metrics output file(s).
	// Each output file contains a single (potentially large) protobuf message.
	// For now, it's up to the client to split large metric datasets into multiple .metrics.pb files.
	if err := processZip(stream, zipFileCloser); err != nil {
		return fmt.Errorf("Error processing %s file: %w", outFileName, err)
	}

	return nil
}

// Use zipstream package to process zip files one-by-one without
// first reading entire zip file contents into memory.
func processZip(stream *bazelStream, zipFile io.Reader) error {
	zr := zipstream.NewReader(zipFile)

	// Accumulate any errors from processing each file within the zip file.
	var errs []error

	// Read each compressed file from the zip file.
	for {
		meta, err := zr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			errs = append(errs, fmt.Errorf("Error accessing zipped file: %w", err))
			continue
		}

		//  Look for any file named *.metrics.pb.
		fileName := filepath.Clean(meta.Name)
		if !strings.HasSuffix(filepath.Base(fileName), ".metrics.pb") {
			continue
		}

		// Read entire .metrics.pb file contents into a byte slice.
		//
		// According to the protobuf documentation, a single .pb message is not designed
		// to be read in chunks. Protobuf does work with large message sizes so there
		// is no attempt to split it, which would require a custom framing technique
		// (e.g. 4-byte length prefixing) by both the sender and receiver.
		//
		// A test client (message sender) does have the option of presenting multiple
		// *.metrics.pb files in outputs.zip, if needed, which is supported by this endpoint.
		pbCompressedData, err := ioutil.ReadAll(zr)
		if err != nil {
			errs = append(errs, fmt.Errorf("Error reading output file %q: %w", fileName, err))
			continue
		}
		debugPrintf("Read output file to process: %s\n", fileName)

		// Extract all metrics from the file contents.
		if err := processMetricsProtobufFile(stream, pbCompressedData[:]); err != nil {
			errs = append(errs, fmt.Errorf("Error processing output file %q: %w", fileName, err))
			continue
		}
	}

	if len(errs) > 0 {
		return multierror.New(errs)
	}
	return nil
}

func processMetricsProtobufFile(stream *bazelStream, pbMsg []byte) error {
	pResult, err := getTestMetricsFromFileData(pbMsg)
	if err != nil {
		return fmt.Errorf("Error extracting protobuf metrics: %w", err)
	}

	// Display the metric data on the console.
	displayTestMetrics(pResult, 2)

	// Upload test metrics to BigQuery database table.
	if err := uploadTestMetrics(stream, pResult); err != nil {
		return fmt.Errorf("Error uploading metrics to BigQuery: %w", err)
	}

	return nil
}

// Extract the test metrics information from the protobuf message data read
// from an output file. This is used for processing *.metrics.pb output files.
func getTestMetricsFromFileData(pbmsg []byte) (*metricTestResult, error) {
	tmet := &tpb.TestMetrics{}
	if err := proto.Unmarshal(pbmsg, tmet); err != nil {
		cidExceptionProtobufError.increment()
		return nil, fmt.Errorf("Error extracting metric data from protobuf message: %w", err)
	}
	table := tmet.GetTable()
	metrics := tmet.GetMetrics()
	var result metricTestResult
	if table != nil {
		result.table = bigQueryTable{
			// The project is not defined by the protobuf message.
			dataset:   table.GetDataset(),
			tableName: table.GetTablename(),
		}
	}
	for _, metric := range metrics {
		m := testMetric{
			metricName: metric.GetMetricname(),
			tags:       map[string]string{},
			value:      metric.GetValue(),
			timestamp:  metric.GetTimestamp(),
		}
		for _, mtag := range metric.GetTags() {
			k, v := mtag.GetKey(), mtag.GetValue()
			if len(k) != 0 {
				m.tags[k] = v
			}
		}
		result.metrics = append(result.metrics, m)
	}
	return &result, nil
}

// Print the test metric data using a starting indentation offset.
func displayTestMetrics(res *metricTestResult, offset int) {
	// Nothing to do when debug mode is disabled.
	if !isDebugMode {
		return
	}
	tableId := "<using default>"
	if len(res.table.dataset) != 0 || len(res.table.tableName) != 0 {
		tableId = fmt.Sprintf("%s.%s", res.table.dataset, res.table.tableName)
	}
	var sbuf strings.Builder
	sbuf.WriteString(fmt.Sprintf("\n%*sBigQuery Table: %s\n", offset, "", tableId))
	for _, m := range res.metrics {
		sbuf.WriteString(fmt.Sprintf("%*sMetric: metricname=%s value=%f timestamp=%d tags=%s\n",
			offset*2, "", m.metricName, m.value, m.timestamp, m.tags))
	}
	sbuf.WriteString("\n")
	debugPrintln(sbuf.String())
}
