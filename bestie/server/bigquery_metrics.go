package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
)

type bigQueryMetric struct {
	metricName string
	tags       string // stringified JSON
	value      float64
	timestamp  string // must be: "YYYY-MM-DD hh:mm:ss.uuuuuu"
}

type bigQueryTable struct {
	project   string
	dataset   string
	tableName string
}

type testMetric struct {
	metricName string
	tags       map[string]string
	value      float64
	timestamp  int64
}

type metricTestResult struct {
	table   bigQueryTable
	metrics []testMetric
}

// Timestamp datetime format required by BigQuery.
// Go dictates that these particular field values must be used
// when formatting datetime as a custom string.
const timestampFormat = "2006-01-02 15:04:05.000000"

// BigQuery test metrics table schema definition.
// Although no longer creating tables in the code, this is kept for reference.
//
//var databaseSchema = bigquery.Schema{
//	{Name: "metricname", Type: bigquery.StringFieldType, Description: "metric name", Required: true},
//	{Name: "tags", Type: bigquery.StringFieldType, Description: "metric attribute tags"},
//	{Name: "value", Type: bigquery.FloatFieldType, Description: "metric value"},
//	{Name: "timestamp", Type: bigquery.TimestampFieldType, Description: "sample collection timestamp", Required: true},
//}

// Define default BigQuery table to use if not specified in Bazel TestResult event message.
//
// NOTE: If BES Endpoint needs to shard metrics into multiple tables, it will add a suitable
// suffix to the table name specified here.
var bigQueryTableDefault = bigQueryTable{
	project:   "bestie-builds",
	dataset:   "",            // Must be specified as --dataset arg on the command line.
	tableName: "testmetrics", // Can be overridden from the --table_name arg on the command line.
}

// Save implements the ValueSaver interface.
// This example disables best-effort de-duplication, which allows for higher throughput.
func (i *bigQueryMetric) Save() (map[string]bigquery.Value, string, error) {
	ret := map[string]bigquery.Value{
		"metricname": i.metricName,
		"tags":       i.tags,
		"value":      i.value,
		"timestamp":  i.timestamp,
	}
	return ret, bigquery.NoDedupeID, nil
}

// Keep track of dataset and table names that were requested,
// but do not exist. Display a log message the first time
// each is encountered and every "rate" occurrence thereafter.
var missingResource = make(map[string]int)
var missingResourceMsgRate = 10

// Report a missing dataset or table.
func reportMissingResource(id, resourceType string) error {
	if _, ok := missingResource[id]; !ok {
		missingResource[id] = 0
	}
	missingResource[id]++
	errMsg := fmt.Sprintf("The BigQuery %s %q does not exist", resourceType, id)
	if (missingResource[id] % missingResourceMsgRate) == 1 {
		logger.Printf("%s. Please create it and try again.", errMsg)
	}
	return fmt.Errorf("%s", errMsg)
}

// UnixMicro returns the local Time corresponding to the given Unix time,
// usec microseconds since January 1, 1970 UTC.
// NOTE: This function is copied from src/time/time.go in the Go library (version 1.17.6).
func UnixMicro(usec int64) time.Time {
	return time.Unix(usec/1e6, (usec%1e6)*1e3)
}

// Produce a formatted string of the table identifier.
func (t *bigQueryTable) formatDatasetId() string {
	return fmt.Sprintf("%s.%s", t.project, t.dataset)
}

// Produce a formatted string of the dataset identifier.
func (t *bigQueryTable) formatTableId() string {
	return fmt.Sprintf("%s.%s.%s", t.project, t.dataset, t.tableName)
}

// Check if a BigQuery dataset exists based on reading its metadata.
func (t *bigQueryTable) isDatasetExist(ctx context.Context, client *bigquery.Client) bool {
	_, err := client.Dataset(t.dataset).Metadata(ctx)
	return err == nil
}

// Check if a BigQuery table exists based on reading its metadata.
func (t *bigQueryTable) isTableExist(ctx context.Context, client *bigquery.Client) bool {
	_, err := client.Dataset(t.dataset).Table(t.tableName).Metadata(ctx)
	return err == nil
}

// Normalize BigQuery table references by filling in with defaults, as needed.
func (t *bigQueryTable) normalizeTableRef() {
	// Always use the default project.
	t.project = bigQueryTableDefault.project

	// If either of the following were not found in the protobuf message,
	// their zero value will show up here.
	if len(t.dataset) == 0 {
		t.dataset = bigQueryTableDefault.dataset
	}
	if len(t.tableName) == 0 {
		t.tableName = bigQueryTableDefault.tableName
	}
}

