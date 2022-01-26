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
}

// Register all service stats metrics with Prometheus.
func (s *serviceStats) registerPrometheusMetrics() {
	prometheus.MustRegister(s.buildsTotal)
	prometheus.MustRegister(s.eventsTotal)
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
