package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
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
	duration   float64
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
func processXmlMetrics(stream *bazelStream, fileReader io.Reader) error {
	// Read entire file into a byte slice.
	fileData, err := ioutil.ReadAll(fileReader)
	if err != nil {
		return fmt.Errorf("Error reading file: %w", err)
	}

	// Extract all metrics from the XML file contents.
	pResult, err := getTestMetricsFromXmlData(fileData[:])
	if err != nil {
		return fmt.Errorf("Error extracting XML metrics: %w", err)
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
		cidExceptionXmlError.increment()
		return nil, fmt.Errorf("Error extracting metric data from XML file: %w", err)
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

		// The presence of a <system-out> tag means the output results are unformatted and
		// must be processed by scraping the console output to the collect test case info.
		var xmlResults []*xmlResult
		var xerr error
		if len(ts.SystemOut) > 0 {
			xmlResults, xerr = parseUnstructuredXml(&ts)
		} else {
			xmlResults, xerr = parseStructuredXml(&ts)
		}
		if xerr != nil {
			return nil, fmt.Errorf("Error parsing XML test results: %w", xerr)
		}

		for _, xr := range xmlResults {
			// Construct the BigQuery metric from the XML results provided.
			var m testMetric = testMetric{
				metricName: "testresult",
				tags: map[string]string{
					"result":     xr.result,
					"test_case":  xr.tcTestCase,
					"test_class": xr.tcClass,
					"test_file":  xr.tcFile,
					"type":       "summary",
				},
				value:     xr.duration,
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

// Parse a test.xml file that is output in a "raw" format, where individual
// test case names and pass/fail results must be scraped from the console
// output text string. This is the default format produced by 'bazel test'.
func parseUnstructuredXml(ts *TestSuite) ([]*xmlResult, error) {
	// Read each line of the console string, looking for two specific lines for each test case:
	//
	//   systest/trial/test_app.py::TestHello::test_app_metrics_nofile Testing resource 1322
	//   MAKEREPORT: test_app.py::TestHello::test_app_metrics_nofile: passed
	//
	// The last token in the MAKEREPORT: line is one of: passed, failed, or skipped.
	//
	// An abbreviated form of a skipped test is reported as follows:
	//
	//   systest/trial/test_app.py::TestHello::test_app_metrics_nofile SKIPPED
	//
	// Also look for a single line containing the SKIPPEDSESSIONFINISH: token designating
	// a test case that contains a pytest.skip() call during its execution:
	//
	//   systest/trial/test_app.py::TestHello::test_app_metrics_file[test.metrics.pb] SKIPPEDSESSIONFINISH: ...
	//
	// Extract the full test case ID from the first entry and the passed/failed/skipped result
	// from the second.
	matches := make(map[string]string)
	var key string
	for _, line := range strings.Split(strings.TrimSuffix(ts.SystemOut, "\n"), "\n") {
		if strings.Contains(line, "Testing resource") {
			key = strings.Split(strings.Split(line, " ")[0], "[")[0]
		} else if strings.Contains(line, "MAKEREPORT") {
			if len(key) > 0 {
				vals := strings.Split(line, " ")
				val := vals[len(vals)-1]
				matches[key] = val
				key = ""
			}
		} else if strings.Contains(line, " SKIPPEDSESSIONFINISH:") ||
			strings.Contains(line, " SKIPPED") {
			key = strings.Split(strings.Split(line, " ")[0], "[")[0]
			matches[key] = "skipped"
			key = ""
		}
	}

	// Process each found test case, storing the relevant result data in a common format.
	var xmlResults []*xmlResult
	testTime := time.Now() // use same time for each result
	for k, v := range matches {
		var xr xmlResult
		tcInfo := strings.Split(k, "::")
		xr.tcFile = tcInfo[0]
		xr.tcClass = tcInfo[1]
		xr.tcTestCase = tcInfo[2]
		xr.duration = float64(0.0)
		if v == "failed" {
			xr.result = "fail"
		} else if v == "skipped" {
			xr.result = "skip"
		} else {
			xr.result = "pass"
		}
		xr.testTime = testTime
		xmlResults = append(xmlResults, &xr)
	}
	return xmlResults, nil
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
		if xr.duration, err = strconv.ParseFloat(tc.Time, 64); err != nil {
			xr.duration = float64(0.0)
		}
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
