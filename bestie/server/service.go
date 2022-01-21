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

// BES Prometheus metric definition.
// Use this for untagged counters.
type besPromMetric struct {
	label string
	ctr   prometheus.Counter
}

// BES Prometheus metric vector definition.
// Use this for tagged counters.
type besPromMetricVec struct {
	ctr *prometheus.CounterVec
}

// BES Prometheus histogram definition.
type besPromHistogram struct {
	ctr prometheus.Histogram
}

// Number of BES messages seen by event type.const
type besEvents struct {
	bazelBuildsTotal besPromMetric
	bazelEventsTotal besPromMetricVec
}

// Define BES Endpoint service counters being exposed through Prometheus.
var (
	promBazelBuildsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "bes",
			Subsystem: "srv",
			Name:      "bazel_builds_total",
			Help:      "Total number of Bazel builds seen",
		})
	promBazelEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bes",
			Subsystem: "srv",
			Name:      "bazel_events_total",
			Help:      "Total observed events, tagged by event ID",
		},
		[]string{"id"},
	)
)

// Service statistics.
type serviceStats struct {
	// General Build Event Stream stats.
	besEvents
}

// Statistic values to report to Prometheus for this service.
var SrvStats serviceStats = serviceStats{
	besEvents: besEvents{
		// List of defined BazelBuildEvent IDs from build_event_stream.proto.
		bazelBuildsTotal: besPromMetric{ctr: promBazelBuildsTotal},
		bazelEventsTotal: besPromMetricVec{ctr: promBazelEventsTotal},
	},
}

// Register all service stats metrics with Prometheus.
func (s *serviceStats) registerPrometheusMetrics() {
	prometheus.MustRegister(s.besEvents.bazelBuildsTotal.ctr)
	prometheus.MustRegister(s.besEvents.bazelEventsTotal.ctr)
}

// Run-time initialization of service stats struct.
func (s *serviceStats) init() {
	s.registerPrometheusMetrics()
}

//
// Accessor methods to update Prometheus metrics.
//

// Increment total number of Bazel builds seen.
func (s *serviceStats) bazelBuildsTotal() {
	s.besEvents.bazelBuildsTotal.ctr.Inc()
}

// Increment per-event count.
func (s *serviceStats) bazelEventsTotal(bevid interface{}) {
	x := strings.Split(fmt.Sprintf("%T", bevid), "_")
	id := x[len(x)-1] // use last split item
	label := besEventIds["Unknown"]
	if promId, ok := besEventIds[id]; ok {
		label = promId
	} else {
		// Make note of this condition. Probably means .proto file definition was updated.
		fmt.Printf("Detected unknown event id: %s\n\n", id)
	}
	s.besEvents.bazelEventsTotal.ctr.WithLabelValues(label).Inc()
}
