package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	bq "cloud.google.com/go/bigquery"
)

type bigQuerySession struct {
	ctx    context.Context
	client *bq.Client
}

type bigQueryMetric struct {
	metricname string
	tags       string // stringified JSON
	value      float64
	timestamp  string // "YYYY-MM-DD hh:mm:ss.uuuuuu"
}

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

// BigQuery service location.
const databaseLocation = "us-west1"

// Define default BigQuery table to use if not specified in Bazel TestResult event message.
//
// NOTE: If BES Endpoint needs to shard metrics into multiple tables, it will add a suitable
// suffix to the tablename specified here.
var bigQueryTableDefault = bigQueryTable{
	project:   "bestie-builds",
	dataset:   "",            // Must be specified as --dataset arg on the command line.
	tablename: "testmetrics", // Can be overridden from the --tablename arg on the command line.
}

// Save implements the ValueSaver interface.
// This example disables best-effort de-duplication, which allows for higher throughput.
func (i *bigQueryMetric) Save() (map[string]bq.Value, string, error) {
	ret := map[string]bq.Value{
		"metricname": i.metricname,
		"tags":       i.tags,
		"value":      i.value,
		"timestamp":  i.timestamp,
	}
	return ret, bq.NoDedupeID, nil
}

func getMetricsTableSchema() bq.Schema {
	databaseSchema := bq.Schema{
		{Name: "metricname", Type: bq.StringFieldType, Description: "metric name", Required: true},
		{Name: "tags", Type: bq.StringFieldType, Description: "metric attribute tags"},
		{Name: "value", Type: bq.FloatFieldType, Description: "metric value"},
		{Name: "timestamp", Type: bq.TimestampFieldType, Description: "sample collection timestamp", Required: true},
	}
	return databaseSchema
}

// Produce a formatted string of the table identifier.
func (t bigQueryTable) formatDatasetId() string {
	return fmt.Sprintf("`%s.%s`", t.project, t.dataset)
}

// Produce a formatted string of the dataset identifier.
func (t bigQueryTable) formatTableId() string {
	return fmt.Sprintf("`%s.%s.%s`", t.project, t.dataset, t.tablename)
}

// Check if a BigQuery table exists based on reading its metadata.
func (t bigQueryTable) isTableExist(session bigQuerySession) bool {
	_, err := session.client.Dataset(t.dataset).Table(t.tablename).Metadata(session.ctx)
	return err == nil
}

// Normalize BigQuery table references by filling in with defaults, as needed.
func normalizeTableRef(t *bigQueryTable) {
	// Always use the default project.
	t.project = bigQueryTableDefault.project

	// If either of the following were not found in the protobuf message,
	// their zero value will show up here.
	if len(t.dataset) == 0 {
		t.dataset = bigQueryTableDefault.dataset
	}
	if len(t.tablename) == 0 {
		t.tablename = bigQueryTableDefault.tablename
	}
}

// Create a new dataset using an explicit destination location.
func createDataset(w io.Writer, t bigQueryTable, location string, session bigQuerySession) error {
	meta := &bq.DatasetMetadata{
		Location: location, // See https://cloud.google.com/bigquery/docs/locations
	}
	if err := session.client.Dataset(t.dataset).Create(session.ctx, meta); err != nil {
		return fmt.Errorf("Error creating dataset '%s': %s\n", t.formatDatasetId(), err)
	}
	fmt.Fprintf(w, "Created dataset %s\n", t.formatDatasetId())
	return nil
}

// Create a BigQuery table with a predefined schema.
func createTable(w io.Writer, t bigQueryTable, session bigQuerySession) error {
	// Always attempt to create the dataset that holds the table.
	// This is faster than first querying the table metadata,
	// then creating the dataset if not present.
	// We expect an "already exists" error if the dataset currently
	// exists for the project, which is the normal case.
	if err := createDataset(w, t, databaseLocation, session); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "already exist") {
			return err
		}
	}

	// Create the table within the dataset.
	tableSchema := getMetricsTableSchema()
	fmt.Fprintln(w, "Metrics Table Schema:")
	for _, field := range tableSchema {
		fmt.Fprintf(w, "  %s (%s)\n", field.Name, field.Type)
	}
	fmt.Fprintln(w)

	// NOTE: Not setting an expiration for BigQuery tables created by the endpoint.
	// If an expiration was used and someone forgets to extend the deadline,
	// the table and all of its data are deleted by BigQuery without warning.
	metaData := &bq.TableMetadata{
		Schema: tableSchema,
		//ExpirationTime: time.Now().AddDate(1, 0, 0), // Table will be automatically deleted in 1 year
	}
	tableRef := session.client.Dataset(t.dataset).Table(t.tablename)
	if err := tableRef.Create(session.ctx, metaData); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "already exist") {
			return fmt.Errorf("Error creating table '%s': %s\n", t.formatTableId(), err)
		}
	} else {
		fmt.Fprintf(w, "Created table %s\n", t.formatTableId())
	}
	return nil
}

