package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// Staging struct to collect raw metric data.
type xmlResult struct {
	tcFile     string
	tcClass    string
	tcTestCase string
	tcProps    map[string]string
	result     string
	duration   string
	testTime   time.Time
}

// Unmarshalling structs for interpreting XML test results.
type TestSuites struct {
	XMLName    xml.Name    `xml:"testsuites"`
	TestSuites []TestSuite `xml:"testsuite"`
}

type TestSuite struct {
	XMLName    xml.Name   `xml:"testsuite"`
	Name       string     `xml:"name,attr"`
	Errors     string     `xml:"errors,attr"`
	Failures   string     `xml:"failures,attr"`
	Skipped    string     `xml:"skipped,attr"`
	Tests      string     `xml:"tests,attr"`
	Time       string     `xml:"time,attr"`
	Timestamp  string     `xml:"timestamp,attr"`
	Hostname   string     `xml:"hostname,attr"`
	TestCases  []TestCase `xml:"testcase"`
	Properties Properties `xml:"properties"`
	SystemOut  string     `xml:"system-out"`
}

type TestCase struct {
	XMLName    xml.Name   `xml:"testcase"`
	ClassName  string     `xml:"classname,attr"`
	Name       string     `xml:"name,attr"`
	Time       string     `xml:"time,attr"`
	Properties Properties `xml:"properties"`
	Failure    FailureMsg `xml:"failure"`
	Skipped    SkippedMsg `xml:"skipped"`
}

type Properties struct {
	XMLName xml.Name   `xml:"properties"`
	Props   []Property `xml:"property"`
}

type Property struct {
	XMLName xml.Name `xml:"property"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:"value,attr"`
}

type FailureMsg struct {
	XMLName xml.Name `xml:"failure"`
	Message string   `xml:"message,attr"`
}

type SkippedMsg struct {
	XMLName xml.Name `xml:"skipped"`
	Type    string   `xml:"type,attr"`
	Message string   `xml:"message,attr"`
}

// Read test result info from a test.xml file and create result metrics.
func processXmlMetrics(stream *bazelStream, fileReader io.Reader, fileName string) error {
	// Read entire file into a byte slice.
	fileData, err := readFileWithLimit(fileReader, maxFileSize)
	if err != nil {
		if errors.Is(err, fileTooBigErr) {
			cidOutputFileTooBigTotal.increment()
		}
		return fmt.Errorf("Error reading file %q: %w", filepath.Base(fileName), err)
	}

	// Extract all metrics from the XML file contents.
	pResult, err := getTestMetricsFromXmlData(fileData[:])
	if err != nil {
		return fmt.Errorf("Error extracting XML metrics: %w", err)
	}
	if pResult == nil {
		// Passive error processing XML data (already counted and/or logged).
		return nil
	}

	// Send the metrics to BigQuery.
	if err := processMetrics(stream, pResult); err != nil {
		return fmt.Errorf("Error processing XML metrics: %w", err)
	}

	return nil
}

