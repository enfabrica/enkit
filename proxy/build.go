package main

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/kataras/muxie"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

// JoinURLQuery takes two escaped query strings (eg, what follows after the ? in a URL)
// and joins them into one query string.
func JoinURLQuery(q1, q2 string) string {
	if q1 == "" || q2 == "" {
		return q1 + q2
	}

	return q1 + "&" + q2
}

// CleanPreserve cleans an URL path (eg, eliminating .., //, useless . and so on) while
// preserving the '/' at the end of the path (path.Clean eliminates trailing /) and
// returning an empty string "" instead of . for an empty path.
func CleanPreserve(urlpath string) string {
	cleaned := path.Clean(urlpath)
	if cleaned == "." {
		cleaned = ""
	}

	if strings.HasSuffix(urlpath, "/") && !strings.HasSuffix(cleaned, "/") {
		return cleaned + "/"
	}
	return cleaned
}

// JoinPreserve joins multiple path fragments with one another, while preserving the final '/',
// if any. JoinPreserve internally calls path.Clean.
func JoinPreserve(add ...string) string {
	result := path.Join(add...)
	if strings.HasSuffix(add[len(add)-1], "/") && !strings.HasSuffix(result, "/") {
		return result + "/"
	}
	return result
}

// RequestURL approximates the URL the browser requested from an http.Request.
//
// Note that RequestURL can only return an approximation: it assumes that if the
// connection was encrypted it must have been done using https, while if it wasn't,
// it must have been done via HTTP.
//
// Further, most modern deployments rely on reverse proxies and load balancers.
// Any one of those things may end up mingling with the request headers, so by
// the time RequestURL is called, who knows what the browser actually supplied.
func RequestURL(req *http.Request) *url.URL {
	u := *req.URL
	if u.Host == "" {
		u.Host = req.Host
	}

	if req.TLS != nil {
		u.Scheme = "https"
	} else {
		u.Scheme = "http"
	}

	return &u
}

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
		req.URL.RawQuery = JoinURLQuery(toQuery, req.URL.RawQuery)

		maintain := false
		for _, t := range transform {
			maintain = t.Apply(req) || maintain
		}

		if !maintain {
			// We have a request for /foo/bar/baz, /foo/bar is mapped to /map, resulting url should be /map/baz.
			//
			// Pseudo code:
			// - Strip the From Path from the To path.
			cleaned := CleanPreserve(req.URL.Path)
			req.URL.Path = strings.TrimPrefix(cleaned, fromStripped)
		}
		req.URL.Path = JoinPreserve(to.Path, req.URL.Path)
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

	domains := []string{}
	for host, mappings := range hosts {
		hmux := mux
		if host != "" {
			hmux = muxie.NewMux()
		}
		for ix, mapping := range mappings {
			proxy, err := creator(mapping)
			if err != nil {
				return nil, domains, fmt.Errorf("error in mapping entry %d - %w", ix, err)
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
			domains = append(domains, host)
			mux.HandleRequest(muxie.Host(host), hmux)
		}
	}
	return mux, domains, multierror.New(errs)
}
