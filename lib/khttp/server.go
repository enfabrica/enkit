package khttp

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/kserver"
	"github.com/enfabrica/enkit/lib/khttp/ktls"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/kirsle/configdir"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type FuncHandler func(w http.ResponseWriter, r *http.Request)

type Flags struct {
	*ServerFlags
	*AutocertFlags

	HTTP *kserver.Flags
	TLS  *ktls.Flags
}

func DefaultFlags() *Flags {
	return &Flags{
		ServerFlags:   DefaultServerFlags(),
		AutocertFlags: DefaultAutocertFlags(),
		HTTP:          kserver.DefaultFlags(),
		TLS:           ktls.DefaultFlags(),
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	f.ServerFlags.Register(set, prefix)
	f.AutocertFlags.Register(set, prefix)

	f.HTTP.Register(set, prefix)
	f.TLS.Register(set, prefix)
	return f
}

func FromFlags(flags *Flags, doms ...string) Modifier {
	return func(opts *Server) error {
		if err := FromServerFlags(flags.ServerFlags)(opts); err != nil {
			return err
		}

		if err := FromAutocertFlags(flags.AutocertFlags, doms...)(opts); err != nil {
			return err
		}

		if err := WithServerOptions(kserver.FromFlags(flags.HTTP))(opts); err != nil {
			return err
		}

		if err := WithTLSOptions(ktls.FromFlags(flags.TLS))(opts); err != nil {
			return err
		}

		return nil
	}
}

type ServerFlags struct {
	HttpPort    int
	HttpAddress string

	HttpsPort    int
	HttpsAddress string
}

// EnvPort returns the port specified in the PORT env variable, or a default if not found.
//
// GCP, appengine, and a few other cloud or container environment default
// to reserving a port for the application and exporting it in the PORT environment
// variable. This function allows to use that port number as the default.
func EnvPort(ifnotfound int) int {
	iport := ifnotfound
	sport := os.Getenv("PORT")
	if sport != "" {
		pport, err := strconv.Atoi(sport)
		if err == nil && pport > 0 && pport <= 65535 {
			iport = pport
		}
	}

	return iport
}

var DefaultPort = EnvPort(9999)

var DefaultCache = configdir.LocalCache("enkit-certs")

// DefaultServerFlags returns a default flags object configured for http only.
//
// It also supplies other parameters so https can be enabled safely with
// a single flag (specifically, the cert default dir).
func DefaultServerFlags() *ServerFlags {
	return &ServerFlags{
		HttpPort: DefaultPort,
	}
}

func (f *ServerFlags) Register(set kflags.FlagSet, prefix string) *ServerFlags {
	set.IntVar(&f.HttpPort, prefix+"http-port", f.HttpPort, "Default port number to listen on for HTTP connections - only used if the address does not include a port.")
	set.StringVar(&f.HttpAddress, prefix+"http-address", f.HttpAddress, "Address to bind on to wait for HTTP connections. 0.0.0.0 is assumed if not specified.")

	set.IntVar(&f.HttpsPort, prefix+"https-port", f.HttpsPort, "Default port number to listen on for HTTP connections - only used if the address does not include a port.")
	set.StringVar(&f.HttpsAddress, prefix+"https-address", f.HttpsAddress, "Address to bind on to wait for HTTP connections. 0.0.0.0 is assumed if not specified.")

	return f
}

func addDefaultPort(address string, port int) (string, error) {
	var err error
	var shost, sport string

	if address != "" {
		shost, sport, err = SplitHostPort(address)
		if err != nil {
			return "", err
		}

		if sport != "" {
			return address, nil
		}
	}

	if port <= 0 || port > 65535 {
		return "", fmt.Errorf("invalid default port - %d", port)
	}
	return net.JoinHostPort(shost, strconv.Itoa(port)), nil
}

type Server struct {
	// Function to use to log output.
	log logger.Printer

	// Functions used to start the http (or https) server.
	run func(logger.Printer, *http.Server) error

	HTTP  *http.Server
	HTTPS *http.Server
	HTTP2 *http2.Server
}

type Modifier func(opts *Server) error

type Modifiers []Modifier

func (mods Modifiers) Apply(opts *Server) error {
	for _, m := range mods {
		if err := m(opts); err != nil {
			return err
		}
	}
	return nil
}

func FromServerFlags(flags *ServerFlags) Modifier {
	return func(opts *Server) error {
		httpsAddr, httpAddr := "", ""
		var err error
		if flags.HttpAddress != "" || flags.HttpPort > 0 {
			httpAddr, err = addDefaultPort(flags.HttpAddress, flags.HttpPort)
			if err != nil {
				return kflags.NewUsageErrorf("invalid or no http address - check --http-address or --http-port - %w", err)
			}
		}
		if flags.HttpsAddress != "" || flags.HttpsPort > 0 {
			httpsAddr, err = addDefaultPort(flags.HttpsAddress, flags.HttpsPort)
			if err != nil {
				return kflags.NewUsageErrorf("invalid or no https address - check --https-address or --https-port - %w", err)
			}
		}
		if httpAddr == "" && httpsAddr == "" {
			return kflags.NewUsageErrorf("neither an http or https address were specified - use --http(s)-address or --http(s)-port")
		}

		WithHTTPSAddr(httpsAddr)(opts)
		WithHTTPAddr(httpAddr)(opts)
		return nil
	}
}

type AutocertFlags struct {
	Cache   string
	Domains string
}

func DefaultAutocertFlags() *AutocertFlags {
	return &AutocertFlags{
		Cache: DefaultCache,
	}
}

func (f *AutocertFlags) Register(set kflags.FlagSet, prefix string) *AutocertFlags {
	set.StringVar(&f.Cache, prefix+"cert-cache", f.Cache, "Location where certificates are cached. "+
		"If empty, 'let's encrypt' SSL autocerts are disabled.")
	set.StringVar(&f.Domains, prefix+"cert-domains", f.Domains,
		"Comma separated list of domains that are mapped to the IP of this server "+
			"for which certificates should be automatically generated using 'let's encrypt'. "+
			"Generally, this flags ADDs domains on top of those that the code already knows about "+
			"from other flags or configuration files")
	return f
}

func FromAutocertFlags(flags *AutocertFlags, doms ...string) Modifier {
	return func(opts *Server) error {
		cacheDir := ""
		if flags.Cache != "" {
			if err := os.MkdirAll(flags.Cache, 0700); err != nil {
				return kflags.NewUsageErrorf("could not create dir selected with --cert-cache: %w", err)
			}
			cacheDir = flags.Cache
		} else {
			opts.log("Let's encrypt autocert -- disabled, empty --cert-cache")
			return nil
		}

		domains := append(doms, strings.Split(flags.Domains, ",")...)
		return WithAutocert(cacheDir, domains...)(opts)
	}
}

func WithAutocert(cachedir string, domains ...string) Modifier {
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache(cachedir),
	}

	return func(opts *Server) error {
		opts.log("Let's encrypt autocert -- enabled, storing certificates in '%s'", cachedir)
		opts.log("Let's encrypt autocert -- allowing domains %q", domains)

		// Install the certificate "fetcher" in the https server.
		if err := WithTLSOptions(ktls.WithGetCertificate(certManager.GetCertificate))(opts); err != nil {
			return err
		}

		// Install the challenger responder/redirector in the http server.
		opts.HTTP.Handler = certManager.HTTPHandler(opts.HTTP.Handler)
		return nil
	}
}

