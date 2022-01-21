package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	tpb "github.com/enfabrica/enkit/bestie/proto"
	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages
	"google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/protobuf/proto"
)

type bigQueryTable struct {
	project   string
	dataset   string
	tablename string
}

type keyValuePair struct {
	key   string
	value string
}

type testMetric struct {
	metricname string
	tags       []keyValuePair
	value      float64
	timestamp  int64
}

type metricTestResult struct {
	table   bigQueryTable
	metrics []testMetric
}

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

	outputFiles := m.GetTestActionOutput()
	var ofname, ofuri string
	found := false
	for _, of := range outputFiles {
		ofname = of.GetName()
		ofuri = of.GetUri()
		if strings.HasSuffix(ofname, "outputs.zip") {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	// Strip off any file:// prefix from the URI to access the local file system path.
	// TODO (PR-394): Handle similar adjustment for Cloud Run URI.
	filePrefix := "file://"
	if strings.HasPrefix(ofuri, filePrefix) {
		ofuri = ofuri[len(filePrefix):]
	}

	// Process test metrics output file(s).
	// Each output file contains a single (potentially large) protobuf message.
	if err := extractZippedFiles(stream, ofuri); err != nil {
		return fmt.Errorf("Error processing %s file: %s", ofname, err)
	}

	return nil
}

// Look for and return a []byte slice with its contents.
func extractZippedFiles(stream bazelStream, zipFile string) error {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		//  Look for any file named *.metrics.pb.
		if !strings.HasSuffix(filepath.Base(file.Name), ".metrics.pb") {
			continue
		}
		fmt.Printf("Found output file to process: %s\n", file.Name)

		f, err := file.Open()
		if err != nil {
			return err
		}
		defer f.Close()

		// Read entire file contents into a byte slice.
		// TODO (PR-394): Need to chunk the .pb file into multiple messages, or enforce a max metrics file size.
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		// Extract all metrics from the file contents.
		pResult, err := getTestMetricsFromFileData(data)
		if err != nil {
			fmt.Printf("Error processing output file %s: %s\n\n", file.Name, err)
			continue
		}
		if pResult != nil {
			// Display the metric data on the console.
			displayTestMetrics(pResult, 2)
			// TODO: Upload test metrics to BigQuery database table.
		}
	}
	return nil
}

// Extract the test metrics information from the protobuf message data read
// from an output file. This is used for processing *.metrics.pb output files.
func getTestMetricsFromFileData(pbmsg []byte) (*metricTestResult, error) {
	tmet := &tpb.TestMetrics{}
	if err := proto.Unmarshal(pbmsg, tmet); err != nil {
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
