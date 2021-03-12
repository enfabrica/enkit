package main

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/server"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/server/assets"
	"github.com/enfabrica/enkit/machinist/server/flags"
	"github.com/enfabrica/enkit/machinist/server/controller"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"log"
	"net/http"
	"os"
)

func Start(oauthFlags *oauth.RedirectorFlags) error {
	controller, err := controller.New()
	if err != nil {
		return err
	}

	grpcs := grpc.NewServer()
	machinist.RegisterControllerServer(grpcs, controller)

	mux := http.NewServeMux()

	stats := server.AssetStats{}
	server.RegisterAssets(&stats, assets.Data, "", server.BasicMapper(server.MuxMapper(mux)))
	stats.Log(log.Printf)

	// The root of the web server, nothing to see here.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from your friendly machinist")
	})

	server.Run(mux, grpcs)
	return nil
}

func main() {
	command := &cobra.Command{
		Use:   "controller",
		Short: "controller is a server in charge of controlling workers",
	}

	oauthFlags := oauth.DefaultRedirectorFlags()
	oauthFlags.Register(&kcobra.FlagSet{command.Flags()}, "")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return Start(oauthFlags)
	}

	kcobra.PopulateDefaults(command, os.Args,
		kflags.NewAssetAugmenter(&logger.NilLogger{}, "controller", flags.Data),
	)
	kcobra.Run(command)
}