func WithTLSOptions(mods ...ktls.Modifier) Modifier {
	return func(opts *Server) error {
		config := opts.HTTPS.TLSConfig
		if config == nil {
			config = &tls.Config{}
		} else {
			config = config.Clone()
		}
		opts.HTTPS.TLSConfig = config

		return ktls.Modifiers(mods).Apply(config)
	}
}

func WithHTTPServerOptions(mods ...kserver.Modifier) Modifier {
	return func(opts *Server) error {
		return kserver.Modifiers(mods).Apply(opts.HTTP)
	}
}

func WithHTTPSServerOptions(mods ...kserver.Modifier) Modifier {
	return func(opts *Server) error {
		return kserver.Modifiers(mods).Apply(opts.HTTPS)
	}
}

func WithServerOptions(mods ...kserver.Modifier) Modifier {
	return func(opts *Server) error {
		if err := WithHTTPServerOptions(mods...)(opts); err != nil {
			return err
		}
		return WithHTTPSServerOptions(mods...)(opts)
	}
}

func WithHTTP2ServerOptions(mods ...kserver.Modifier2) Modifier {
	return func(opts *Server) error {
		if opts.HTTP2 == nil {
			opts.HTTP2 = &http2.Server{}
		}

		return kserver.Modifiers2(mods).Apply(opts.HTTP2)
	}
}

