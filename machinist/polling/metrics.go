package polling

import (
	"bufio"
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net"
	"net/http"
	"os/exec"
	"strconv"
)

var (
	registerFailCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "machinist_register_fail",
		Help: "The number of times the machine has failed to re register itself",
	})
	keepAliveErrorCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "machinist_keepalive_fail",
		Help: "The amount of times keepalive has failed",
	})
	dmesgErrors = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "dmesg_errors",
		Namespace: "machinist",
		Help:      "Logs from dmesg",
	})
)

// SendMetricsRequest polls the controlplane for metrics as well as spin up prometheus' node exporter.
func SendMetricsRequest(ctx context.Context, c *config.Node) error {
	if !c.EnableMetrics {
		c.Root.Log.Infof("Metrics are disabled")
		return nil
	}
	go func() {
		cmd := exec.Command("dmesg", "-w", "--level=err")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Println(err)
		}
		err = cmd.Start()
		if err != nil {
			fmt.Println(err)
		}
		buf := bufio.NewReader(stdout) // Notice that this is not in a loop
		for {
			line, _, _ := buf.ReadLine()
			dmesgErrors.Inc()
			c.Root.Log.Infof(string(line))
		}
	}()

	h := promhttp.Handler()
	http.Handle("/metrics", h)
	return goroutine.WaitFirstError(func() error {
		return http.ListenAndServe(net.JoinHostPort("0.0.0.0", strconv.Itoa(c.MetricsPort)), h)
	})
}