// Translate protobuf metric to BigQuery metric.
func translateMetric(stream *bazelStream, m *testMetric) (*bigQueryMetric, error) {
	// Create a map to hold key/value pairs for various properties.
	dat := make(map[string]string)

	// Store the metric name as a tag in addition to the BigQuery table 'metricname'
	// column so that it can be displayed in Grafana dashboard tables.
	dat["name"] = m.metricName

	// Process the map of key/value pairs representing individual metric tags.
	for k, v := range m.tags {
		dat[k] = v
	}

	// If a "created" tag was provided (nanoseconds since epoch),
	// use it in place of the original metric timestamp value.
	timestamp := m.timestamp
	if val, ok := dat["created"]; ok {
		ts, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			debugPrintf("Error converting string %s to int64: %q (value ignored)\n", val, err)
		} else {
			timestamp = ts
		}
	}

	// Change the metric timestamp from integer to formatted string,
	// since that is what BigQuery expects for a TIMESTAMP column.
	//
	// The incoming timestamp is UTC nanoseconds since epoch (int64), which is
	// a familiar value that can be produced by all test clients and passed in
	// a protobuf message. It is formatted and stored with microsecond granularity
	// in BigQuery.
	//
	// Note that a specific Format() string is required to create a timestamp
	// format that BigQuery can work with. For example, BigQuery does not like "+0000 UTC"
	// at the end of the string, which is the default format emitted by UnixMicro().String().
	// See https://cloud.google.com/bigquery/docs/reference/standard-sql/data-types#timestamp_type
	// for details.
	tsf := UnixMicro(timestamp / 1e3).Format(timestampFormat)

	// Store the formatted timestamp string as a "created" tag for the metrics
	// to facilitate defining Grafana metric queries by time of run, since
	// Prometheus supplies its scrape time, which is not what we want.
	// This is in addition to using it for the BigQuery table timestamp column.
	// Overwrite the original value, which is not a formatted datetime string.
	dat["created"] = tsf

	// Insert additional tags using information that is only available
	// to the BES endpoint through the Bazel event message itself.
	//
	// Storing both the invocationId and the invocationSha in the database,
	// the former being useful to match up with build log references, etc.
	dat["invocation_id"] = stream.invocationId
	dat["invocation_sha"] = stream.invocationSha
	dat["run"] = stream.run
	dat["test_target"] = stream.testTarget
	tags, err := json.Marshal(dat)
	if err != nil {
		return nil, fmt.Errorf("Error converting JSON to string: %w", err)
	}

	return &bigQueryMetric{
		metricName: m.metricName,
		tags:       string(tags),
		value:      m.value,
		timestamp:  tsf,
	}, nil
}

// Upload this set of metrics to the specified BigQuery table.
func uploadTestMetrics(stream *bazelStream, r *metricTestResult) error {
	// Normalize the BigQuery table identifier based on whether one was specified
	// in the protobuf message.
	r.table.normalizeTableRef()
	debugPrintf("Normalized table ref: %q\n", r.table.formatTableId())

	// Get client context for this BigQuery operation.
	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, r.table.project)
	if err != nil {
		return fmt.Errorf("Error opening bigquery.NewClient: %w", err)
	}
	defer client.Close()

	// Check that the BigQuery dataset and table already exists.
	// For simplicity, the BES Endpoint is not responsible for creating
	// either one. An administrator is expected to create these ahead of time.
	if exist := r.table.isDatasetExist(ctx, client); !exist {
		cidExceptionDatasetNotFound.increment()
		return reportMissingResource(r.table.formatDatasetId(), "dataset")
	}
	if exist := r.table.isTableExist(ctx, client); !exist {
		cidExceptionTableNotFound.increment()
		return reportMissingResource(r.table.formatTableId(), "table")
	}

	// Prepare the metric rows for uploading to BigQuery.
	// Make sure each row element references its own array item.
	// Note: bigQueryMetric implements the ValueSaver interface.
	var bqMetrics []bigQueryMetric
	var rows []*bigQueryMetric
	idx := 0
	for _, m := range r.metrics {
		pMetric, err := translateMetric(stream, &m)
		if err != nil {
			cidMetricDiscard.increment()
			logger.Printf("Discarding metric %s due to error: %s\n\n", m, err)
			continue
		}
		bqMetrics = append(bqMetrics, *pMetric)
		rows = append(rows, &bqMetrics[idx])
		idx++
	}
	var sbuf strings.Builder
	sbuf.WriteString("\nTranslated metrics for BigQuery upload:\n")
	for _, bqMetric := range bqMetrics {
		sbuf.WriteString(fmt.Sprintf("  %v\n", bqMetric))
	}
	sbuf.WriteString("\n")
	debugPrintln(sbuf.String())

	// Attempt to upload the metrics, assuming the dataset and table
	// both exist. If a "not found" error occurs, sleep for a while
	// then try uploading again.
	//
	// Using a finite delay loop here to give BigQuery time to instantiate
	// the table, which can take a relativly long time (i.e. tens of seconds).
	ok := false
	insertionDelay := 0
	sleepTime := 10
	inserter := client.Dataset(r.table.dataset).Table(r.table.tableName).Inserter()
	debugPrintf("Waiting for table insertion...\n")
	for i := 0; i < 12; i++ {
		if err := inserter.Put(ctx, rows); err != nil {
			// Treat anything other than a "not found" error as a failure.
			if !strings.Contains(strings.ToLower(err.Error()), "not found") {
				cidInsertError.increment()
				return err
			}
		} else {
			ok = true
			break
		}
		time.Sleep(time.Duration(sleepTime) * time.Second)
		insertionDelay += sleepTime
	}
	if !ok {
		cidInsertTimeout.increment()
		return fmt.Errorf("Error uploading rows to table %q: insertion timed out", r.table.formatTableId())
	}
	cidInsertOK.increment()
	cidInsertDelay.update(insertionDelay)
	cidMetricUpload.update(len(rows))
	debugPrintf("Successfully inserted %d rows (delay=%d)\n\n", len(rows), insertionDelay)
	return nil
}
