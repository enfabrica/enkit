package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"regexp"
	"strings"

	"time"

	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	errorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "probe_errors",
		Help: "Break down of errors encountered per target.",
	}, []string{"target", "error"})

	accumulatedDelay = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "probe_delay",
		Help: "Delay accumulated in probes due to blocking requests.",
	}, []string{"target"})

	exportedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "probe_count",
		Help: "Total number of probes run, and their status. A probe is considered failed if it does not complete within a pre-defined timeout.",
	}, []string{"target", "status"})

	exportedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "probe_sum",
		Help: "Total time to complete each probe, broken down by operation type.",
	}, []string{"target", "type"})

	exportedSummary = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "probe_summary",
		Help:       "Total time to complete each probe, broken down by operation type.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"target", "type"})

	exportedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "probe_gauge",
		Help: "Time to complete the last probe, broken down by operation type.",
	}, []string{"target", "type"})
)

var removeIPsAndDomains = regexp.MustCompile(`[a-zA-Z0-9_-]+(\.[a-zA-Z0-9_-]+)+[.]?(:[0-9]+)?`)

func FormatError(err error) string {
	return removeIPsAndDomains.ReplaceAllString(err.Error(), "${ip:port}")
}

func trace(transport *http.Transport, name, site string) {
	exportedCount.WithLabelValues(name, "attempted").Add(1)

	req, err := http.NewRequest("GET", site, nil)
	if err != nil {
		errorCount.WithLabelValues(name, FormatError(err)).Add(1)
		exportedCount.WithLabelValues(name, "internal-error").Add(1)
		return
	}

	var RunStart, DNSStart, ConnectStart, WriteStart, ReadStart time.Time
	var RunDuration, DNSDuration, ConnectDuration, WriteDuration, FirstReadDuration, FullReadDuration time.Duration
	var bodyBytes []byte

	RunStart = time.Now()
	defer func() {
		now := time.Now()
		RunDuration = now.Sub(RunStart)
		metrics := []struct {
			name  string
			start time.Time
			value time.Duration
		}{
			{"run", RunStart, RunDuration},
			{"dns", DNSStart, DNSDuration},
			{"connect", ConnectStart, ConnectDuration},
			{"write", WriteStart, WriteDuration},
			{"firstread", ReadStart, FirstReadDuration},
			{"fullread", ReadStart, FullReadDuration},
		}

		for _, metric := range metrics {
			if metric.start.IsZero() {
				log.Printf("skipping metric %s - still zero - %#v", metric.name, metric)
				continue
			}
			// This code is run as part of a defer, when an error may have happened.
			// Blame the time we spent until the error happened on any timer that was started
			// before the error happened.
			difference := metric.value
			if difference == 0 {
				difference = now.Sub(metric.start)
			}

			exportedSummary.WithLabelValues(name, metric.name).Observe(difference.Seconds())
			exportedGauge.WithLabelValues(name, metric.name).Set(difference.Seconds())
			exportedTotal.WithLabelValues(name, fmt.Sprintf("%s", metric.name)).Add(difference.Seconds())
		}

		log.Printf("%s - %d bytes - run:%s, dns:%s, connect:%s, write:%s, firstread:%s, fullread:%s",
			site, len(bodyBytes), RunDuration, DNSDuration, ConnectDuration, WriteDuration, FirstReadDuration, FullReadDuration)

	}()

	trace := &httptrace.ClientTrace{
		DNSStart: func(dnsInfo httptrace.DNSStartInfo) {
			DNSStart = time.Now()
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			DNSDuration = time.Since(DNSStart)
		},

		ConnectStart: func(network, addr string) {
			ConnectStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			ConnectDuration = time.Since(ConnectStart)
			WriteStart = time.Now()
		},

		WroteHeaders: func() {
			WriteDuration = time.Since(WriteStart)
			ReadStart = time.Now()
		},
		GotFirstResponseByte: func() {
			FirstReadDuration = time.Since(ReadStart)
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		errorCount.WithLabelValues(name, FormatError(err)).Add(1)
		exportedCount.WithLabelValues(name, "request-error").Add(1)
		return
	}
	exportedCount.WithLabelValues(name, fmt.Sprintf("status-%d", resp.StatusCode)).Add(1)

	defer resp.Body.Close()
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		errorCount.WithLabelValues(name, FormatError(err)).Add(1)
		exportedCount.WithLabelValues(name, "read-error").Add(1)
		return
	}
	FullReadDuration = time.Since(ReadStart)
}

func probe(p Probe) {
	if p.Interval <= 0 {
		p.Interval = 10 * time.Second
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = p.Address
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   p.Interval,
			KeepAlive: p.Interval,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		DisableKeepAlives:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	for {
		started := time.Now()
		trace(transport, p.Name, p.Address)

		elapsed := time.Now().Sub(started)
		difference := p.Interval - elapsed
		if difference > 0 {
			time.Sleep(difference)
		} else {
			difference = -1 * difference
			accumulatedDelay.WithLabelValues(p.Name).Add(difference.Seconds())
		}
	}
}

type Probe struct {
	Type string

	Name    string
	Address string

	Interval time.Duration
}

type Config struct {
	Probe []Probe
}

func main() {
	http.Handle("/metrics", promhttp.Handler())

	root := &cobra.Command{
		Use:           "monitor",
		Long:          `monitor - starts a prober to monitor remote endpoints`,
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		Example: `  $ monitor -p 8080 ./probes.toml
	To start a monitoring daemon running the probes defined in probes.cfg,
	providing a UI on port 8080.`,
	}

	port := 7777
	log := logger.Nil
	root.Flags().IntVarP(&port, "port", "p", port, "Port number on which the probing daemon will be listening for connections.")

	root.RunE = func(cmd *cobra.Command, args []string) error {
		log.Warnf("Listening on port %d", port)

		for _, arg := range args {
			var config Config
			err := marshal.UnmarshalFile(arg, &config)
			if err != nil {
				return err
			}

			for _, p := range config.Probe {
				go probe(p)
			}
		}

		return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}

	kcobra.RunWithDefaults(root, nil, &log)
}
