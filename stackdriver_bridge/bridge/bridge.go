package bridge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	gometrics "github.com/hashicorp/go-metrics"
	promapi "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/model"

	spb "github.com/enfabrica/enkit/stackdriver_bridge/proto"
	"github.com/enfabrica/enkit/stackdriver_bridge/sinks"
	"github.com/enfabrica/enkit/stackdriver_bridge/sources"
)

var (
	metricQueryTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "stackdriver_bridge",
		Name:      "query_time_seconds",
		Buckets:   prometheus.ExponentialBucketsRange(1e-3, 10, 12),
		Help:      "Total query time, by name and query outcome",
	},
		[]string{
			"query_name",
			"outcome",
		},
	)
	metricWarningCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "stackdriver_bridge",
		Name:      "promql_warning_count",
		Help:      "Warnings returned for PromQL queries, by query",
	},
		[]string{
			"query_name",
		},
	)
)

// Bridge performs queries for multiple PromQL queries and updates the sink with
// a timeseries for each resulting combination of labels in the response.
type Bridge struct {
	config   *spb.ScrapeConfig
	src      sources.PromQL
	dst      gometrics.MetricSink
	stopFunc func()
}

// New returns a bridge that will start handling uploads for all the queries
// listed in the config. The returned Bridge is inactive until Start() is
// called.
func New(ctx context.Context, config *spb.Config) (*Bridge, error) {
	if err := checkConfig(config); err != nil {
		return nil, fmt.Errorf("config fails validation: %w", err)
	}

	sc := config.GetScrapeConfigs()[0]
	src, err := sources.OpenPromQL(sc.GetEndpoint())
	if err != nil {
		return nil, fmt.Errorf("failed to open PromQL endpoint %q: %w", sc.GetEndpoint(), err)
	}

	dst, err := sinks.NewStackdriver(ctx, config.GetGcpProjectId())
	if err != nil {
		return nil, fmt.Errorf("while opening Stackdriver sink: %w", err)
	}

	return &Bridge{
		config: config.GetScrapeConfigs()[0],
		src:    src,
		dst:    dst,
	}, nil
}

// Start starts polling all the configured PromQL queries.
func (b *Bridge) Start(ctx context.Context) {
	b.Stop()
	ctx, b.stopFunc = context.WithCancel(ctx)
	for _, metric := range b.config.GetMetrics() {
		go b.metricLoop(ctx, metric)
	}
}

// Stop stops polling all configured PromQL queries.
func (b *Bridge) Stop() {
	if b.stopFunc != nil {
		b.stopFunc()
	}
}

func (b *Bridge) metricLoop(ctx context.Context, metric *spb.Metric) {
	tmr := time.NewTicker(metric.GetScrapeTimeout().AsDuration())
	defer tmr.Stop()

	metricName := uniqueMetricName(metric)

reportLoop:
	for {
		select {
		case <-ctx.Done():
			return

		case <-tmr.C:
		}

		start := time.Now()

		res, warnings, err := b.src.Query(ctx, metric.GetQuery(), time.Now(), promapi.WithTimeout(metric.GetScrapeTimeout().AsDuration()))
		if err != nil {
			metricQueryTime.WithLabelValues(metricName, "query_failure").Observe(time.Now().Sub(start).Seconds())
			glog.Errorf("Query failed for metric %q: %v", metricName, err)
			glog.V(2).Infof("Error response: %+v", res)
			continue
		}
		if len(warnings) > 0 {
			metricWarningCount.WithLabelValues(metricName).Add(float64(len(warnings)))
			glog.Warningf("Query for metric %q caused %d warnings:", metric.GetName(), len(warnings))
			for i, w := range warnings {
				glog.Warningf("\t[%d]: %s", i, w)
			}
		}
		switch val := res.(type) {
		default:
			metricQueryTime.WithLabelValues(metricName, "bad_response_type").Observe(time.Now().Sub(start).Seconds())
			glog.Errorf("Unsupported response type: %T", res)
			continue reportLoop
		case model.Vector:
			metricQueryTime.WithLabelValues(metricName, "ok").Observe(time.Now().Sub(start).Seconds())
			for _, sample := range val {
				metricName := metric.GetName()
				labels := append(labelsFromMetric(sample.Metric), labelsFromMap(metric.GetExtraLabels())...)
				value := float32(sample.Value)
				glog.V(2).Infof("Writing metric %q %v = %v", metricName, labels, value)
				b.dst.SetGaugeWithLabels([]string{metricName}, value, labels)
			}
		}
	}
}

func uniqueMetricName(m *spb.Metric) string {
	var lbls []string
	for _, label := range m.GetExtraLabels() {
		lbls = append(lbls, label)
	}
	name := m.GetName()
	if len(lbls) > 0 {
		name = fmt.Sprintf("%s__%s", name, strings.Join(lbls, "_"))
	}
	return name
}

func labelsFromMap(m map[string]string) []gometrics.Label {
	var lbls []gometrics.Label
	for k, v := range m {
		lbls = append(lbls, gometrics.Label{Name: k, Value: v})
	}
	sort.Slice(lbls, func(i, j int) bool { return lbls[i].Name < lbls[j].Name })
	return lbls
}

func labelsFromMetric(metric model.Metric) []gometrics.Label {
	var lbls []gometrics.Label
	for k, v := range metric {
		lbls = append(lbls, gometrics.Label{Name: string(k), Value: string(v)})
	}
	sort.Slice(lbls, func(i, j int) bool { return lbls[i].Name < lbls[j].Name })
	return lbls
}

func checkConfig(config *spb.Config) error {
	if config.GetGcpProjectId() == "" {
		return fmt.Errorf("gcp_project_id must be set")
	}
	if len(config.GetScrapeConfigs()) != 1 {
		return fmt.Errorf("Only one scrape_configs entry is currently supported; got %d", len(config.GetScrapeConfigs()))
	}
	return nil
}
