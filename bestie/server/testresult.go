package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
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
		return fmt.Errorf("Error extracting TestResult data from message")
	}
	// Get the 'bazel test' target name from the bazel build event id label.
	testname := bazelBuildEvent.GetId().GetTestResult().GetLabel()
	fmt.Printf("TestResult for %s: %s\n", testname, m.GetStatus())
	fmt.Printf("  invocationId: %s\n  invocationSha: %s\n\n", invocationId, invocationSha)
	outputFiles := m.GetTestActionOutput()
	foundTestResults := -1
	for i, of := range outputFiles {
		ofname := of.GetName()
		if strings.HasSuffix(ofname, "outputs.zip") {
			foundTestResults = i
			break
		}
	}
	if foundTestResults >= 0 {
		ofname := outputFiles[foundTestResults].GetName()
		ofuri := outputFiles[foundTestResults].GetUri()
		// Strip off any file:// prefix from the URI to access the local file system path.
		filePrefix := "file://"
		if strings.HasPrefix(ofuri, filePrefix) {
			ofuri = ofuri[len(filePrefix):]
		}
		// Unzip the output file contents into a temporary directory.
		//
		// Note: Must protect this with a mutex since multiple bazel test targets
		// end up using this same bazel workspace directory. There is currently a
		// lock protecting this entire handler function, so should be OK.
		outfiles, err := unzipSource(ofuri, "")
		if err != nil {
			return fmt.Errorf("Error unzipping output file %s: %s", ofname, err)
		}
		// Display list of files contained in output zip file.
		fmt.Printf("Found output file: %s\n  uri:\n    %s\n  files:\n", ofname, ofuri)
		for _, outfile := range outfiles {
			fmt.Printf("    %s\n", outfile)
		}
		fmt.Println()
		// For certain file names of interest (sans path), process their contents.
		for _, outfile := range outfiles {
			basename := filepath.Base(outfile)
			switch basename {
			case "test_metrics.pb":
				fmt.Printf("Processing output file: %s\n", basename)
				res, err := getTestMetrics(outfile)
				if err != nil {
					// Handle error internally.
					fmt.Printf("%s\n\n", err)
					continue
				}
				fmt.Printf("  BigQuery Table: %s.%s.%s\n",
					res.table.project, res.table.dataset, res.table.tablename)
				for _, m := range res.metrics {
					fmt.Printf("    Metric: metricname=%s value=%f timestamp=%d tags=%s\n",
						m.metricname, m.value, m.timestamp, m.tags)
				}
				fmt.Println()
				// TODO: Write metrics to BigQuery database table
			case "test_summary.pb":
				// TODO: Add support for testcase summary pass/fail metrics.
			default:
				// Ignore file.
			}
			fmt.Println()
		}
	}
	return nil
}

// Read the test_metrics.pb file in its entirety and process the protobuf message within.
func getTestMetrics(fn string) (*metricTestResult, error) {
	var result metricTestResult
	in, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, fmt.Errorf("Error reading file %s: %s", fn, err)
	}
	tmet := &tpb.TestMetrics{}
	if err := proto.Unmarshal(in, tmet); err != nil {
		return nil, fmt.Errorf("Error parsing metric data: %s: %s", err, in)
	}
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