// Extract the test metrics information from the XML data read
// from the test.xml output file contained in outputs.zip
// (not the standalone test.xml that is produced by Bazel).
func getTestMetricsFromXmlData(pbmsg []byte) (*metricTestResult, error) {
	var result metricTestResult
	var testSuites TestSuites
	if err := xml.Unmarshal(pbmsg, &testSuites); err != nil {
		cidExceptionXmlParseError.increment()
		return nil, fmt.Errorf("Error extracting metric data from XML file: %s", err)
	}

	// Process each of the testcases contained in the testsuite.
	for _, ts := range testSuites.TestSuites {
		// Check for optional BigQuery table specification within testsuite XML data.
		dataset := ""
		tableName := ""
		for _, prop := range ts.Properties.Props {
			if prop.Name == "bq_dataset" {
				dataset = prop.Value
			}
			if prop.Name == "bq_tablename" {
				tableName = prop.Value
			}
		}
		result.table = bigQueryTable{
			// The project field is omitted from the XML data.
			dataset:   dataset,
			tableName: tableName,
		}

		// The presence of a <system-out> tag means the output results are unstructured and
		// can only be processed by scraping the console output to the collect test case info.
		// Since this is non-deterministic and error prone, unstructured XML is not supported
		// (i.e. it requires the test applications to use junitxml style of output).
		if len(ts.SystemOut) > 0 {
			cidExceptionXmlUnstructuredError.increment()
			return nil, fmt.Errorf("Unstructured XML test results not supported (use junitxml)")
		}

		xmlResults, err := parseStructuredXml(&ts)
		if err != nil {
			cidExceptionXmlStructuredError.increment()
			return nil, fmt.Errorf("Error parsing structured XML test results: %s", err)
		}

		metricName := "testresult"
		for _, xr := range xmlResults {
			// Construct the BigQuery metric from the XML results provided.
			// Note that the metric name and creation datetime are also being stored
			// in the metric tags to facilitate Grafana queries and displays,
			// since Prometheus supplies its scrape time, which is not what we want.
			//
			// Note: To avoid potential tag name conflicts with tags created by the
			// test application, the tags uniquely inserted by the BES Endpoint all
			// begin with a leading underscore, by convention.
			//
			// Exception: Since the "type" tag normally comes from the test application,
			// use the same tag name here for consistency in the database.
			var m testMetric = testMetric{
				metricName: metricName,
				tags: map[string]string{
					"_duration":    xr.duration,
					"_metric_name": metricName,
					"_result":      xr.result,
					"_test_case":   xr.tcTestCase,
					"_test_class":  xr.tcClass,
					"_test_file":   xr.tcFile,
					"type":         "summary",
				},
				// Setting value to 1.0 so each test case result has same weight
				// for PromQL count() and sum() operations.
				value:     float64(1.0),
				timestamp: xr.testTime.UnixNano(),
			}
			// Append any test case properties to result tags.
			for k, v := range xr.tcProps {
				m.tags[k] = v
			}
			result.metrics = append(result.metrics, m)
		}
	}
	return &result, nil
}

// Parse a test.xml file that is properly structured with beginning and ending
// tags for each information element, thereby avoiding the need to scrape information
// from freeform console output text. This is the format produced by the pytest
// --junitxml command line option.
func parseStructuredXml(ts *TestSuite) ([]*xmlResult, error) {
	// Interpret the testsuite time formatted as: "2006-01-02T15:04:05.999999" (must eliminate the "T").
	testTime, err := time.Parse(timestampFormat, strings.Replace(ts.Timestamp, "T", " ", 1))
	if err != nil {
		testTime = time.Now()
	}

	// Process each of the testcases contained in the testsuite.
	var xmlResults []*xmlResult
	for _, tc := range ts.TestCases {
		var xr xmlResult
		// Determine file suffix based on testsuite name
		suffix := ""
		if ts.Name == "pytest" {
			suffix = ".py"
		}
		// Split the class name into separate parts for tagging.
		classNameParts := strings.Split(tc.ClassName, ".")
		xr.tcFile = strings.Join(classNameParts[:len(classNameParts)-1], "/") + suffix
		xr.tcClass = classNameParts[len(classNameParts)-1]
		xr.tcTestCase = tc.Name
		// Use the testcase time attribute as its duration.
		xr.duration = tc.Time
		// Derive the metric result string
		// Note: The absence of a 'failure' or 'skipped' section means the test passed.
		if len(tc.Failure.Message) > 0 {
			xr.result = "fail"
		} else if len(tc.Skipped.Message) > 0 {
			xr.result = "skip"
		} else {
			xr.result = "pass"
		}
		xr.testTime = testTime
		// Pick up any test case <property> attributes.
		tcProps := make(map[string]string)
		for _, prop := range tc.Properties.Props {
			tcProps[prop.Name] = prop.Value
		}
		xr.tcProps = tcProps
		xmlResults = append(xmlResults, &xr)
	}
	return xmlResults, nil
}
