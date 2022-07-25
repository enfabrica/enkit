package main

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/golang/glog"
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

// List of all Prometheus counter IDs.
type counterId int

const (
	cidBuildsTotal                   counterId = iota // 0
	cidEventsTotal                                    // 1
	cidExceptionDatasetNotFound                       // 2
	cidExceptionExcessDelay                           // 3
	cidExceptionProtobufError                         // 4
	cidExceptionTableNotFound                         // 5
	cidExceptionXmlParseError                         // 6
	cidExceptionXmlStructuredError                    // 7
	cidExceptionXmlUnstructuredError                  // 8
	cidInsertDelay                                    // 9
	cidInsertError                                    // 10
	cidInsertOK                                       // 11
	cidInsertTimeout                                  // 12
	cidMetricDiscard                                  // 13
	cidMetricUpload                                   // 14
	cidOutputFileTooBigTotal                          // 15
)

// Service statistics.
type serviceStats struct {
	// General Build Event Stream stats.
	buildsTotal           prometheus.Counter
	eventsTotal           *prometheus.CounterVec
	outputFileTooBigTotal prometheus.Counter

	// BigQuery interaction stats.
	bigQueryExceptionsTotal *prometheus.CounterVec
	bigQueryInsertDelay     prometheus.Histogram
	bigQueryInsertsTotal    *prometheus.CounterVec
	bigQueryMetricsTotal    *prometheus.CounterVec
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
	outputFileTooBigTotal: prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "output_file_too_big_total",
			Help:      "Total output files not processed due to excessive size",
		}),
	bigQueryExceptionsTotal: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_exceptions_total",
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
	bigQueryInsertsTotal: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_inserts_total",
			Help:      "Total BigQuery table insertions, tagged by result status",
		},
		[]string{"status"},
	),
	bigQueryMetricsTotal: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_metrics_total",
			Help:      "Total BigQuery metrics processed, tagged by result status",
		},
		[]string{"status"},
	),
}

// Register all service stats metrics with Prometheus.
func (s *serviceStats) registerPrometheusMetrics() {
	prometheus.MustRegister(s.buildsTotal)
	prometheus.MustRegister(s.eventsTotal)
	prometheus.MustRegister(s.outputFileTooBigTotal)
	prometheus.MustRegister(s.bigQueryExceptionsTotal)
	prometheus.MustRegister(s.bigQueryInsertDelay)
	prometheus.MustRegister(s.bigQueryInsertsTotal)
	prometheus.MustRegister(s.bigQueryMetricsTotal)
}

// Run-time initialization of service stats struct.
func (s *serviceStats) init() {
	s.registerPrometheusMetrics()
}

// Get the label used to identify a BES event.
func getEventLabel(bevid interface{}) string {
	x := strings.Split(fmt.Sprintf("%T", bevid), "_")
	id := x[len(x)-1] // use last split item
	label := besEventIds["Unknown"]
	if promId, ok := besEventIds[id]; ok {
		label = promId
	} else {
		// Make note of this condition. Probably means .proto file definition was updated.
		glog.Warningf("Detected unknown event id: %s", id)
	}
	return label
}

// Increment the designated service statistic by 1.
func (cid counterId) increment() {
	go updatePrometheusCounter(cid, "", float64(1.0))
}

// Update the designated service statistic by adding the specified amount.
func (cid counterId) update(amount int) {
	go updatePrometheusCounter(cid, "", float64(amount))
}

// Update the designated service statistic by adding the specified amount.
// Use this method whenever a qualifying label is needed for the metric.
func (cid counterId) updateWithLabel(label string, amount int) {
	go updatePrometheusCounter(cid, label, float64(amount))
}

// Update the Prometheus counter corresponding to the service statistic ID.
// NOTE: This should be invoked as a goroutine to do the actual counter update.
func updatePrometheusCounter(cid counterId, label string, n float64) {
	s := &ServiceStats
	switch cid {
	case cidBuildsTotal:
		s.buildsTotal.Add(n)
	case cidEventsTotal:
		// There are too many BES event names to define a unique counter ID
		// for each, so the caller must pass in the label to use.
		s.eventsTotal.WithLabelValues(label).Add(n)
	case cidExceptionDatasetNotFound:
		s.bigQueryExceptionsTotal.WithLabelValues("dataset_not_found").Add(n)
	case cidExceptionExcessDelay:
		s.bigQueryExceptionsTotal.WithLabelValues("excess_delay").Add(n)
	case cidExceptionProtobufError:
		s.bigQueryExceptionsTotal.WithLabelValues("protobuf_error").Add(n)
	case cidExceptionTableNotFound:
		s.bigQueryExceptionsTotal.WithLabelValues("table_not_found").Add(n)
	case cidExceptionXmlParseError:
		s.bigQueryExceptionsTotal.WithLabelValues("xml_parse_error").Add(n)
	case cidExceptionXmlStructuredError:
		s.bigQueryExceptionsTotal.WithLabelValues("xml_structured_error").Add(n)
	case cidExceptionXmlUnstructuredError:
		s.bigQueryExceptionsTotal.WithLabelValues("xml_unstructured_error").Add(n)
	case cidInsertDelay:
		s.bigQueryInsertDelay.Observe(float64(n))
	case cidInsertError:
		s.bigQueryInsertsTotal.WithLabelValues("error").Add(n)
	case cidInsertOK:
		s.bigQueryInsertsTotal.WithLabelValues("ok").Add(n)
	case cidInsertTimeout:
		s.bigQueryInsertsTotal.WithLabelValues("timeout").Add(n)
	case cidMetricDiscard:
		s.bigQueryMetricsTotal.WithLabelValues("discard").Add(float64(n))
	case cidMetricUpload:
		s.bigQueryMetricsTotal.WithLabelValues("upload").Add(float64(n))
	case cidOutputFileTooBigTotal:
		s.outputFileTooBigTotal.Add(n)
	}
}
