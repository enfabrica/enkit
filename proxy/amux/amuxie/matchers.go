package amuxie

import (
	"errors"
	"net"
	"net/http"

	"github.com/kataras/muxie"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var metricPortStrippingActionCount = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "enproxy",
	Subsystem: "port_stripping",
	Name:      "action_count",
	Help:      "Counts of the actions that the PortStripping hostname matcher took",
},
	[]string{
		"host",
		"action",
	},
)

type PortStripping struct {
	// Keep track of the host this is matching purely for metrics recording
	// purposes.
	host string
	// Underlying matcher to delegate to after (possibly) stripping the port off
	// the hostname
	m muxie.Matcher
}

func NewPortStripping(host string, wrapped muxie.Matcher) muxie.Matcher {
	return &PortStripping{
		host: host,
		m:    wrapped,
	}
}

func (ps *PortStripping) Match(req *http.Request) bool {
	stripped, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		addrError := &net.AddrError{}
		if errors.As(err, &addrError) {
			// SplitHostPort will fail if the address doesn't contain a port. Assume
			// in this case the original host had no port component to strip. While
			// this is too broad a check, it's better than nothing
			stripped = req.Host
			metricPortStrippingActionCount.WithLabelValues(ps.host, "noop")
		} else {
			// Some unknown error occurred; don't allow any matches
			metricPortStrippingActionCount.WithLabelValues(ps.host, "failed")
			return false
		}
	} else {
		metricPortStrippingActionCount.WithLabelValues(ps.host, "stripped")
	}
	// Modify the request so that if it is propagated to a reverse proxy, the
	// reverse proxy doesn't need to deal with detecting whether to strip the port
	// again.
	req.Host = stripped

	// Delegate to the downstream matcher
	return ps.m.Match(req)
}
