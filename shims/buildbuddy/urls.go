package buildbuddy

import (
	"fmt"
	"github.com/rs/cors"
	"net/http"
	"net/url"
)

const ByteStreamUrlQueryParam = "bytestream_url"

func DefaultHandleFunc(proxy http.Handler, host string) func(writer http.ResponseWriter, r *http.Request) {
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
		}
		byteStreamUrl.Host = host

		fmt.Println("new bytesstream url is ", byteStreamUrl.String())
		queryValues := r.URL.Query()
		queryValues.Set(ByteStreamUrlQueryParam, byteStreamUrl.String())
		r.URL.RawQuery = queryValues.Encode()
		fmt.Println("reverse serving to ", r.URL.String())
		proxy.ServeHTTP(writer, r)
		return
	}
}

func NewHandler(prefix string, host string, proxy http.Handler) http.Handler {
	h := http.NewServeMux()
	h.HandleFunc(prefix, DefaultHandleFunc(proxy, host))
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
