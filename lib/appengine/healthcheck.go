// Package appengine contains helpers for adapting servers to run under
// AppEngine.
package appengine

import (
	"fmt"
	"log"
	"net/http"
)

type CheckHandler func() error

// RegisterHealthchecks registers necessary AppEngine handlers on the specified
// mux. The handler paths are nonconfigurable and are as follows:
// * /_ah/start - Start handler (see https://cloud.google.com/appengine/docs/legacy/standard/python/how-instances-are-managed#startup)
// * /_ah/live - Liveness handler
// * /_ah/ready - Readiness handler
//
// Start handler always succeeds; liveness and readiness handlers used the
// supplied callbacks to check for errors, and fail with a non-200 code if their
// respective callback returns a non-nil error.
func RegisterHealthchecks(mux *http.ServeMux, liveness CheckHandler, readiness CheckHandler) {
	mux.HandleFunc("/_ah/start", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/_ah/live", func(w http.ResponseWriter, r *http.Request) {
		if err := liveness(); err != nil {
			log.Printf("AppEngine liveness check failed: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "liveness check reports: %v", err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/_ah/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := readiness(); err != nil {
			log.Printf("AppEngine readiness check failed: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "readiness check reports: %v", err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
}
