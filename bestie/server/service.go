package main

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// List of event ID names defined by build_event_stream.proto.
// These formal event names are mapped to Prometheus metric label names.
var besEventIds = map[string]string{
	"Unknown":                       "unknown", // should not normally see any of these
	"Progress":                      "progress",
	"Started":                       "started",
	"CommandLine":                   "unstructured_command_line", // CommandLine replaced by UnstructuredCommandLine
	"UnstructuredCommandLine":       "unstructured_command_line",
	"StructuredCommandLine":         "structured_command_line",
	"WorkspaceStatus":               "workspace_status",
	"OptionsParsed":                 "options_parsed",
	"Fetch":                         "fetch",
	"Configuration":                 "configuration",
	"TargetConfigured":              "target_configured",
	"Pattern":                       "pattern",
	"PatternSkipped":                "pattern_skipped",
	"NamedSet":                      "named_set",
	"TargetCompleted":               "target_completed",
	"ActionCompleted":               "action_completed",
	"UnconfiguredLabel":             "unconfigured_label",
	"ConfiguredLabel":               "configured_label",
	"TestResult":                    "test_result",
	"TestSummary":                   "test_summary",
	"TargetSummary":                 "target_summary",
	"BuildFinished":                 "build_finished",
	"BuildToolLogs":                 "build_tool_logs",
	"BuildMetrics":                  "build_metrics",
	"Workspace":                     "workspace",
	"BuildMetadata":                 "build_metadata",
	"ConvenienceSymlinksIdentified": "convenience_symlinks_identified",
}

// Service statistics.
type serviceStats struct {
	// General Build Event Stream stats.
	buildsTotal prometheus.Counter
	eventsTotal *prometheus.CounterVec

	// BigQuery interaction stats.
	bigQueryExceptionTotal *prometheus.CounterVec
	bigQueryInsertDelay    prometheus.Histogram
	bigQueryInsertTotal    *prometheus.CounterVec
	bigQueryMetricTotal    *prometheus.CounterVec
}

// Statistic values to report to Prometheus for this service.
var ServiceStats serviceStats = serviceStats{
	buildsTotal: prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "builds_total",
			Help:      "Total number of Bazel builds seen",
		}),
	eventsTotal: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "events_total",
			Help:      "Total observed Bazel events, tagged by event ID",
		},
		[]string{"id"},
	),
	bigQueryExceptionTotal: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_exception_total",
			Help:      "Total BigQuery operational exceptions, tagged by type",
		},
		[]string{"type"},
	),
	bigQueryInsertDelay: prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "bestie",
			Name:      "bigquery_insert_delay",
			Help:      "Historgram of BigQuery table insertion delay, in seconds",
			Buckets:   []float64{0.0, 5.0, 10.0, 30.0, 60.0, 90.0, 120.0},
		},
	),
	bigQueryInsertTotal: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_insert_total",
			Help:      "Total BigQuery table insertions, tagged by result status",
		},
		[]string{"status"},
	),
	bigQueryMetricTotal: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_metric_total",
			Help:      "Total BigQuery metrics processed, tagged by result status",
		},
		[]string{"status"},
	),
}

// Register all service stats metrics with Prometheus.
func (s *serviceStats) registerPrometheusMetrics() {
	prometheus.MustRegister(s.buildsTotal)
	prometheus.MustRegister(s.eventsTotal)
	prometheus.MustRegister(s.bigQueryExceptionTotal)
	prometheus.MustRegister(s.bigQueryInsertDelay)
	prometheus.MustRegister(s.bigQueryInsertTotal)
	prometheus.MustRegister(s.bigQueryMetricTotal)
}

// Run-time initialization of service stats struct.
func (s *serviceStats) init() {
	s.registerPrometheusMetrics()
}

//
// Accessor methods to update Prometheus metrics.
//

// Increment total number of Bazel builds seen.
func (s *serviceStats) incrementBuildsTotal() {
	s.buildsTotal.Inc()
}

// Increment per-event count.
func (s *serviceStats) incrementEventsTotal(bevid interface{}) {
	x := strings.Split(fmt.Sprintf("%T", bevid), "_")
	id := x[len(x)-1] // use last split item
	label := besEventIds["Unknown"]
	if promId, ok := besEventIds[id]; ok {
		label = promId
	} else {
		// Make note of this condition. Probably means .proto file definition was updated.
		fmt.Printf("Detected unknown event id: %s\n\n", id)
	}
	s.eventsTotal.WithLabelValues(label).Inc()
}

// BigQuery operational exceptions.
func (s *serviceStats) incrementBigQueryExcessDelay() {
	s.bigQueryExceptionTotal.WithLabelValues("excess_delay").Inc()
}
func (s *serviceStats) incrementBigQueryProtobufError() {
	s.bigQueryExceptionTotal.WithLabelValues("protobuf_error").Inc()
}
func (s *serviceStats) incrementBigQueryTableNotFound() {
	s.bigQueryExceptionTotal.WithLabelValues("table_not_found").Inc()
}

// BigQuery row insertion delay (histogram).
func (s *serviceStats) incrementBigQueryInsertDelay(seconds int) {
	s.bigQueryInsertDelay.Observe(float64(seconds))
}

// BigQuery row insertion outcome.
func (s *serviceStats) incrementBigQueryInsertError() {
	s.bigQueryInsertTotal.WithLabelValues("error").Inc()
}
func (s *serviceStats) incrementBigQueryInsertOK() {
	s.bigQueryInsertTotal.WithLabelValues("ok").Inc()
}
func (s *serviceStats) incrementBigQueryInsertTimeout() {
	s.bigQueryInsertTotal.WithLabelValues("timeout").Inc()
}

// BigQuery total metrics processed.
func (s *serviceStats) incrementBigQueryMetricDiscard(count int) {
	s.bigQueryMetricTotal.WithLabelValues("discard").Add(float64(count))
}
func (s *serviceStats) incrementBigQueryMetricUpload(count int) {
	s.bigQueryMetricTotal.WithLabelValues("upload").Add(float64(count))
}
