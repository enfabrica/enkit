package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricBigqueryInsertDelay = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "bestie",
			Name:      "bigquery_insert_delay",
			Help:      "Historgram of BigQuery table insertion delay, in seconds",
			Buckets:   []float64{0.0, 5.0, 10.0, 30.0, 60.0, 90.0, 120.0},
		},
	)
	metricBigqueryInsertsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_inserts_total",
			Help:      "Total BigQuery table insertions, tagged by result status",
		},
		[]string{"status"},
	)
	metricBigqueryMetricsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_metrics_total",
			Help:      "Total BigQuery metrics processed, tagged by result status",
		},
		[]string{"status"},
	)
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

	// Process the map of key/value pairs representing individual metric tags.
	// These come from the test application.
	for k, v := range m.tags {
		dat[k] = v
	}

	// If a "created" tag was provided by the test application (nanoseconds since epoch),
	// use it in place of the original metric timestamp value. Delete the test app's
	// "created" tag; it is replaced with a different "_created" tag below.
	timestamp := m.timestamp
	if val, ok := dat["created"]; ok {
		delete(dat, "created")
		ts, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			glog.Errorf("Error converting 'created' timestamp %q to int64: %q (value ignored)", val, err)
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

	// Store the metric name as a tag in addition to the BigQuery table 'metricname'
	// column so that it can be displayed in certain Grafana dashboard tables.
	//
	// Note: To avoid potential tag name conflicts with tags created by the
	// test application, the tags uniquely inserted by the BES Endpoint all
	// begin with a leading underscore, by convention.
	dat["_metric_name"] = m.metricName

	// Store the formatted timestamp string as a "_created" tag for the metrics
	// to facilitate defining Grafana metric queries by time of run, since
	// Prometheus supplies its scrape time, which is not what we want.
	// This is in addition to using it for the BigQuery table timestamp column.
	//
	// This replaces the test application's original "created" timestamp value,
	// which is not a formatted datetime string. Since this is essentially a
	// different tag value, use this opportunity to follow the BES-inserted tag
	// naming convention.
	dat["_created"] = tsf

	// Insert additional tags using information that is only available
	// to the BES endpoint through the Bazel event message itself.
	//
	// Storing both the invocationId and the invocationSha in the database,
	// the former being useful to match up with build log references, etc.
	dat["_invocation_id"] = stream.invocationId
	dat["_invocation_sha"] = stream.invocationSha
	dat["_run"] = stream.run
	dat["_test_target"] = stream.testTarget
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
	glog.V(2).Infof("Normalized table ref: %q", r.table.formatTableId())

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
		metricBigqueryExceptionsTotal.WithLabelValues("dataset_not_found").Inc()
		return fmt.Errorf("dataset_not_found for bigquery dataset %q", r.table.formatDatasetId())
	}
	if exist := r.table.isTableExist(ctx, client); !exist {
		metricBigqueryExceptionsTotal.WithLabelValues("table_not_found").Inc()
		return fmt.Errorf("table_not_found for bigquery table %q in dataset %q", r.table.formatTableId(), r.table.formatDatasetId())
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
			metricBigqueryMetricsTotal.WithLabelValues("discard").Inc()
			glog.Errorf("Discarding metric %q due to error: %v", m, err)
			continue
		}
		bqMetrics = append(bqMetrics, *pMetric)
		rows = append(rows, &bqMetrics[idx])
		idx++
	}

	if glog.V(2) {
		var sbuf strings.Builder
		sbuf.WriteString("\nTranslated metrics for BigQuery upload:\n")
		for _, bqMetric := range bqMetrics {
			sbuf.WriteString(fmt.Sprintf("  %v\n", bqMetric))
		}
		sbuf.WriteString("\n")
		glog.Info(sbuf.String())
	}

	// Attempt to upload the metrics, assuming the dataset and table
	// both exist. If a "not found" error occurs, sleep for a while
	// then try uploading again.
	//
	// Using a finite delay loop here to give BigQuery time to instantiate
	// the table, which can take a relativly long time (i.e. tens of seconds).
	ok := false
	insertStart := time.Now()
	sleepTime := 10
	inserter := client.Dataset(r.table.dataset).Table(r.table.tableName).Inserter()
	glog.V(2).Info("Waiting for table insertion...")
	for i := 0; i < 12; i++ {
		if err := inserter.Put(ctx, rows); err != nil {
			// Treat anything other than a "not found" error as a failure.
			if !strings.Contains(strings.ToLower(err.Error()), "not found") {
				metricBigqueryInsertsTotal.WithLabelValues("error").Inc()
				return err
			}
		} else {
			ok = true
			break
		}
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
	if !ok {
		metricBigqueryInsertsTotal.WithLabelValues("timeout").Inc()
		return fmt.Errorf("Error uploading rows to table %q: insertion timed out", r.table.formatTableId())
	}
	metricBigqueryInsertsTotal.WithLabelValues("ok").Inc()
	metricBigqueryInsertDelay.Observe(time.Now().Sub(insertStart).Seconds())
	metricBigqueryMetricsTotal.WithLabelValues("upload").Add(float64(len(rows)))
	glog.V(1).Infof("Successfully inserted %d rows", len(rows))
	return nil
}
