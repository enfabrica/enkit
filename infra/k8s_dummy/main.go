// Webserver that exposes metrics for testing k8s setups
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/enfabrica/enkit/lib/server"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricPathVisits = promauto.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "k8s_dummy",
		Name:      "path_visit_count",
	},
		[]string{
			"path",
		},
	)
)

func printPath(w http.ResponseWriter, r *http.Request) {
	metricPathVisits.WithLabelValues(r.URL.Path).Inc()
	w.WriteHeader(200)
	fmt.Fprintf(w, "You reached page: %s\n", r.URL.Path)
}

func logHeartbeat(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	i := 0

	for {
		select {
		case <-t.C:
			log.Printf("Log heartbeat %d", i)
			i++
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Generate logs to ensure that logs collection works as expected
	go logHeartbeat(ctx, 60*time.Second)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/printpath/", printPath)

	server.Run(ctx, mux, nil, nil)
}
