package httpp

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/slice"
)

type HostPath struct {
	Host, Path string
}

type Regex struct {
	Match, Sub string
	match      *regexp.Regexp
}

type HeaderGroupMapping struct {
	// Header which should get a value
	Header string
	// List of groups that should be tested. The header gets the value of the
	// first group that matches the current request. If no match occurs, the
	// request is rejected.
	GroupMapping []ValueByGroup
}

type ValueByGroup struct {
	// Name of the group this value should apply to. If set to the empty string,
	// should match all requests (as a sort of "default case").
	Group string
	// If the group specifier matches, this value is used.
	Value string
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
	XForwardedForAdd XForwardedForTreatment = "add"
	// No X-Forwarded-For supplied to the backend. If there is one, it is stripped.
	XForwardedForNone XForwardedForTreatment = "none"
)

type XWebauthTreatment string

const (
	// Default behavior: set no additional headers on the forwarded request.
	XWebauthNone XWebauthTreatment = "none"
	// Sets:
	//   * X-Webauth-Userid to the user's unique ID
	//   * X-Webauth-Username to the user's username
	//   * X-Webauth-Organization to the user's organization
	//   * X-Webauth-Globalname to the user's "global name" (see oauth.(*Identity).GlobalName)
	XWebauthSet XWebauthTreatment = "set"
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
	// Defines whether to set additional headers signaling the user's identity on
	// the request to the backend.
	XWebauth XWebauthTreatment
	// List of regular expressions defining which cookies to strip in requests to the backend.
	StripCookie []string
	// By default, requests to the backend are forwarded with the Host field set to the
	// value of the From.Host map. You can override that value with SetHost.
	SetHost string

	// Re-map request headers. If the string is empty, the header is stripped.
	// This is useful to propagate non-RFC compliant headers, or to strip headers.
	// For example, by setting MapRequestHeaders to "Sec-Websocket-Key" to "Sec-WebSocket-Key"
	// the case for WebSocket will be changed.
	MapRequestHeaders map[string]string

	// Add additional headers based on the groups the user is part of.
	// This is useful to conditionally set headers based on the user's identity -
	// for instance, to add an "X-Webauth-Role" to Grafana requests to indicate
	// the user's role in Grafana.
	//
	// TODO(INFRA-4919): Implement this feature
	MapRequestHeadersByGroup []HeaderGroupMapping

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

	for expected, desired := range t.MapRequestHeaders {
		if len(desired) <= 0 {
			delete(req.Header, expected)
			continue
		}

		value, found := req.Header[expected]
		if !found {
			continue
		}

		delete(req.Header, expected)
		req.Header[desired] = value
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

	switch t.XWebauth {
	case "":
		fallthrough
	case XWebauthNone:
		// No extra headers set
	case XWebauthSet:
		creds := oauth.GetCredentials(req.Context())
		if creds == nil {
			logger.GetCtx(req.Context()).Errorf("Missing credentials on request; can't set X-Webauth-* headers")
			break
		}
		req.Header.Set("X-Webauth-Userid", creds.Identity.Id)
		req.Header.Set("X-Webauth-Username", creds.Identity.Username)
		req.Header.Set("X-Webauth-Organization", creds.Identity.Organization)
		req.Header.Set("X-Webauth-Globalname", creds.Identity.GlobalName())
	}

	userGroups := map[string]struct{}{}
	creds := oauth.GetCredentials(req.Context())
	if creds != nil {
		userGroups = slice.ToSet(creds.Identity.Groups)
	}

nextHeader:
	for _, header := range t.MapRequestHeadersByGroup {
		for _, groupMap := range header.GroupMapping {
			// Empty group means this header value should always apply
			if groupMap.Group == "" {
				req.Header.Set(header.Header, groupMap.Value)
				continue nextHeader
			}

			if _, ok := userGroups[groupMap.Group]; ok {
				req.Header.Set(header.Header, groupMap.Value)
				continue nextHeader
			}
		}
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
