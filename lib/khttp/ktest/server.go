// +build !release

package ktest

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

// Returns a file, cachable.
func CachableFileHandler(file string) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(file)
		if err != nil {
			panic(fmt.Sprintf("could not open %s", file))
		}

		defer f.Close()
		http.ServeContent(w, r, "hello.html", CacheTime, f)
	}
}

// Returns a file not cachable.
func FileHandler(file string) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(file)
		if err != nil {
			panic(fmt.Sprintf("could not open %s", file))
		}

		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		io.Copy(w, f)
		return
	}
}

// Returns a file from the "testdata" directory", cachable.
func CachableTestDataHandler(file string) Handler {
	return CachableFileHandler(filepath.Join("testdata", file))
}

// Returns a file from the "testdata" directory", not cachable.
func TestDataHandler(file string) Handler {
	return FileHandler(filepath.Join("testdata", file))
}

// Always returns the string "hello", with headers that allow caching.
func CachableHelloHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeContent(w, r, "hello.html", CacheTime, strings.NewReader("hello"))
}

// Always returns the string "hello".
func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello")
}

// ErrorHandler returns a StatusInternalServerError.
func ErrorHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "all kitties have died", http.StatusInternalServerError)
}

// HangingHandler hangs forever.
func HangingHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(24 * 365 * time.Hour)
}

// StartURL starts an http server listening on a random port.
//
// Uses the supplied http.Handler to serve pages, returns the
// full URL it is listening on.
func StartURL(s http.Handler) (*url.URL, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := &http.Server{Handler: s}
	go func() { server.Serve(ln) }()
	port := ln.Addr().(*net.TCPAddr).Port
	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("127.0.0.1:%d", port),
		Path:   "/",
	}, nil
}

// Start is just like StartURL, but returns a string instead of an URL object.
func Start(s http.Handler) (string, error) {
	u, err := StartURL(s)
	if err != nil {
		return "", err
	}
	return u.String(), err
}

// StartServerURL is like StartURL, except it creates and returns a new mux.
func StartServerURL(h Handler) (*http.ServeMux, *url.URL, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h)
	res, err := StartURL(mux)
	return mux, res, err
}

// StartServer is just like Start, excepts it creates and returns a new mux.
func StartServer(h Handler) (*http.ServeMux, string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h)
	res, err := Start(mux)
	return mux, res, err
}