func WithHTTPAddr(addr string) Modifier {
	return func(opts *Server) error {
		opts.HTTP.Addr = addr
		return nil
	}
}

func WithHTTPSAddr(addr string) Modifier {
	return func(opts *Server) error {
		opts.HTTPS.Addr = addr
		return nil
	}
}

func WithLogger(log logger.Printer) Modifier {
	return func(opts *Server) error {
		opts.log = log
		return nil
	}
}

func WithWaiter(wg *sync.WaitGroup, httpa, httpsa **net.TCPAddr) Modifier {
	defer wg.Done()

	return func(opts *Server) error {
		if opts.run != nil {
			return fmt.Errorf("WithWaiter is overriding the final runner - incorrect API usage")
		}

		opts.run = func(log logger.Printer, s *http.Server) error {
			if s.Addr == "" {
				wg.Done()
				return nil
			}

			ln, err := net.Listen("tcp", s.Addr)
			if err != nil {
				wg.Done()
				return err
			}
			defer ln.Close()

			if s.TLSConfig != nil {
				if httpsa != nil {
					*httpsa = ln.Addr().(*net.TCPAddr)
				}
				wg.Done()

				log("Listening on HTTPs address %s - configured for %s", ln.Addr(), s.Addr)
				return s.ServeTLS(ln, "", "")
			}

			if httpa != nil {
				*httpa = ln.Addr().(*net.TCPAddr)
			}
			wg.Done()

			log("Listening on HTTP address %s - configured for %s", ln.Addr(), s.Addr)
			return s.Serve(ln)
		}

		return nil
	}
}

func WithH2C() Modifier {
	return func(opts *Server) error {
		runner := opts.run
		if runner == nil {
			runner = ListenAndServe
		}

		opts.run = func(log logger.Printer, s *http.Server) error {
			h2server := opts.HTTP2
			if opts.HTTP2 == nil {
				h2server = &http2.Server{}
			}

			if s.TLSConfig == nil {
				s.Handler = h2c.NewHandler(s.Handler, h2server)
			}

			return runner(log, s)
		}

		return nil
	}
}

func New(handler http.Handler, mods ...Modifier) (*Server, error) {
	opts := &Server{
		log: logger.Nil.Infof,
		HTTP: &http.Server{
			Addr:    fmt.Sprintf(":%d", DefaultPort),
			Handler: handler,
		},
		HTTPS: &http.Server{
			Handler:   handler,
			TLSConfig: &tls.Config{},
		},
	}

	if err := Modifiers(mods).Apply(opts); err != nil {
		return nil, err
	}

	if opts.HTTP2 != nil {
		if err := http2.ConfigureServer(opts.HTTPS, opts.HTTP2); err != nil {
			return nil, err
		}
	}

	if opts.run == nil {
		opts.run = ListenAndServe
	}

	return opts, nil
}

func (opts *Server) Run() error {
	defer opts.HTTPS.Close()
	defer opts.HTTP.Close()

	err := goroutine.WaitFirstError(
		func() error {
			return opts.run(opts.log, opts.HTTPS)
		},
		func() error {
			return opts.run(opts.log, opts.HTTP)
		},
	)

	return err
}

func (opts *Server) Shutdown(ctx context.Context) error {
	return goroutine.WaitAll(
		func() error {
			return opts.HTTP.Shutdown(ctx)
		},
		func() error {
			return opts.HTTPS.Shutdown(ctx)
		},
	)
}

func Run(handler http.Handler, mods ...Modifier) error {
	server, err := New(handler, mods...)
	if err != nil {
		return err
	}

	return server.Run()
}

// ListenAndServe invokes the correct ListenAndServe on the passed http.Server.
//
// It invokes ListenAndServeTLS if the server has a TLSConfig assigned while
// it invokes ListenAndServe if there is no TLSConfig.
//
// Rather than listen on the default http port, if no address is specified
// no server is started.
//
// This function is a commodity wrapper convenient when starting servers
// from flags or configuration files, where a lack of address means nothing
// to start, and where the server type is determined by its configuration.
func ListenAndServe(log logger.Printer, s *http.Server) error {
	if s.Addr == "" {
		return nil
	}

	if s.TLSConfig != nil {
		log("Listening on HTTPs address %s", s.Addr)
		return s.ListenAndServeTLS("", "")
	}

	log("Listening on HTTP address %s", s.Addr)
	return s.ListenAndServe()
}

