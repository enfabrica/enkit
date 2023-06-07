package main

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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

var (
	metricOutputFileTooBigTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "output_file_too_big_total",
			Help:      "Total output files not processed due to excessive size",
		},
		[]string{"filetype"},
	)
	metricBigqueryExceptionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bestie",
			Name:      "bigquery_exceptions_total",
			Help:      "Total BigQuery operational exceptions, tagged by type",
		},
		[]string{"type"},
	)
)

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
