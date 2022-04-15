package khttp

import (
	"crypto/tls"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/kirsle/configdir"
	"golang.org/x/crypto/acme/autocert"
	"net"
	"net/http"
	"os"
	"strconv"
)

type FuncHandler func(w http.ResponseWriter, r *http.Request)

type Flags struct {
	HttpPort    int
	HttpAddress string

	HttpsPort    int
	HttpsAddress string

	Cache string
}

const DefaultPort = 9999

var DefaultCache = configdir.LocalCache("enkit-certs")

func DefaultFlags() *Flags {
	return &Flags{
		HttpPort: DefaultPort,
		Cache:    DefaultCache,
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.IntVar(&f.HttpPort, prefix+"http-port", f.HttpPort, "Default port number to listen on for HTTP connections - only used if the address does not include a port.")
	set.StringVar(&f.HttpAddress, prefix+"http-address", f.HttpAddress, "Address to bind on to wait for HTTP connections. 0.0.0.0 is assumed if not specified.")

	set.IntVar(&f.HttpsPort, prefix+"https-port", f.HttpsPort, "Default port number to listen on for HTTP connections - only used if the address does not include a port.")
	set.StringVar(&f.HttpsAddress, prefix+"https-address", f.HttpsAddress, "Address to bind on to wait for HTTP connections. 0.0.0.0 is assumed if not specified.")

	set.StringVar(&f.Cache, prefix+"cert-cache", f.Cache, "Location where certificates are cached.")
	return f
}

type Server struct {
	HttpAddress  string
	HttpsAddress string
	CacheDir     string
}

func DefaultServer() *Server {
	return &Server{
		HttpAddress: fmt.Sprintf(":%d", DefaultPort),
		CacheDir:    DefaultCache,
	}
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

func FromFlags(flags *Flags) (*Server, error) {
	server := &Server{}

	var err error
	if flags.HttpAddress != "" || flags.HttpPort > 0 {
		server.HttpAddress, err = addDefaultPort(flags.HttpAddress, flags.HttpPort)
		if err != nil {
			return nil, kflags.NewUsageErrorf("invalid or no http address - check --http-address or --http-port - %w", err)
		}
	}
	if flags.HttpsAddress != "" || flags.HttpsPort > 0 {
		server.HttpsAddress, err = addDefaultPort(flags.HttpsAddress, flags.HttpsPort)
		if err != nil {
			return nil, kflags.NewUsageErrorf("invalid or no https address - check --https-address or --https-port - %w", err)
		}
	}
	if server.HttpAddress == "" && server.HttpsAddress == "" {
		return nil, kflags.NewUsageErrorf("neither an http or https address were specified - use --http(s)-address or --http(s)-port")
	}

	if flags.Cache != "" {
		if err := os.MkdirAll(flags.Cache, 0700); err != nil {
			return nil, err
		}
		server.CacheDir = flags.Cache
	}

	return server, nil
}

func (p *Server) Run(log logger.Printer, handler http.Handler, domains ...string) error {
	if log == nil {
		log = logger.Nil.Infof
	}

	if p.HttpsAddress == "" {
		log("Listening on HTTP address %s", p.HttpAddress)
		return http.ListenAndServe(p.HttpAddress, handler)
	}

	log("Storing certificates in '%s'", p.CacheDir)
	// create the autocert.Manager with domains and path to the cache
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache(p.CacheDir),
	}

	// create the server itself
	server := &http.Server{
		Addr: p.HttpsAddress,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
		Handler: handler,
	}

	log("Serving http/https for domains: %+v", domains)
	go func() {
		log("Listening on HTTP address %s", p.HttpAddress)
		http.ListenAndServe(p.HttpAddress, certManager.HTTPHandler(nil))
	}()

	log("Listening on HTTPs address %s", p.HttpsAddress)
	return server.ListenAndServeTLS("", "")
}
