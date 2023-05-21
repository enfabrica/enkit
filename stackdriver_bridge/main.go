package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/enfabrica/enkit/lib/protocfg"
	"github.com/enfabrica/enkit/lib/server"
	"github.com/enfabrica/enkit/stackdriver_bridge/bridge"
	spb "github.com/enfabrica/enkit/stackdriver_bridge/proto"
)

var (
	configPath = flag.String("config", "", "Path to config")

	metricConfigCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "stackdriver_bridge",
		Name:      "config_last_apply_time",
		Help:      "Time a config application was last attempted, by outcome",
	},
		[]string{
			"outcome",
		},
	)
)

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	configStream, err := protocfg.FromFile[spb.Config](*configPath).LoadOnSignals(syscall.SIGHUP)
	exitIf(err)

	glog.Infof("Waiting for config...")
	config := <-configStream
	glog.Infof("Got config: %+v", config)

	currentBridge, err := bridge.New(ctx, config)
	exitIf(err)
	metricConfigCount.WithLabelValues("ok").SetToCurrentTime()

	go func() {
		currentBridge.Start(ctx)

	configLoop:
		for {
			select {
			case <-ctx.Done():
				currentBridge.Stop()
				break configLoop

			case config := <-configStream:
				currentBridge.Stop()
				newBridge, err := bridge.New(ctx, config)
				if err != nil {
					metricConfigCount.WithLabelValues("failed").SetToCurrentTime()
					glog.Errorf("Failed to create new Bridge from config: %v", err)
				} else {
					metricConfigCount.WithLabelValues("ok").SetToCurrentTime()
					currentBridge = newBridge
				}
				currentBridge.Start(ctx)
			}
		}
	}()

	go server.Run(ctx, mux, nil, nil)

	<-ctx.Done()
	currentBridge.Stop()
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}
