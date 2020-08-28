package httpp

import (
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/kataras/muxie"

	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Flags struct {
	*oauth.ExtractorFlags // OK

	Name   string // OK
	Config []byte // OK

	AuthURL string
}

func DefaultFlags() *Flags {
	return &Flags{
		ExtractorFlags: oauth.DefaultExtractorFlags(),
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.StringVar(&f.AuthURL, "auth-url", "", "Where to redirect users for authentication")
	set.ByteFileVar(&f.Config, "config", f.Name, "Default config file location.", kflags.WithFilename(&f.Name))

	f.ExtractorFlags.Register(set, "")
	return f
}

type Proxy struct {
	// Public, as it provides the ServeHTTP method, needed to serve the proxy.
	*muxie.Mux
	// List of domains for which an SSL certificate would be needed.
	Domains []string

	extractor *oauth.Extractor
	config    *Config
	log       logger.Logger

	cachedir     string
	httpAddress  string
	httpsAddress string
	authURL      *url.URL
}

type AuthenticatedProxy struct {
	Proxy     http.Handler
	Extractor *oauth.Extractor
	AuthURL   *url.URL
}

func (as *AuthenticatedProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	creds, err := as.Extractor.GetCredentialsFromRequest(r)
	if creds != nil && err == nil {
		r = r.WithContext(oauth.SetCredentials(r.Context(), creds))
		as.Proxy.ServeHTTP(w, r)
		return
	}

	if as.AuthURL == nil {
		http.Error(w, "Who are you? Sorry, you have no authentication cookie, and there is no authentication service configured", http.StatusUnauthorized)
		return
	}

	rurl := khttp.RequestURL(r)
	_, redirected := rurl.Query()["_redirected"]
	if redirected {
		http.Error(w, "You have been redirected back to this url (%s) - but you still don't have an authentication token.<br />"+
			"As a sentinent web server, I've decided that you human don't deserve any further redirect, as that would cause a loop<br />"+
			"which would be bad for the future of the internet, my load, and your bandwidth. Hit refresh if you want, but there's likely<br />"+
			"something wrong in your cookies, or your setup", http.StatusInternalServerError)
		return
	}

	rurl.RawQuery = khttp.JoinURLQuery(rurl.RawQuery, "_redirected")
	target := *as.AuthURL
	target.RawQuery = khttp.JoinURLQuery(target.RawQuery, "r="+url.QueryEscape(rurl.String()))
	http.Redirect(w, r, target.String(), http.StatusTemporaryRedirect)
}

func (p *Proxy) CreateServer(mapping *Mapping) (http.Handler, error) {
	proxy, err := NewProxy(mapping.From.Path, mapping.To, mapping.Transform)
	if err != nil {
		return nil, err
	}
	if mapping.Auth == MappingPublic {
		return proxy, nil
	}

	return &AuthenticatedProxy{AuthURL: p.authURL, Proxy: proxy, Extractor: p.extractor}, nil
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

func WithConfig(config *Config) Modifier {
	return func(p *Proxy) error {
		p.config = config
		return nil
	}
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

func WithExtractor(extractor *oauth.Extractor) Modifier {
	return func(p *Proxy) error {
		p.extractor = extractor
		return nil
	}
}

func FromFlags(fl *Flags) Modifier {
	return func(p *Proxy) error {
		extractor, err := oauth.NewExtractor(oauth.WithExtractorFlags(fl.ExtractorFlags))
		if err != nil {
			return err
		}
		p.extractor = extractor

		var config Config
		if err := marshal.UnmarshalDefault(fl.Name, fl.Config, marshal.Json, &config); err != nil {
			return kflags.NewUsageError(err)
		}

		if len(config.Mapping) <= 0 {
			return kflags.NewUsageError(fmt.Errorf("invalid config: it has no mappings"))
		}

		if fl.AuthURL == "" {
			return kflags.NewUsageError(fmt.Errorf("must specify --auth-url parameter"))
		}
		authURL := fl.AuthURL
		if strings.Index(authURL, "//") < 0 {
			authURL = "https://" + authURL
		}

		u, err := url.Parse(authURL)
		if err != nil || u.Host == "" {
			return kflags.NewUsageError(fmt.Errorf("invalid url %s supplied with --auth-url: %w", fl.AuthURL, err))
		}

		mods := Modifiers{
			WithConfig(&config),
			WithAuthURL(u),
		}
		return mods.Apply(p)
	}
}

func New(mod ...Modifier) (*Proxy, error) {
	p := &Proxy{
		log: logger.Nil,
	}
	if err := Modifiers(mod).Apply(p); err != nil {
		return nil, err
	}

	p.log.Infof("Config is: %v", *p.config)
	mux, domains, err := BuildMux(nil, p.log, p.config.Mapping, p.CreateServer)
	if err != nil {
		return nil, err
	}

	p.Mux = mux
	p.Domains = append(domains, p.config.Domains...)

	return p, nil
}
