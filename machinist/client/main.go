package main

import (
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/server"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"fmt"
	"github.com/enfabrica/enkit/machinist/client/machinist"
	"github.com/enfabrica/enkit/machinist/client/assets"
	"github.com/enfabrica/enkit/machinist/client/flags"
	"log"
	"net/http"
	"os"
)

func Start(mflags *machinist.Flags) error {
	mux := http.NewServeMux()

	stats := server.AssetStats{}
	server.RegisterAssets(&stats, assets.Data, "", server.BasicMapper(server.MuxMapper(mux)))
	stats.Log(log.Printf)

	grpcs := grpc.NewServer()

	// The root of the web server, nothing to see here.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from your friendly machinist")
	})

	m, err := machinist.New(machinist.FromFlags(mflags), machinist.WithLogger(&logger.DefaultLogger{Printer: log.Printf}))
	if err != nil {
		return err
	}
	go m.Run()

	server.Run(mux, grpcs)
	return nil
}

func main() {
	command := &cobra.Command{
		Use:   "machinist",
		Short: "machinist controls the allocation of a machine through an controller",
	}

	mflags := machinist.DefaultFlags()
	mflags.Register(&kcobra.FlagSet{FlagSet: command.Flags()}, "")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return Start(mflags)
	}

	kcobra.PopulateDefaults(command, os.Args,
		kflags.NewAssetAugmenter(&logger.NilLogger{}, "machinist", flags.Data),
	)

	kcobra.Run(command)
}
