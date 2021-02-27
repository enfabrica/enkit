package main

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/server"
	"github.com/enfabrica/kbuild/assets"
	"log"
	"net/http"
)

func Start(oauthFlags *oauth.RedirectorFlags) error {

	mux := http.NewServeMux()

	stats := server.AssetStats{}
	server.RegisterAssets(&stats, assets.Data, "", server.BasicMapper(server.MuxMapper(mux)))
	stats.Log(log.Printf)

	// The root of the web server, nothing to see here.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from your friendly machinist")
	})

	return nil
}
