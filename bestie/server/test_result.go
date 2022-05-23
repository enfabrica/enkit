package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
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

func readFileWithLimit(fileReader io.Reader, limit int) ([]byte, error) {
	// Attempt to read the file contents all at once, bounded by
	// the specified limit. This uses a LimitReader to restrict the number
	// of bytes read by ReadAll, producing an EOF when the limit is reached.
	// Note that ReadAll treats EOF as a normal condition and does not
	// report it as an error. Adding 1 to the limit to detect if the file size
	// went over it using a single read.
	limitReader := io.LimitReader(fileReader, int64(limit+1))
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("File read error: %w", err)
	}
	// A successful ReadAll here means either EOF occurred due
	// to the file being within the limit, or the LimitReader kicked in.
	// Since an extra byte was added to the limit above, check
	// if the data length exceeds the limit (it will be by one byte
	// in this case).
	if len(data) > limit {
		return nil, fileTooBigErr
	}
	return data, nil
}

// Open an output file for reading.
func openOutputFile(fileName, fileUri string) (io.ReadCloser, error) {
	u, err := url.Parse(fileUri)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s file: malformed URL: %s", fileName, fileUri)
	}

	var fileCloser io.ReadCloser
	var readErr error = nil
	switch u.Scheme {
	case "bytestream":
		// Handle cluster build scenario, translating bytestream:// URL to file URL for http.Get().
		fileCloser, readErr = openBytestreamFile(fileName, fileUri)
	case "file":
		// Use URI without file:// prefix to access the local file system path.
		fileCloser, readErr = openLocalFile(u.Path)
	default:
		// log and ignore this file: not a supported URL scheme prefix.
		readErr = fmt.Errorf("Unsupported URI scheme: %s", fileUri)
	}
	if readErr != nil {
		// Attempt to read the zip file failed.
		return nil, fmt.Errorf("Error reading file %q: %w", fileName, readErr)
	}
	debugPrintf("Opened output file %q for processing\n", fileName)
	return fileCloser, nil
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

	var errs []error
	var fileName, fileUri string
	for _, of := range m.GetTestActionOutput() {
		fileName = of.GetName()
		fileUri = of.GetUri()

		var fileCloser io.ReadCloser
		var err error
		switch {
		case strings.HasSuffix(fileName, "outputs.zip"):
			fileCloser, err = openOutputFile(fileName, fileUri)
			if err != nil {
				break
			}
			defer fileCloser.Close()
			err = processZipMetrics(stream, fileCloser)
		case strings.HasSuffix(fileName, "test.xml"):
			fileCloser, err = openOutputFile(fileName, fileUri)
			if err != nil {
				break
			}
			defer fileCloser.Close()
			err = processXmlMetrics(stream, fileCloser, fileName)
		default:
			continue
		}

		// Check for file open or processing error.
		if err != nil {
			errs = append(errs, fmt.Errorf("%w", err))
			continue
		}
	}
	if len(errs) > 0 {
		// Display any errors that occurred, but don't fail the event processing
		for _, err := range errs {
			debugPrintln(err)
		}
	}
	return nil
}

// Use zipstream package to process zip files one-by-one without
// first reading entire zip file contents into memory.
func processZipMetrics(stream *bazelStream, fileReader io.Reader) error {
	zr := zipstream.NewReader(fileReader)

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

		// Check for a supported file type(s).
		fileName := filepath.Clean(meta.Name)
		baseName := filepath.Base(fileName)
		if !strings.HasSuffix(baseName, ".metrics.pb") {
			continue
		}

		// Read entire file contents into a byte slice.
		// Each output file contains a single (potentially large) protobuf message.
		// For now, it's up to the client to split large metric datasets into multiple
		// *.metrics.pb files.
		//
		// According to the protobuf documentation, a single .pb message is not designed
		// to be read in chunks. Protobuf does work with large message sizes so there
		// is no attempt to split it, which would require a "custom" framing technique
		// (e.g. 4-byte length prefixing) by both the sender and receiver.
		fileData, err := readFileWithLimit(zr, maxFileSize)
		if err != nil {
			if errors.Is(err, fileTooBigErr) {
				cidOutputFileTooBigTotal.increment()
			}
			errs = append(errs, fmt.Errorf("Error reading file %q: %w", fileName, err))
			continue
		}
		debugPrintf("Read output file to process: %s\n", fileName)

		// Extract all metrics from the protobuf file contents.
		pResult, err := getTestMetricsFromProtobufData(fileData)
		if err != nil {
			errs = append(errs, fmt.Errorf("Error extracting protobuf metrics from file %q: %w", fileName, err))
			continue
		}

		// Send the metrics to BigQuery.
		if err := processMetrics(stream, pResult); err != nil {
			errs = append(errs, fmt.Errorf("Error processing output file %q: %w", fileName, err))
			continue
		}
	}

	if len(errs) > 0 {
		return multierror.New(errs)
	}
	return nil
}

// Extract the test metrics information from the protobuf message data read
// from an output file. This is used for processing *.metrics.pb output files.
func getTestMetricsFromProtobufData(pbmsg []byte) (*metricTestResult, error) {
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

	// Process each of the metrics contained in the protobuf message.
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

// Process the raw metrics data to store into BigQuery.
func processMetrics(stream *bazelStream, pResult *metricTestResult) error {
	// Display the metric data on the console.
	displayTestMetrics(pResult, 2)

	// Upload test metrics to BigQuery database table.
	if err := uploadTestMetrics(stream, pResult); err != nil {
		return fmt.Errorf("Error uploading XML metrics to BigQuery: %w", err)
	}

	return nil
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
