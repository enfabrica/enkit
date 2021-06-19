package polling

import (
	"context"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net"
	"net/http"
	"strconv"
)

var (
	registerCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "machinist_register_fail",
		Help: "The number of times the machine has failed to re register itself",
	})
	keepAliveErrorCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "machinist_keepalive_fail",
		Help: "The amount of times keepalive has failed",
	})
)

// SendMetricsRequest polls the controlplane for metrics as well as spin up prometheus' node exporter.
func SendMetricsRequest(ctx context.Context, c config.Node) error {
	h := promhttp.Handler()
	http.Handle("/metrics", h)
	return goroutine.WaitFirstError(func() error {
		return http.ListenAndServe(net.JoinHostPort("0.0.0.0", strconv.Itoa(c.MetricsPort)), h)
	})
}
