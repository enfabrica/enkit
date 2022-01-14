package main

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	tpb "github.com/enfabrica/enkit/bestie/proto"
	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages
	"google.golang.org/protobuf/proto"
)

type bigQueryTable struct {
	project   string
	dataset   string
	tablename string
}

type testMetric struct {
	metricname string
	tags       string
	value      float64
	timestamp  int64
}

type metricTestResult struct {
	table   bigQueryTable
	metrics []testMetric
}

// Handle metrics extraction from the TestResult event.
func handleTestResultEvent(bazelBuildEvent bes.BuildEvent, invocationId, invocationSha string) error {
	m := bazelBuildEvent.GetTestResult()
	if m == nil {
		return fmt.Errorf("Error extracting TestResult data from event message")
	}
	// Get the 'bazel test' target name from the bazel build event id label.
	testname := bazelBuildEvent.GetId().GetTestResult().GetLabel()
	fmt.Printf("TestResult for %s: %s\n", testname, m.GetStatus())
	fmt.Printf("  invocationId: %s\n  invocationSha: %s\n\n", invocationId, invocationSha)
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
	filePrefix := "file://"
	if strings.HasPrefix(ofuri, filePrefix) {
		ofuri = ofuri[len(filePrefix):]
	}

	// Process test metrics output file(s).
	// Each output file contains a single (potentially large) protobuf message.
	if err := extractZippedFiles(ofuri); err != nil {
		return fmt.Errorf("Error processing %s file: %s", ofname, err)
	}

	return nil
}

// Look for and return a []byte slice with its contents.
func extractZippedFiles(zipFile string) error {
	metricsFileRE := regexp.MustCompile(`(^test_metrics.pb$|^test_metrics[_-]+[[:word:]-]*[[:alnum:]]\.pb$)`)
	summaryFileRE := regexp.MustCompile(`(^test_summary.pb$|^test_summary[_-]+[[:word:]-]*[[:alnum:]]\.pb$)`)
	allRE := []*regexp.Regexp{metricsFileRE, summaryFileRE}

	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		//  Look for a matching file name using regular expression patterns.
		match := ""
		for _, re := range allRE {
			match = re.FindString(filepath.Base(file.Name))
			if len(match) > 0 {
				break
			}
		}
		if len(match) == 0 {
			continue
		}
		fmt.Printf("Found output file to process: %s\n", file.Name)

		f, err := file.Open()
		if err != nil {
			return err
		}
		defer f.Close()

		// Read entire file contents into a byte slice.
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		// Extract all metrics from the file contents.
		pResult, err := getTestMetricsFromFileData(&data)
		if err != nil {
			fmt.Printf("Error processing output file %s: %s\n", file.Name, err)
			continue
		}
		if pResult != nil {
			// Display the metric data on the console.
			displayMetrics(pResult, 2)
			// TODO: Upload test metrics to BigQuery database table.
		}
	}
	return nil
}

// Extract the test metrics information from the protobuf message data read
// from an output file. This is used for processing test_metrics*.pb and
// test_summary*.pb output files.
func getTestMetricsFromFileData(pbmsg *[]byte) (*metricTestResult, error) {
	tmet := &tpb.TestMetrics{}
	if err := proto.Unmarshal(*pbmsg, tmet); err != nil {
		return nil, fmt.Errorf("Error parsing metric data: %s: %s", err, pbmsg)
	}
	var result metricTestResult
	table := tmet.GetTable()
	metrics := tmet.GetMetrics()
	t := bigQueryTable{
		project:   table.GetProject(),
		dataset:   table.GetDataset(),
		tablename: table.GetTablename(),
	}
	result.table = t
	for _, metric := range metrics {
		m := testMetric{
			metricname: metric.GetMetricname(),
			tags:       metric.GetTags(),
			value:      metric.GetValue(),
			timestamp:  metric.GetTimestamp(),
		}
		result.metrics = append(result.metrics, m)
	}
	return &result, nil
}

// Print the test metric data using a starting indentation offset.
func displayMetrics(res *metricTestResult, offset int) {
	fmt.Printf("%*sBigQuery Table: %s.%s.%s\n", offset, "",
		res.table.project, res.table.dataset, res.table.tablename)
	for _, m := range res.metrics {
		fmt.Printf("%*sMetric: metricname=%s value=%f timestamp=%d tags=%s\n",
			offset*2, "", m.metricname, m.value, m.timestamp, m.tags)
	}
	fmt.Println()
}
