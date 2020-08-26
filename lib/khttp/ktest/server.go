package ktest

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

type Handler func(w http.ResponseWriter, r *http.Request)

type Recorder struct {
	Handler  Handler
	Request  []*http.Request
	Response []*http.Response
}

func Capture(handler Handler) *Recorder {
	return &Recorder{Handler: handler}
}

func (capture *Recorder) Handle(w http.ResponseWriter, r *http.Request) {
	capture.Request = append(capture.Request, r)
	response := httptest.NewRecorder()
	response.Body = bytes.NewBuffer(nil)

	capture.Handler(response, r)
	result := response.Result()
	capture.Response = append(capture.Response, result)

	for key, values := range result.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(result.StatusCode)
	w.Write(response.Body.Bytes())
}

func TimeoutHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(60 * time.Second)
	fmt.Fprintf(w, "hello")
}

var CacheTime = time.Unix(10, 0)

// Slow will slow down the responses by the specified amount.
// Convenient to try to trigger timeouts, or race conditions.
func Slow(d time.Duration, h Handler) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(d)
		h(w, r)
	}
}

// StringHandler just reeturns a string, WITHOUT any header that allows caching.
func StringHandler(message string) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s", message)
	}
}

// CachableStringHandler reeturns a string, adding headers that allow client caching.
func CachableStringHandler(message string) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "hello.html", CacheTime, strings.NewReader(message))
	}
}

func CachableHelloHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeContent(w, r, "hello.html", CacheTime, strings.NewReader("hello"))
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello")
}

func ErrorHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "all kitties have died", http.StatusInternalServerError)
}
func HangingHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(24 * 365 * time.Hour)
}

func StartServer(h Handler) (*http.ServeMux, string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return mux, "", err
	}
	server := &http.Server{Handler: mux}
	go func() { server.Serve(ln) }()
	port := ln.Addr().(*net.TCPAddr).Port
	return mux, fmt.Sprintf("http://127.0.0.1:%d/", port), nil
}
