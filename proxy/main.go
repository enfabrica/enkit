package main

import (
	"github.com/spf13/cobra"

	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/proxy/httpp"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"net/http"
	"log"
)

type Dumper struct {
	Real http.Handler
	Log logger.Printer
}

func (d *Dumper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.Log("REQUEST %s", r.Method)
	d.Log(" - host %s", r.Host)
	d.Log(" - url %s", r.URL)
	d.Log(" - headers")
	for key, value := range r.Header {
	  d.Log("   - %s: %s", key, value)
	}
	d.Real.ServeHTTP(w, r)
}

func main() {
	root := &cobra.Command{
		Use:           "proxy",
		Long:          `proxy - starts an authenticating proxy`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Example: `  $ proxy -c ./mappings.toml
	To start a proxy mapping the urls defined in mappings.toml.`,
	}

	pflags := httpp.DefaultFlags()
	pflags.Register(&kcobra.FlagSet{FlagSet: root.Flags()}, "")

	hflags := khttp.DefaultFlags()
	hflags.Register(&kcobra.FlagSet{FlagSet: root.Flags()}, "")

	mylog := logger.Nil
	root.RunE = func(cmd *cobra.Command, args []string) error {
		hproxy, err := httpp.New(httpp.FromFlags(pflags), httpp.WithLogging(mylog))
		if err != nil {
			return err
		}

		nasshp, err := nasshp.New(mylog)
		if err != nil {
			return err
		}

		mux := http.NewServeMux()
		mux.Handle("/", hproxy)
		nasshp.Register(mux.HandleFunc)

		server, err := khttp.FromFlags(hflags)
		if err != nil {
			return err
		}

		return server.Run(mylog.Infof, &Dumper{Real: mux, Log: log.Printf}, hproxy.Domains...)
	}

	kcobra.RunWithDefaults(root, nil, &mylog)
}
