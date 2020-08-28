package khttp

import (
	"path"
	"strings"
	"net/url"
	"net/http"
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


