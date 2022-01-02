// Package enproxy provides a complete proxy implementation with support
// for HTTP, HTTP/2, and NASSH, with OAUTH authentication, all in a simple
// API to use.
//
// This package glues together the default go net/http/httputil ReverseProxy
// packaged in proxy/httpp and the SSH over HTTPs implementation in proxy/nasshp
// together witha frontend server implemented using net/http, packaged in
// lib/khttp.
//
// The simplest use of this library is via flags:
//
//    import (
//        // Secure random numbers.
//        "github.com/enfabrica/enkit/lib/srand"
//        "github.com/enfabrica/enkit/lib/kflags"
//        "flag"
//    )
//
//    flags := enproxy.DefaultFlags()
//    flags.Register(&kflags.GoFlagSet{FlagSet: flag.CommandLine})
//
//    // Parse flags after registering them!!
//    flag.Parse()
//
//    rng := rand.New(srand.Source)
//    proxy, err := enproxy.New(rng, enproxy.FromFlags(flags))
//    if err != nil {
//      ...
//    }
//
//    proxy.Run()
//
// You can, of course, create a proxy manually with the desired options.
// In that case, you want to use `WithConfig` and other `With.*` modifiers
// to set all the desired options.
//
package enproxy

import (
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/proxy/httpp"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"github.com/enfabrica/enkit/proxy/utils"
	"log"
	"math/rand"
	"net/http"
)

// Config is the content of the proxy configuration file.
type Config struct {
	// Which URLs to map to which other URLs.
	Mapping []httpp.Mapping
	// Extra domains for which to obtain a certificate.
	Domains []string
	// List of allowed tunnels.
	Tunnels []string
}

// Warnings represents a list of warnings.
type Warnings []string

// Add adds a new warning.
func (w *Warnings) Add(warning string) {
	(*w) = append(*w, warning)
}

// Print prints the list of warnings.
//
// For example:
//   warnings.Print(log.Printf)
// or:
//   warnings.Print(klogger.Warnf)
func (w *Warnings) Print(printer logger.Printer) {
	for _, warn := range *w {
		printer("%s", warn)
	}
}

// Parse verifies and indexes a loaded Config.
//
// Returns the parsed whitelist of tunnels allowed, followed by a list of warnings.
func (config *Config) Parse() (utils.PatternList, Warnings, error) {
	var warn Warnings

	if len(config.Mapping) <= 0 {
		return nil, warn, kflags.NewUsageErrorf("config file: has no Mapping(s) defined")
	}
	if len(config.Tunnels) <= 0 {
		warn.Add("config file: empty whitelist for tunnels - no tunnel will be allowed!")
	}
	wl, err := utils.NewPatternList(config.Tunnels)
	if err != nil {
		return nil, warn, kflags.NewUsageErrorf("config file: illegal patterns specified in tunnels: %s", err)
	}

	return wl, warn, nil
}

// Flags represents command line flags necessary to define a proxy.
type Flags struct {
	Http  *khttp.Flags
	Oauth *oauth.RedirectorFlags
	Nassh *nasshp.Flags

	ConfigContent          []byte
	ConfigName             string
	DisabledAuthentication bool
}

// DefaultFlags returns the default flags.
//
// The default is generally a valid, working, one except for mandatory
// configuration parameters.
func DefaultFlags() *Flags {
	fl := &Flags{
		Http:  khttp.DefaultFlags(),
		Oauth: oauth.DefaultRedirectorFlags(),
		Nassh: nasshp.DefaultFlags(),
	}
	return fl
}

// Register register the flags necessary to configure enproxy.
func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	fl.Http.Register(set, prefix)
	fl.Oauth.Register(set, prefix)
	fl.Nassh.Register(set, prefix)

	set.ByteFileVar(&fl.ConfigContent, "config", fl.ConfigName, "Default config file location.", kflags.WithFilename(&fl.ConfigName))
	set.BoolVar(&fl.DisabledAuthentication, "without-authentication", false, "allow tunneling even without authentication")

	return fl
}

// Starter is a function capable of starting a web server.
//
// Requires providing a logger, an http.Handler (typically some form of mux), and
// a list of domains for which an https certificate is necessary.
type Starter func(log logger.Printer, handler http.Handler, domains ...string) error

type Options struct {
	log     logger.Logger
	starter Starter

	config Config

	pmods []httpp.Modifier
	nmods []nasshp.Modifier

	authenticate               oauth.Authenticate
	withoutNasshAuthentication bool
}

type Modifier func(opt *Options) error
type Modifiers []Modifier

func (mods Modifiers) Apply(o *Options) error {
	for _, m := range mods {
		if err := m(o); err != nil {
			return err
		}
	}
	return nil
}

func WithConfig(config Config) Modifier {
	return func(op *Options) error {
		op.config = config
		return nil
	}
}

func WithDisabledNasshAuthentication(disabled bool) Modifier {
	return func(op *Options) error {
		op.withoutNasshAuthentication = disabled
		return nil
	}
}

func WithAuthenticator(auth oauth.Authenticate) Modifier {
	return func(op *Options) error {
		op.authenticate = auth
		return nil
	}
}

func WithHttpStarter(starter Starter) Modifier {
	return func(op *Options) error {
		op.starter = starter
		return nil
	}
}

