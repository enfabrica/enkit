package main

import (
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/proxy/credentials"
	"github.com/enfabrica/enkit/proxy/httpp"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"github.com/enfabrica/enkit/proxy/utils"
	"github.com/spf13/cobra"
	"log"
	"math/rand"
	"net/http"
	"os"
)

type Config struct {
	// Which URLs to map to which other URLs.
	Mapping []httpp.Mapping
	// Extra domains for which to obtain a certificate.
	Domains []string
	// List of allowed tunnels.
	Tunnels []string
}

func main() {
	root := &cobra.Command{
		Use:           "enproxy",
		Long:          `proxy - starts an authenticating proxy`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Example: `  $ proxy -c ./mappings.toml
	To start a proxy mapping the urls defined in mappings.toml.`,
	}

	set, populator, runner := kcobra.Runner(root, os.Args)

	base := client.DefaultBaseFlags(root.Name(), "enkit")

	hflags := khttp.DefaultFlags()
	hflags.Register(set, "")

	rflags := oauth.DefaultRedirectorFlags()
	rflags.Register(set, "")

	nflags := nasshp.DefaultFlags()
	nflags.Register(set, "")

	configbytes := []byte{}
	configname := ""
	withoutAuthentication := false
	set.BoolVar(&withoutAuthentication, "without-authentication", false,
		"allow tunneling even without authentication")
	set.ByteFileVar(&configbytes, "config", configname, "Default config file location.", kflags.WithFilename(&configname))

	root.RunE = func(cmd *cobra.Command, args []string) error {
		var config Config
		if err := marshal.UnmarshalDefault(configname, configbytes, marshal.Json, &config); err != nil {
			return kflags.NewUsageError(err)
		}
		if len(config.Mapping) <= 0 {
			return kflags.NewUsageErrorf("invalid config: it has no mappings")
		}
		if len(config.Tunnels) <= 0 {
			base.Log.Warnf("watch out, your config has no whitelisted tunnel - no tunnel will be allowed!")
		}
		wl, err := utils.NewPatternList(config.Tunnels)
		if err != nil {
			return kflags.NewUsageErrorf("invalid patterns specified in tunnels: %s", err)
		}

		mods := []httpp.Modifier{httpp.WithLogging(base.Log)}
		var authenticate oauth.Authenticate
		if rflags.AuthURL != "" && !withoutAuthentication{
			redirector, err := oauth.NewRedirector(oauth.WithRedirectorFlags(rflags))
			if err != nil {
				return err
			}
			authenticate = redirector.Authenticate
			mods = append(mods, httpp.WithStripCookie([]string{
				redirector.CredentialsCookieName(),
			}))
			mods = append(mods, httpp.WithAuthenticator(authenticate))
		}

		hproxy, err := httpp.New(config.Mapping, mods...)
		if err != nil {
			return err
		}

		dispatcher := http.Handler(hproxy)
		if authenticate == nil && !withoutAuthentication {
			base.Log.Warnf("ssh gateway disabled as no authentication was configured")
		} else {
			if withoutAuthentication {
				base.Log.Errorf("Watch out! The proxy is being started without authentication! Relying entirely on a filmsy whitelist")
				authenticate = nil
			}

			rng := rand.New(srand.Source)
			nasshp, err := nasshp.New(rng, authenticate, nasshp.WithFilter(wl.Allow), nasshp.FromFlags(nflags), nasshp.WithLogging(base.Log))
			if err != nil {
				return err
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
				{Host: nflags.RelayHost, Handler: mux},
			}
			if nflags.RelayHost != "" {
				hosts = append(hosts, khttp.HostDispatch{Handler: hproxy})
			}

			handler, err := khttp.NewHostDispatcher(hosts)
			if err != nil {
				return err
			}
			dispatcher = handler
		}

		server, err := khttp.FromFlags(hflags)
		if err != nil {
			return err
		}

		return server.Run(base.Log.Infof, &khttp.Dumper{Real: dispatcher, Log: log.Printf}, append(config.Domains, hproxy.Domains...)...)
	}

	base.LoadFlagAssets(populator, credentials.Data)
	base.Run(set, populator, runner)
}