// Translate protobuf metric to BigQuery metric.
func translateMetric(stream bazelStream, m testMetric) (bigQueryMetric, error) {
	// Process the slice of key/value pairs representing individual metric tags.
	dat := make(map[string]string)
	for _, kv := range m.tags {
		dat[kv.key] = kv.value
	}
	// Insert additional tags using information that is only available
	// to the BES endpoint through the Bazel event message itself.
	//
	// Storing both the invocationId and the invocationSha in the database,
	// the former being useful to match up with build log references, etc.
	dat["invocation_id"] = stream.invocationId
	dat["invocation_sha"] = stream.invocationSha
	dat["run"] = stream.run
	dat["test_name"] = stream.testName
	tags, err := json.Marshal(dat)
	if err != nil {
		return bigQueryMetric{}, fmt.Errorf("Error converting JSON to string: %s", err)
	}

	// Change the metric timestamp from integer to formatted string,
	// since that is what BigQuery expects for a TIMESTAMP column.
	//
	// The incoming timestamp is UTC nanoseconds since epoch, but
	// storing with microsecond granularity in BigQuery.
	ts := m.timestamp / 1000
	tsf := time.Unix(ts/1000000, ts%1000000).Format("2006-01-02 15:04:05.000000")

	return bigQueryMetric{
		metricname: m.metricname,
		tags:       string(tags),
		value:      m.value,
		timestamp:  tsf,
	}, nil
}

// Upload this set of metrics to the specified BigQuery table.
func uploadTestMetrics(w io.Writer, stream bazelStream, r *metricTestResult) error {
	// Normalize the BigQuery table identifier based on whether one was specified
	// in the protobuf message.
	normalizeTableRef(&r.table)
	fmt.Fprintf(w, "Normalized table ref: %s\n", r.table.formatTableId())

	// Get client context for this BigQuery operation.
	ctx := context.Background()
	client, err := bq.NewClient(ctx, r.table.project)
	if err != nil {
		return fmt.Errorf("bigquery.NewClient: %v", err)
	}
	defer client.Close()
	session := bigQuerySession{ctx: ctx, client: client}

	// Check if the BigQuery dataset and table exists; create each as needed.
	if exist := r.table.isTableExist(session); !exist {
		ServiceStats.incrementBigQueryTableNotFound()
		if err := createTable(w, r.table, session); err != nil {
			return err
		}
	}

	// Prepare the metric rows for uploading to BigQuery.
	// Make sure each row element references its own array item.
	// Note: bigQueryMetric implements the ValueSaver interface.
	var bqMetrics []bigQueryMetric
	var rows []*bigQueryMetric
	idx := 0
	for _, m := range r.metrics {
		// TODO: Decide whether to override client metric time with BES server current time.
		bqMetric, err := translateMetric(stream, m)
		if err != nil {
			ServiceStats.incrementBigQueryMetricDiscard(1)
			fmt.Fprintf(w, "Discarding metric %s due to error: %s\n", m, err)
			continue
		}
		bqMetrics = append(bqMetrics, bqMetric)
		rows = append(rows, &bqMetrics[idx])
		idx++
	}
	fmt.Fprintf(w, "\nTranslated metrics for BigQuery upload:\n")
	for _, bqMetric := range bqMetrics {
		fmt.Fprintf(w, "  %v\n", bqMetric)
	}
	fmt.Fprintln(w)

	// Attempt to upload the metrics, assuming the dataset and table
	// both exist. If a "not found" error occurs, sleep for a while
	// then try uploading again.
	//
	// Using a finite delay loop here to give BigQuery time to instantiate
	// the table, which can take a relativly long time (i.e. tens of seconds).
	ok := false
	insertionDelay := 0
	sleepTime := 10
	inserter := client.Dataset(r.table.dataset).Table(r.table.tablename).Inserter()
	fmt.Fprintf(w, "Waiting for table insertion")
	for i := 0; i < 12; i++ {
		if err := inserter.Put(ctx, rows); err != nil {
			// Treat anything other than a "not found" error as a failure.
			if !strings.Contains(strings.ToLower(err.Error()), "not found") {
				fmt.Println(w)
				ServiceStats.incrementBigQueryInsertError()
				return err
			}
			fmt.Fprintf(w, ".")
		} else {
			ok = true
			break
		}
		time.Sleep(time.Duration(sleepTime) * time.Second)
		insertionDelay += sleepTime
	}
	fmt.Fprintln(w)
	if !ok {
		ServiceStats.incrementBigQueryInsertTimeout()
		return fmt.Errorf("Error uploading rows to table %s: insertion timed out", r.table.formatTableId())
	}
	ServiceStats.incrementBigQueryMetricUpload(len(rows))
	ServiceStats.incrementBigQueryInsertOK()
	ServiceStats.incrementBigQueryInsertDelay(insertionDelay)
	fmt.Fprintf(w, "Successfully inserted %d rows (delay=%d)\n\n", len(rows), insertionDelay)
	return nil
}