func WithHttpFlags(flags *khttp.Flags) Modifier {
	return func(op *Options) error {
		server, err := khttp.FromFlags(flags)
		if err != nil {
			return err
		}

		return WithHttpStarter(server.Run)(op)
	}
}

func WithProxyMods(pmods ...httpp.Modifier) Modifier {
	return func(op *Options) error {
		op.pmods = append(op.pmods, pmods...)
		return nil
	}
}

func WithNasshpMods(nmods ...nasshp.Modifier) Modifier {
	return func(op *Options) error {
		op.nmods = append(op.nmods, nmods...)
		return nil
	}
}

func WithOauthRedirector(rflags *oauth.RedirectorFlags) Modifier {
	return func(op *Options) error {
		redirector, err := oauth.NewRedirector(oauth.WithRedirectorFlags(rflags))
		if err != nil {
			return err
		}
		if err := WithAuthenticator(redirector.Authenticate)(op); err != nil {
			return err
		}

		pmods := []httpp.Modifier{
			httpp.WithStripCookie([]string{redirector.CredentialsCookieName()}),
		}
		return WithProxyMods(pmods...)(op)
	}

}

func WithLogging(logger logger.Logger) Modifier {
	return func(op *Options) error {
		op.log = logger
		return nil
	}
}

func FromFlags(flags *Flags) Modifier {
	return func(op *Options) error {
		var config Config
		if len(flags.ConfigContent) <= 0 {
			return kflags.NewUsageErrorf("Config file is empty, or no config file specified. Check the --config flag.")
		}
		if err := marshal.UnmarshalDefault(flags.ConfigName, flags.ConfigContent, marshal.Json, &config); err != nil {
			return kflags.NewUsageErrorf("Invalid configuration file '%s': %w", flags.ConfigName, err)
		}

		if flags.Oauth.AuthURL != "" && !flags.DisabledAuthentication {
			if err := WithOauthRedirector(flags.Oauth)(op); err != nil {
				return err
			}
		}

		if err := WithNasshpMods(nasshp.FromFlags(flags.Nassh))(op); err != nil {
			return err
		}

		if err := WithHttpFlags(flags.Http)(op); err != nil {
			return err
		}

		return WithConfig(config)(op)
	}
}

type Enproxy struct {
	log logger.Logger

	mux     http.Handler
	domains []string
	starter Starter
}

func New(rng *rand.Rand, mods ...Modifier) (*Enproxy, error) {
	op := &Options{
		log:     &logger.DefaultLogger{Printer: log.Printf},
		starter: khttp.DefaultServer().Run,
	}
	if err := Modifiers(mods).Apply(op); err != nil {
		return nil, err
	}

	wl, warns, err := op.config.Parse()
	if err != nil {
		return nil, err
	}
	warns.Print(op.log.Warnf)

	pmods := []httpp.Modifier{httpp.WithLogging(op.log), httpp.WithAuthenticator(op.authenticate)}
	hproxy, err := httpp.New(op.config.Mapping, append(pmods, op.pmods...)...)
	if err != nil {
		return nil, err
	}

	dispatcher := http.Handler(hproxy)
	if op.authenticate == nil && !op.withoutNasshAuthentication {
		op.log.Warnf("ssh gateway disabled as no authentication was configured")
	} else {
		authenticate := op.authenticate
		if op.withoutNasshAuthentication {
			op.log.Errorf("Watch out! The proxy is being started without authentication! SSH tunneling will rely entirely on a filmsy whitelist")
			authenticate = nil
		}

		nasshp, err := nasshp.New(rng, authenticate, append([]nasshp.Modifier{nasshp.WithFilter(wl.Allow), nasshp.WithLogging(op.log)}, op.nmods...)...)
		if err != nil {
			return nil, err
		}

		// Why is a new mux created? Why not re-use the mux in hproxy? Why the funky logic below with empty host names?
		//
		// The httpp package uses the muxie mux by default. This mux can match directly on host name, and is generally
		// great. Except it mangles the http request objects in such a way that gorilla/websocket fails to upgrade the
		// connection.
		//
		// To work around that issue, we use two muxes:
		// - one that dispatches based on host name, very simple, does not mangle the http request.
		//   The goal of this mux is to route connection requests to either the ssh handler, or http proxy handler.
		// - muxie, used by the proxy, to route all other requests.
		//
		// To support the two being configured on the same domain, or default domain, the muxie mux is configured
		// as a fallback to the ssh mux.
		mux := http.NewServeMux()
		nasshp.Register(mux.HandleFunc)
		mux.Handle("/", hproxy)

		hosts := []khttp.HostDispatch{
			{Host: nasshp.RelayHost(), Handler: mux},
		}
		if nasshp.RelayHost() != "" {
			hosts = append(hosts, khttp.HostDispatch{Handler: hproxy})
		}

		handler, err := khttp.NewHostDispatcher(hosts)
		if err != nil {
			return nil, err
		}
		dispatcher = handler
	}

	return &Enproxy{log: op.log, mux: dispatcher, domains: append(op.config.Domains, hproxy.Domains...), starter: op.starter}, nil
}

func (ep *Enproxy) Run() error {
	return ep.starter(ep.log.Infof, &khttp.Dumper{Real: ep.mux, Log: log.Printf}, ep.domains...)
}
