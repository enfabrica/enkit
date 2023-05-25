package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enfabrica/enkit/shims/buildbuddy"
)

func mapFromStringMapping(ms []string, delimiter string) (map[string]string, error) {
	ret := map[string]string{}
	for _, m := range ms {
		parts := strings.Split(m, delimiter)
		if len(parts) != 2 {
			return ret, fmt.Errorf("got %d parts of string %q with delimiter %q; want exactly 2", len(parts), m, delimiter)
		}
		ret[parts[0]] = parts[1]
	}
	return ret, nil
}

func main() {
	var redirectUrlTo string
	var byteStreamMappings []string
	var servePrefix string
	var port int

	c := cobra.Command{
		Use: "serve",
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := url.Parse(redirectUrlTo)
			if err != nil {
				return fmt.Errorf("err with parsing %s %v", redirectUrlTo, err)
			}
			mappings, err := mapFromStringMapping(byteStreamMappings, " -> ")
			if err != nil {
				return err
			}
			proxy := httputil.NewSingleHostReverseProxy(u)
			h := buildbuddy.NewHandler(servePrefix, mappings, proxy)
			fmt.Println("listening and serving with cors")
			return http.ListenAndServe(fmt.Sprintf(":%d", port), h)
		},
	}
	c.Flags().StringVar(&servePrefix, "prefix", "/file/download", "the prefix of the http handler")
	c.Flags().StringVar(&redirectUrlTo, "redirect-url", "buildbarn-buildbuddy-svc.buildbarn:8080", "the url of which to reverse proxy to")
	c.Flags().StringArrayVar(&byteStreamMappings, "bytestream-mapping", []string{}, "Map of host rewrites to perform on the bytestream URL")
	c.Flags().IntVar(&port, "port", 8080, "port to serve on")

	log.Fatal(c.Execute())
}
