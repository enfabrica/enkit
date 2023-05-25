package buildbuddy

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/rs/cors"
)

const ByteStreamUrlQueryParam = "bytestream_url"

func DefaultHandleFunc(proxy http.Handler, hostMappings map[string]string) func(writer http.ResponseWriter, r *http.Request) {
	return func(writer http.ResponseWriter, r *http.Request) {
		rawByteStreamUrl := r.URL.Query().Get(ByteStreamUrlQueryParam)
		if rawByteStreamUrl == "" {
			http.Error(writer, "bytestream_url not found in url", http.StatusNotFound)
			return
		}
		fmt.Println("raw bytestream url is ", rawByteStreamUrl)
		byteStreamUrl, err := url.Parse(rawByteStreamUrl)
		if err != nil {
			http.Error(writer, fmt.Errorf("error parsing bytestream url %+v", err).Error(), http.StatusInternalServerError)
			return
		}

		newHost, ok := hostMappings[byteStreamUrl.Host]
		if !ok {
			http.Error(writer, fmt.Errorf("no mapping for host: %q", byteStreamUrl.Host).Error(), http.StatusNotFound)
			return
		}
		byteStreamUrl.Host = newHost

		fmt.Println("new bytesstream url is ", byteStreamUrl.String())
		queryValues := r.URL.Query()
		queryValues.Set(ByteStreamUrlQueryParam, byteStreamUrl.String())
		r.URL.RawQuery = queryValues.Encode()
		proxy.ServeHTTP(writer, r)
		return
	}
}

func NewHandler(prefix string, hostMappings map[string]string, proxy http.Handler) http.Handler {
	h := http.NewServeMux()
	h.HandleFunc(prefix, DefaultHandleFunc(proxy, hostMappings))
	return cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		Debug:            true,
	}).Handler(h)
}
