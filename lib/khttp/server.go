package khttp

import (
	"crypto/tls"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/kirsle/configdir"
	"golang.org/x/crypto/acme/autocert"
	"net/http"
	"os"
)

type FuncHandler func(w http.ResponseWriter, r *http.Request)

type Flags struct {
	HttpPort    int
	HttpAddress string

	HttpsPort    int
	HttpsAddress string

	Cache string
}

func DefaultFlags() *Flags {
	return &Flags{
		HttpPort: 9999,
		Cache:    configdir.LocalCache("enkit-certs"),
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.IntVar(&f.HttpPort, "http-port", f.HttpPort, "Port number on which the proxy will be listening for HTTP connections.")
	set.StringVar(&f.HttpAddress, "http-address", f.HttpAddress, "Port number on which the proxy will be listening for HTTP connections.")

	set.IntVar(&f.HttpsPort, "https-port", f.HttpsPort, "Port number on which the proxy will be listening for HTTPs connections.")
	set.StringVar(&f.HttpsAddress, "https-address", f.HttpsAddress, "Port number on which the proxy will be listening for HTTP connections.")

	set.StringVar(&f.Cache, "cert-cache", f.Cache, "Location where certificates are cached.")
	return f
}

type Server struct {
	HttpAddress  string
	HttpsAddress string
	CacheDir     string
}

func FromFlags(flags *Flags) (*Server, error) {
	server := &Server{}

	server.HttpAddress = flags.HttpAddress
	if flags.HttpAddress == "" {
		if flags.HttpPort <= 0 {
			return nil, kflags.NewUsageErrorf("no http port specified - use --http-address or --http-port")
		}
		server.HttpAddress = fmt.Sprintf(":%d", flags.HttpPort)
	}

	server.HttpsAddress = flags.HttpsAddress
	if flags.HttpsAddress == "" && flags.HttpsPort > 0 {
		server.HttpsAddress = fmt.Sprintf(":%d", flags.HttpsPort)
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
