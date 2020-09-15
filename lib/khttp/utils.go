package khttp

import (
	"github.com/enfabrica/enkit/lib/logger"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Dumper is an http.Handler capable of logging the content of the request.
//
// Use it anywhere an http.Handler would be accepted (eg, with http.Serve,
// http.Handle, wrapping a Mux, ...).
//
// Example:
//    mux := http.NewServeMux()
//    ...
//    http.ListenAndServe(":8080", &Dumper{Real: mux, Log: log.Printf})
type Dumper struct {
	Real http.Handler
	Log  logger.Printer
}

func (d *Dumper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	LogRequest(d.Log, r)
	d.Real.ServeHTTP(w, r)
}

func LogRequest(log logger.Printer, r *http.Request) {
	log("REQUEST %s", r.Method)
	log(" - host %s", r.Host)
	log(" - url %s", r.URL)
	log(" - headers")
	for key, value := range r.Header {
		log("   - %s: %s", key, value)
	}
}

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

// LooselyGetHost is a version of net.SplitHostPort which performs no checks, and returns
// the host part only of an hostport pair.
func LooselyGetHost(hostport string) string {
	hoststart, hostend := 0, 0
	if len(hostport) >= 1 && hostport[0] == '[' {
		hoststart = 1
		hostend = strings.IndexByte(hostport, ']')
	} else {
		hostend = strings.IndexByte(hostport, ':')
	}
	if hostend < 0 {
		hostend = len(hostport)
	}
	return hostport[hoststart:hostend]
}

// OptionallyJoinHostPort is like net.JoinHostPort, but the port is added only if != 0.
func OptionallyJoinHostPort(host string, port int) string {
	is_ipv6 := strings.IndexByte(host, ':') >= 0
	has_port := port > 0
	if is_ipv6 {
		host = "[" + host + "]"
	}
	if has_port {
		host += ":" + strconv.Itoa(port)
	}
	return host
}

// SplitHostPort is like net.SplitHostPort, but the port is optional.
// If no port is specified, an empty string will be returned.
func SplitHostPort(hostport string) (host, port string, err error) {
	addrErr := func(addr, why string) (host, port string, err error) {
		return "", "", &net.AddrError{Err: why, Addr: addr}
	}

	hoststart, hostend := 0, 0
	portstart := len(hostport)
	if len(hostport) >= 1 && hostport[0] == '[' {
		hoststart = 1
		hostend = strings.IndexByte(hostport, ']')
		if hostend < 0 {
			return addrErr(hostport, "missing ']' in address")
		}
		portstart = hostend + 1
	} else {
		hostend = strings.IndexByte(hostport, ':')
		if hostend < 0 {
			hostend = len(hostport)
		}
		portstart = hostend
	}
	if portstart < len(hostport) {
		if hostport[portstart] != ':' {
			return addrErr(hostport, "invalid character at the end of address, expecting ':'")
		}
		portstart += 1
	}

	port = hostport[portstart:]
	host = hostport[hoststart:hostend]

	if strings.IndexByte(port, ':') >= 0 {
		return addrErr(hostport, "too many colons in suspected port number")
	}
	if strings.IndexByte(port, ']') >= 0 {
		return addrErr(hostport, "unexpected ']' in port")
	}
	if strings.IndexByte(port, '[') >= 0 {
		return addrErr(hostport, "unexpected '[' in port")
	}
	if strings.IndexByte(host, '[') >= 0 {
		return addrErr(hostport, "unexpected '[' in host")
	}
	if strings.IndexByte(host, ']') >= 0 {
		return addrErr(hostport, "unexpected ']' in host")
	}

	return host, port, nil
}
