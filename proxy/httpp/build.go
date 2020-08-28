package httpp

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/kataras/muxie"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func NewProxy(fromurl, tourl string, transform []*Transform) (*httputil.ReverseProxy, error) {
	to, err := url.Parse(tourl)
	if err != nil {
		return nil, err
	}

	from, err := url.Parse(fromurl)
	if err != nil {
		return nil, err
	}

	for _, t := range transform {
		if err := t.Compile(fromurl, tourl); err != nil {
			return nil, err
		}
	}

	fromStripped := strings.TrimSuffix(from.Path, "/")
	toQuery := to.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = to.Scheme
		req.URL.Host = to.Host
		req.URL.RawQuery = khttp.JoinURLQuery(toQuery, req.URL.RawQuery)

		httpptain := false
		for _, t := range transform {
			httpptain = t.Apply(req) || httpptain
		}

		if !httpptain {
			// We have a request for /foo/bar/baz, /foo/bar is mapped to /map, resulting url should be /map/baz.
			//
			// Pseudo code:
			// - Strip the From Path from the To path.
			cleaned := khttp.CleanPreserve(req.URL.Path)
			req.URL.Path = strings.TrimPrefix(cleaned, fromStripped)
		}
		req.URL.Path = khttp.JoinPreserve(to.Path, req.URL.Path)
		req.URL.RawPath = ""
	}

	return &httputil.ReverseProxy{Director: director}, nil
}

type ProxyCreator func(m *Mapping) (http.Handler, error)

func BuildMux(mux *muxie.Mux, log logger.Logger, mappings []Mapping, creator ProxyCreator) (*muxie.Mux, []string, error) {
	if log == nil {
		log = logger.Nil
	}

	hosts := map[string][]*Mapping{}
	var errs []error
	for ix, mapping := range mappings {
		fromHost := strings.TrimSpace(mapping.From.Host)
		hosts[fromHost] = append(hosts[fromHost], &mappings[ix])
	}

	if mux == nil {
		mux = muxie.NewMux()
	}

	add := func(host, from, to string, mux *muxie.Mux, proxy http.Handler) {
		log.Infof("Mapping: %s%s to %s", host, from, to)
		mux.Handle(from, proxy)
	}

	dohttpps := []string{}
	for host, mappings := range hosts {
		hmux := mux
		if host != "" {
			hmux = muxie.NewMux()
		}
		for ix, mapping := range mappings {
			proxy, err := creator(mapping)
			if err != nil {
				return nil, dohttpps, fmt.Errorf("error in mapping entry %d - %w", ix, err)
			}
			path := mapping.From.Path
			if path == "" {
				path = "/"
			}
			if strings.HasSuffix(path, "/") {
				add(host, path, mapping.To, hmux, proxy)
				add(host, path+"*", mapping.To, hmux, proxy)
			} else {
				add(host, path, mapping.To, hmux, proxy)
			}
		}
		if host != "" {
			dohttpps = append(dohttpps, host)
			mux.HandleRequest(muxie.Host(host), hmux)
		}
	}
	return mux, dohttpps, multierror.New(errs)
}
