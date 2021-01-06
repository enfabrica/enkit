package khttp

import (
	"github.com/enfabrica/enkit/lib/multierror"

	"fmt"
	"net/http"
	"strings"
)

// HostDispatcher is an http.Handler (implements ServeHTTP) that invokes
// a different http.Handler based on the host specified in the HTTP request.
//
// It complements http.ServeMUX in the sense that http.ServeMUX invokes different
// http.Handler based on the path of the request, while HostDispatcher invokes
// them based on the host header.
//
// By combining the two, you can easily create handlers with multiple virtual
// hosts involved.
//
type HostDispatcher map[string]http.Handler

func (hd HostDispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := strings.ToLower(r.Host)
	handler, found := hd[host]
	if found {
		handler.ServeHTTP(w, r)
		return
	}

	handler, found = hd[""]
	if found {
		handler.ServeHTTP(w, r)
		return
	}

	http.Error(w, fmt.Sprintf("Host '%s' not found", r.Host), http.StatusNotFound)
}

// HostDispatch mapping. Maps a string to a http handler.
//
// If the host is empty, the corresponding http.Handler will be invoked when the
// host is unknown, as well as for every host that has not been configured explicitly.
type HostDispatch struct {
	// Which host the client has to request in the Host HTTP header for the Handler
	// below to be invoked. If empty, the handler will be invoked for every request
	// for which a direct match cannot be found.
	//
	// Matching is case insensitive. Domains with a period appended are considered
	// equivalent to domains without a period. Example: www.mydomain.com is considered
	// equivalent to www.mydomain.com. (trailing period).
	//
	// The Host string can also specify a port number for example: www.mydomain.com:1001.
	// Port 80 or 443 are stripped by default and matched without port, as the RFC
	// recommandation is that port 80 and 443 are to be stripped in host headers.
	Host string

	// Handler is the handler to invoke for all requests matching this host.
	Handler http.Handler
}

// NewHostDispatcher creates a new HostDispatcher.
func NewHostDispatcher(todispatch []HostDispatch) (HostDispatcher, error) {
	hd := HostDispatcher{}
	var errs []error

	add := func(ix int, host string, handler http.Handler) {
		_, found := hd[host]
		if found {
			errs = append(errs, fmt.Errorf("entry %d (host %s): already mapped", ix, host))
			return
		}
		hd[host] = handler
	}

	for ix, entry := range todispatch {
		if entry.Host == "" {
			add(ix, "", entry.Handler)
			continue
		}
		host, port, err := SplitHostPort(entry.Host)
		if err != nil {
			errs = append(errs, fmt.Errorf("entry %d (host %s): %s", ix, entry.Host, err))
			continue
		}
		host = strings.ToLower(strings.TrimSuffix(host, "."))
		if port == "443" || port == "80" || port == "" {
			add(ix, host, entry.Handler)
			add(ix, host+".", entry.Handler)
		} else {
			add(ix, host+":"+port, entry.Handler)
			add(ix, host+".:"+port, entry.Handler)
		}
	}
	return hd, multierror.New(errs)
}
