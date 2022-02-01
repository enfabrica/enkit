package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tpb "github.com/enfabrica/enkit/bestie/proto"
	kbb "github.com/enfabrica/enkit/lib/kbuildbarn"
	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages
	"google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/protobuf/proto"
)

// Base URL to use for reading bytestream:// artifacts from cluster builds.
var deploymentBaseUrl string

// Pertinent fields to uniquely identify a Bazel event stream.
type bazelStream struct {
	buildId       string
	invocationId  string
	testName      string
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
func identifyStream(bazelBuildEvent bes.BuildEvent, streamId *build.StreamId) bazelStream {
	// Extract the stream identifier fields of interest.
	stream := bazelStream{
		buildId:      streamId.GetBuildId(),
		invocationId: streamId.GetInvocationId(),
		run:          strconv.Itoa(int(bazelBuildEvent.GetId().GetTestResult().GetRun())),
		testName:     bazelBuildEvent.GetId().GetTestResult().GetLabel(),
	}
	// Calculate a SHA256 hash using the following fields to uniquely identify this stream.
	stream.invocationSha = deriveInvocationSha([]string{stream.invocationId, stream.buildId, stream.run})
	return stream
}

// Read the contents of a bytestream file.
func readBytestreamFile(fileName, bytestreamUri string) ([]byte, error) {
	if len(deploymentBaseUrl) == 0 {
		return nil, fmt.Errorf("base URL not specified")
	}

	hash, size, err := kbb.ParseByteStreamUrl(bytestreamUri)
	if err != nil {
		return nil, err
	}
	fileUrl := kbb.Url(deploymentBaseUrl, hash, size, kbb.WithFileName(fileName))

	client := http.DefaultClient
	resp, err := client.Get(fileUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respStatus := resp.StatusCode

	// Read response body regardless of response status
	// so it can be used in the HTTP error message below.
	respBody, readErr := ioutil.ReadAll(resp.Body)

	if respStatus != http.StatusOK {
		return nil, fmt.Errorf("HTTP error status %d: response: %s", respStatus, respBody)
	}
	if readErr != nil {
		return nil, fmt.Errorf("%s file read error: %s", fileName, readErr)
	}

	return respBody, nil
}

// Handle metrics extraction from the TestResult event.
func handleTestResultEvent(bazelBuildEvent bes.BuildEvent, streamId *build.StreamId) error {
	stream := identifyStream(bazelBuildEvent, streamId)
	m := bazelBuildEvent.GetTestResult()
	if m == nil {
		return fmt.Errorf("Error extracting TestResult data from event message")
	}

	fmt.Printf("TestResult for %s: %s\n", stream.testName, m.GetStatus())
	fmt.Printf("  run: %s\n", stream.run)
	fmt.Printf("  buildId: %s\n", stream.buildId)
	fmt.Printf("  invocationId: %s\n", stream.invocationId)
	fmt.Printf("  invocationSha: %s\n\n", stream.invocationSha)

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

	var fileBytes []byte = []byte{}
	var readErr error = nil
	urlParts := strings.Split(outFileUri, "://")
	if len(urlParts) == 2 {
		scheme, fileRef := urlParts[0], urlParts[1]
		switch scheme {
		case "bytestream":
			// Handle cluster build scenario, translating bytestream:// URL to file URL for http.Get().
			fileBytes, readErr = readBytestreamFile(outFileName, outFileUri)
		case "file":
			// Use URI without file:// prefix to access the local file system path.
			fileBytes, readErr = ioutil.ReadFile(fileRef)
		default:
			// Silently ignore file: not a supported URL scheme prefix.
			return nil
		}
	}
	if readErr != nil {
		// Attempt to read the zip file failed.
		return fmt.Errorf("Error reading %s file: %s", outFileName, readErr)
	}
	if len(fileBytes) == 0 {
		// Something went wrong reading zip file -- no data.
		return fmt.Errorf("Error reading %s file: no data", outFileName)
	}
	// Process test metrics output file(s).
	// Each output file contains a single (potentially large) protobuf message.
	// For now, it's up to the client to split large metric datasets into multiple .metrics.pb files.
	if err := extractZippedFiles(stream, fileBytes); err != nil {
		return fmt.Errorf("Error processing %s file: %s", outFileName, err)
	}

	return nil
}

// Look for and return a []byte slice with its contents.
func extractZippedFiles(stream bazelStream, fileBytes []byte) error {
	zipReader, err := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
	if err != nil {
		return err
	}

	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		//  Look for any file named *.metrics.pb.
		if !strings.HasSuffix(filepath.Base(file.Name), ".metrics.pb") {
			continue
		}

		// TODO (PR-394): May need to chunk the .pb file into multiple messages, or enforce a max metrics file size.
		// Read entire file contents into a byte slice.
		f, err := file.Open()
		if err != nil {
			continue
		}
		data, err := ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			continue
		}
		fmt.Printf("Found output file to process: %s\n", file.Name)

		// Extract all metrics from the file contents.
		pResult, err := getTestMetricsFromFileData(data)
		if err != nil {
			fmt.Printf("Error processing output file %s: %s\n\n", file.Name, err)
			continue
		}
		if pResult != nil {
			// Display the metric data on the console.
			displayTestMetrics(pResult, 2)

			// Upload test metrics to BigQuery database table.
			if err := uploadTestMetrics(os.Stdout, stream, pResult); err != nil {
				fmt.Printf("Error uploading metrics to BigQuery: %s\n", err)
			}
		}
	}
	return nil
}

// Extract the test metrics information from the protobuf message data read
// from an output file. This is used for processing *.metrics.pb output files.
func getTestMetricsFromFileData(pbmsg []byte) (*metricTestResult, error) {
	tmet := &tpb.TestMetrics{}
	if err := proto.Unmarshal(pbmsg, tmet); err != nil {
		ServiceStats.incrementBigQueryProtobufError()
		return nil, fmt.Errorf("Error extracting metric data from protobuf message: %s", err)
	}
	table := tmet.GetTable()
	metrics := tmet.GetMetrics()
	var result metricTestResult
	if table != nil {
		result.table = bigQueryTable{
			// The project is not defined by the protobuf message.
			dataset:   table.GetDataset(),
			tablename: table.GetTablename(),
		}
	}
	for _, metric := range metrics {
		m := testMetric{
			metricname: metric.GetMetricname(),
			value:      metric.GetValue(),
			timestamp:  metric.GetTimestamp(),
		}
		for _, mtag := range metric.GetTags() {
			m.tags = append(m.tags, keyValuePair{mtag.GetKey(), mtag.GetValue()})
		}
		result.metrics = append(result.metrics, m)
	}
	return &result, nil
}

// Print the test metric data using a starting indentation offset.
func displayTestMetrics(res *metricTestResult, offset int) {
	tableId := "<using default>"
	if len(res.table.dataset) != 0 || len(res.table.tablename) != 0 {
		tableId = fmt.Sprintf("%s.%s", res.table.dataset, res.table.tablename)
	}
	fmt.Printf("%*sBigQuery Table: %s\n", offset, "", tableId)
	for _, m := range res.metrics {
		fmt.Printf("%*sMetric: metricname=%s value=%f timestamp=%d tags=%s\n",
			offset*2, "", m.metricname, m.value, m.timestamp, m.tags)
	}
	fmt.Println()
}
