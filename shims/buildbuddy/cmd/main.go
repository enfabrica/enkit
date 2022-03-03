package main

import (
	"fmt"
	"github.com/enfabrica/enkit/shims/buildbuddy"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	var redirectUrlTo string
	var byteStreamHost string
	var servePrefix string
	var port int

	c := cobra.Command{
		Use: "serve",
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := url.Parse(redirectUrlTo)
			if err != nil {
				return fmt.Errorf("err with parsing %s %v", redirectUrlTo, err)
			}
			parsedByteStreamHost, err := url.Parse(byteStreamHost)
			if err != nil {
				return fmt.Errorf("err with parsing bytestreamhost %s %v", byteStreamHost, err)
			}
			fmt.Printf("Redirecting to %s \n", u.String())
			fmt.Printf("BytestreamHost is to %s \n", parsedByteStreamHost.Host)
			proxy := httputil.NewSingleHostReverseProxy(u)
			h := buildbuddy.NewHandler(servePrefix, parsedByteStreamHost.Host, proxy)
			fmt.Println("listening and serving with cors")
			return http.ListenAndServe(fmt.Sprintf(":%d", port), h)
		},
	}
	c.Flags().StringVar(&servePrefix, "prefix", "/file/download", "the prefix of the http handler")
	c.Flags().StringVar(&redirectUrlTo, "redirect-url", "buildbarn-buildbuddy-svc.buildbarn:8080", "the url of which to reverse proxy to")
	c.Flags().StringVar(&byteStreamHost, "bytestream-host", "buildbarn-browser-svc.buildbarn", "the url of which to reverse proxy to")
	c.Flags().IntVar(&port, "port", 8080, "port to serve on")

	log.Fatal(c.Execute())
}
