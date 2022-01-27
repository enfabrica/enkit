package httpp

import (
	"crypto/tls"
	"fmt"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/proxy/amux"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func NewProxy(fromurl, tourl string, transform *Transform) (*httputil.ReverseProxy, error) {
	to, err := url.Parse(tourl)
	if err != nil {
		return nil, err
	}
	if transform == nil {
		transform = &Transform{}
	}

	if err := transform.Compile(fromurl, tourl); err != nil {
		return nil, err
	}

	toQuery := to.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = to.Scheme
		req.URL.Host = to.Host
		req.URL.RawQuery = khttp.JoinURLQuery(toQuery, req.URL.RawQuery)

		transform.Apply(req)

		req.URL.Path = khttp.JoinPreserve(to.Path, req.URL.Path)
		req.URL.RawPath = ""
	}

	proxy := &httputil.ReverseProxy{Director: director}
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return proxy, nil
}

type ProxyCreator func(m *Mapping) (http.Handler, error)

func PopulateMux(mux amux.Mux, log logger.Logger, mappings []Mapping, creator ProxyCreator) ([]string, error) {
	if log == nil {
		log = logger.Nil
	}

	hosts := map[string][]*Mapping{}
	var errs []error
	for ix, mapping := range mappings {
		fromHost := strings.TrimSpace(mapping.From.Host)
		hosts[fromHost] = append(hosts[fromHost], &mappings[ix])
	}

	add := func(host, from, to string, trans *Transform, mux amux.Mux, proxy http.Handler) {
		t := "default transforms"
		if trans != nil {
			t = fmt.Sprintf("%+v", trans)
		}
		log.Infof("Mapping: %s%s to %s (%+v)", host, from, to, t)
		mux.Handle(from, proxy)
	}

	dohttpps := []string{}
	for host, mappings := range hosts {
		hmux := mux
		if host != "" {
			hmux = mux.Host(host)
		}
		for ix, mapping := range mappings {
			proxy, err := creator(mapping)
			if err != nil {
				return dohttpps, fmt.Errorf("error in mapping entry %d - %w", ix, err)
			}
			path := mapping.From.Path
			if path == "" {
				path = "/"
			}
			if strings.HasSuffix(path, "/") {
				add(host, path, mapping.To, mapping.Transform, hmux, proxy)
				add(host, path+"*", mapping.To, mapping.Transform, hmux, proxy)
			} else {
				add(host, path, mapping.To, mapping.Transform, hmux, proxy)
			}
		}
		if host != "" {
			dohttpps = append(dohttpps, host)
		}
	}
	return dohttpps, multierror.New(errs)
}
