package httpp

import (
	"github.com/enfabrica/enkit/lib/khttp"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type HostPath struct {
	Host, Path string
}

type Regex struct {
	Match, Sub string
	match      *regexp.Regexp
}

func (t *Regex) Compile() error {
	var err error
	t.match, err = regexp.Compile(t.Match)
	return err
}

func (t *Regex) Apply(req *http.Request) {
	t.match.ReplaceAllString(req.URL.Path, t.Sub)
	req.URL.RawPath = req.URL.EscapedPath()
}

type XForwardedForTreatment string

const (
	// Default behavior: ignore X-Forwarded-For from the client, but provide one to the backend.
	XForwardedForSet XForwardedForTreatment = "set"
	// X-Forwarded-For header is forwarded to the backend, with the ip of the client added.
	XForwardedForAdd = "add"
	// No X-Forwarded-For supplied to the backend. If there is one, it is stripped.
	XForwardedForNone = "none"
)

type Transform struct {
	// Apply a regular expression to adapt the resulting URL.
	UrlRegex []Regex
	// Maintain the original path of the proxy. Normally, it is stripped.
	// For example: if you map "proxy.address/path/p1/" to "backend.address/path2/", a request
	// for "proxy.address/path/p1/test" will by default land to "backend.address/path2/test".
	// If you set Maintain to true, it will instead land on "backend.address/path2/path/p1/test".
	Maintain bool

	// Defines what to do with the X-Forwarded-For header, and the IP of the client.
	XForwardedFor XForwardedForTreatment
	// List of regular expressions defining which cookies to strip in requests to the backend.
	StripCookie []string
	// By default, requests to the backend are forwarded with the Host field set to the
	// value of the From.Host map. You can override that value with SetHost.
	SetHost string

	stripCookie     []*regexp.Regexp
	noSlashFromPath string
}

func (t *Transform) Apply(req *http.Request) bool {
	if t.UrlRegex != nil {
		for _, regex := range t.UrlRegex {
			regex.Apply(req)
		}
	}
	if !t.Maintain {
		// We have a request for /foo/bar/baz, /foo/bar is mapped to /map, resulting url should be /map/baz.
		//
		// Pseudo code:
		// - Strip the From Path from the To path.
		cleaned := khttp.CleanPreserve(req.URL.Path)
		req.URL.Path = strings.TrimPrefix(cleaned, t.noSlashFromPath)
	}
	if len(t.stripCookie) > 0 {
		cookies := req.Cookies()
		req.Header.Del("Cookie")
		for _, cookie := range cookies {
			for _, strip := range t.stripCookie {
				if strip.MatchString(cookie.Name) {
					continue
				}
				req.AddCookie(cookie)
			}
		}
	}
	if t.SetHost != "" {
		req.Host = t.SetHost
	}

	switch t.XForwardedFor {
	case "":
		fallthrough
	case XForwardedForSet:
		req.Header.Del("X-Forwarded-For")
	case XForwardedForNone:
		req.Header["X-Forwarded-For"] = nil
	case XForwardedForAdd:
	}
	return t.Maintain
}

func (t *Transform) Compile(fromurl, tourl string) error {
	from, err := url.Parse(fromurl)
	if err != nil {
		return err
	}

	t.noSlashFromPath = strings.TrimSuffix(from.Path, "/")
	for _, regex := range t.UrlRegex {
		regex.Compile()
	}

	for _, strip := range t.StripCookie {
		compiled, err := regexp.Compile(strip)
		if err != nil {
			return err
		}
		t.stripCookie = append(t.stripCookie, compiled)
	}
	return nil
}

type MappingAuth string

const (
	MappingAuthenticated MappingAuth = ""
	MappingPublic        MappingAuth = "public"
)

type Mapping struct {
	From HostPath
	To   string

	Transform *Transform
	Auth      MappingAuth
}
