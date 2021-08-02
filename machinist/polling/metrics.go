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
	"go.uber.org/atomic"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"time"
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
)

// SendMetricsRequest polls the controlplane for metrics as well as spin up prometheus' node exporter.
func SendMetricsRequest(ctx context.Context, c *config.Node) error {
	if !c.EnableMetrics {
		c.Root.Log.Infof("Metrics are disabled")
		return nil
	}
	go func() {
		numErr := 0.0
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "dmesg_errors",
			Namespace: "machinist",
			Help:      "Logs from dmesg",
		}, func() float64 {
			return numErr
		})
		averageErr := atomic.NewFloat64(0.0)
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "dmesg_errors_per_second",
			Namespace: "machinist",
			Help:      "Logs from dmesg",
		}, func() float64 {
			return averageErr.Load()
		})

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
		go func() {
			for {
				_ = <- time.After(1 * time.Second)
				averageErr.Store(0.0)
			}
		}()
		for {
			line, _, _ := buf.ReadLine()
			numErr += 1
			averageErr.Add(1.0)
			c.Root.Log.Infof(string(line))
		}
	}()

	h := promhttp.Handler()
	http.Handle("/metrics", h)
	return goroutine.WaitFirstError(func() error {
		return http.ListenAndServe(net.JoinHostPort("0.0.0.0", strconv.Itoa(c.MetricsPort)), h)
	})
}
