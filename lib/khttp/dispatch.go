package khttp

import (
	"github.com/enfabrica/enkit/lib/multierror"

	"net/http"
	"strings"
	"fmt"
)

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

type HostDispatch struct {
	Host string
	Handler http.Handler
}

func NewHostDispatcher(todispatch []HostDispatch) (HostDispatcher, error) {
	hd := HostDispatcher{}
	var errs []error

	add := func (ix int, host string, handler http.Handler) {
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
			add(ix, host + ".", entry.Handler)
		} else {
			add(ix, host + ":" + port, entry.Handler)
			add(ix, host + ".:" + port, entry.Handler)
		}
	}
	return hd, multierror.New(errs)
}
