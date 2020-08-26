package main

import (
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/kirsle/configdir"
	"golang.org/x/crypto/acme/autocert"

	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Flags struct {
	*oauth.ExtractorFlags // OK

	HttpPort  int // OK
	HttpsPort int // OK
	Cache     string

	Name   string // OK
	Config []byte // OK

	AuthURL string
}

func DefaultFlags() *Flags {
	return &Flags{
		ExtractorFlags: oauth.DefaultExtractorFlags(),
		HttpPort:       9999,
		Cache:          configdir.LocalCache("enkit-certs"),
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.IntVar(&f.HttpPort, "http-port", f.HttpPort, "Port number on which the proxy will be listening for HTTP connections.")
	set.IntVar(&f.HttpsPort, "https-port", f.HttpsPort, "Port number on which the proxy will be listening for HTTPs connections.")

	set.StringVar(&f.AuthURL, "auth-url", "", "Where to redirect users for authentication")
	set.StringVar(&f.Cache, "cert-cache", f.Cache, "Location where certificates are cached.")
	set.ByteFileVar(&f.Config, "config", f.Name, "Default config file location.", kflags.WithFilename(&f.Name))

	f.ExtractorFlags.Register(set, "")
	return f
}

type Proxy struct {
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

	rurl := RequestURL(r)
	_, redirected := rurl.Query()["_redirected"]
	if redirected {
		http.Error(w, "You have been redirected back to this url (%s) - but you still don't have an authentication token.<br />"+
			"As a sentinent web server, I've decided that you human don't deserve any further redirect, as that would cause a loop<br />"+
			"which would be bad for the future of the internet, my load, and your bandwidth. Hit refresh if you want, but there's likely<br />"+
			"something wrong in your cookies, or your setup", http.StatusInternalServerError)
		return
	}

	rurl.RawQuery = JoinURLQuery(rurl.RawQuery, "_redirected")
	target := *as.AuthURL
	target.RawQuery = JoinURLQuery(target.RawQuery, "r="+url.QueryEscape(rurl.String()))
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

func (p *Proxy) Run() error {
	p.log.Infof("Config is: %v", *p.config)
	mux, domains, err := BuildMux(nil, p.log, p.config.Mapping, p.CreateServer)
	if err != nil {
		return err
	}

	if p.httpsAddress == "" {
		p.log.Infof("Listening on HTTP address %s", p.httpAddress)
		return http.ListenAndServe(p.httpAddress, mux)
	}

	p.log.Infof("Storing certificates in '%s'", p.cachedir)
	if p.cachedir != "" {
		if err := os.MkdirAll(p.cachedir, 0700); err != nil {
			return err
		}
	}

	// create the autocert.Manager with domains and path to the cache
	domains = append(domains, p.config.Domains...)
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache(p.cachedir),
	}

	// create the server itself
	server := &http.Server{
		Addr: p.httpsAddress,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
		Handler: mux,
	}

	p.log.Infof("Serving http/https for domains: %+v", domains)
	go func() {
		p.log.Infof("Listening on HTTP address %s", p.httpAddress)
		http.ListenAndServe(p.httpAddress, certManager.HTTPHandler(nil))
	}()

	p.log.Infof("Listening on HTTPs address %s", p.httpsAddress)
	return server.ListenAndServeTLS("", "")
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

func WithHttpsAddress(address string) Modifier {
	return func(p *Proxy) error {
		p.httpsAddress = address
		return nil
	}
}

func WithHttpsPort(port int) Modifier {
	return func(p *Proxy) error {
		return WithHttpsAddress(fmt.Sprintf(":%d", port))(p)
	}
}

func WithHttpAddress(address string) Modifier {
	return func(p *Proxy) error {
		p.httpAddress = address
		return nil
	}
}

func WithHttpPort(port int) Modifier {
	return func(p *Proxy) error {
		return WithHttpAddress(fmt.Sprintf(":%d", port))(p)
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

func WithCacheDir(dir string) Modifier {
	return func(p *Proxy) error {
		p.cachedir = dir
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
			WithHttpPort(fl.HttpPort),
			WithConfig(&config),
			WithAuthURL(u),
			WithCacheDir(fl.Cache),
		}
		if fl.HttpsPort > 0 {
			mods = append(mods, WithHttpsPort(fl.HttpsPort))
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

	return p, nil
}
