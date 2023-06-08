package metrics

import (
	"net/http"
	"strconv"

	"github.com/enfabrica/enkit/lib/stamp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricBuildInfo = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "enfabrica",
		Subsystem: "bin",
		Name:      "build_time",
		Help:      "Info on when/where/how this binary was generated",
		ConstLabels: prometheus.Labels(map[string]string{
			"build_user":     stamp.BuildUser,
			"git_branch":     stamp.GitBranch,
			"git_sha":        stamp.GitSha,
			"git_master_sha": stamp.GitMasterSha,
			"is_clean":       strconv.FormatBool(stamp.IsClean()),
			"is_official":    strconv.FormatBool(stamp.IsOfficial()),
		}),
	})

	metricStartTime = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "enfabrica",
		Subsystem: "runtime",
		Name:      "start_time",
		Help:      "When this instance started",
	})
)

func StartServer(hostPort string, endpoint string) {
	mux := http.NewServeMux()
	initMux(mux, endpoint)
	http.ListenAndServe(hostPort, mux)
}

func AddHandler(mux *http.ServeMux, endpoint string) {
	initMux(mux, endpoint)
}

func initMux(mux *http.ServeMux, endpoint string) {
	// Initialize common metrics
	metricBuildInfo.Set(float64(stamp.BuildTimestamp().UnixNano()) / 1e9)
	metricStartTime.SetToCurrentTime()

	mux.Handle(endpoint, promhttp.Handler())
}
