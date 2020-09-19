package httpp

import (
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/kataras/muxie"

	"fmt"
	"net/http"
	"net/url"
)

type Proxy struct {
	// Public, as it provides the ServeHTTP method, needed to serve the proxy.
	*muxie.Mux
	Domains []string

	authenticator oauth.Authenticate
	mapping       []Mapping
	log           logger.Logger

	stripCookie  []string
	cachedir     string
	httpAddress  string
	httpsAddress string
	authURL      *url.URL
}

type AuthenticatedProxy struct {
	Proxy         http.Handler
	Authenticator oauth.Authenticate
	AuthURL       *url.URL
}

func (as *AuthenticatedProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	creds, err := as.Authenticator(w, r, oauth.CreateRedirectURL(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if creds == nil {
		return
	}

	as.Proxy.ServeHTTP(w, r.WithContext(oauth.SetCredentials(r.Context(), creds)))
}

func (p *Proxy) CreateServer(mapping *Mapping) (http.Handler, error) {
	// Ensure that default transforms are applied.
	transform := mapping.Transform
	if transform == nil {
		transform = &Transform{}
	}

	if len(p.stripCookie) > 0 {
		transform.StripCookie = append(transform.StripCookie, p.stripCookie...)
	}

	proxy, err := NewProxy(mapping.From.Path, mapping.To, transform)
	if err != nil {
		return nil, err
	}
	if mapping.Auth == MappingPublic {
		return proxy, nil
	}
	if p.authenticator == nil {
		return nil, fmt.Errorf("proxy for mapping %v requires authentication - but no authentication configured", *mapping)
	}

	return &AuthenticatedProxy{AuthURL: p.authURL, Proxy: proxy, Authenticator: p.authenticator}, nil
}

type Modifier func(*Proxy) error

type Modifiers []Modifier

func (mods Modifiers) Apply(p *Proxy) error {
	for _, m := range mods {
		if err := m(p); err != nil {
			return err
		}
	}
	return nil
}

func WithLogging(log logger.Logger) Modifier {
	return func(p *Proxy) error {
		p.log = log
		return nil
	}
}

func WithAuthURL(u *url.URL) Modifier {
	return func(p *Proxy) error {
		p.authURL = u
		return nil
	}
}

func WithStripCookie(toStrip []string) Modifier {
	return func(p *Proxy) error {
		p.stripCookie = toStrip
		return nil
	}
}

func WithAuthenticator(authenticator oauth.Authenticate) Modifier {
	return func(p *Proxy) error {
		p.authenticator = authenticator
		return nil
	}
}

func New(mapping []Mapping, mod ...Modifier) (*Proxy, error) {
	p := &Proxy{
		log: logger.Nil,
	}
	if err := Modifiers(mod).Apply(p); err != nil {
		return nil, err
	}

	p.log.Infof("Mappings are: %v", mapping)
	mux, domains, err := BuildMux(nil, p.log, mapping, p.CreateServer)
	if err != nil {
		return nil, err
	}

	p.Mux = mux
	p.Domains = domains
	return p, nil
}
