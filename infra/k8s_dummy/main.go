// Webserver that exposes metrics for testing k8s setups
package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/enfabrica/enkit/lib/server"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricPathVisits = promauto.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "k8s_dummy",
		Name: "path_visit_count",
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

func main() {
	flag.Parse()
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/printpath/", printPath)

	server.Run(mux, nil, nil)
}
