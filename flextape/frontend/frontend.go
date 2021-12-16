// Package frontend provides some server-side generated HTML for viewing
// Flextape state. This package is temporary; functionality should be moved to a
// client-side React UI that calls the backend service directly.
package frontend

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"

	fpb "github.com/enfabrica/enkit/flextape/proto"
	"github.com/enfabrica/enkit/flextape/service"
)

// Frontend is an HTTP handler for Flextape server-side generated pages.
type Frontend struct {
	tmpl *template.Template
	svc  *service.Service
}

// New returns a Frontend that serves templates based on state from the supplied
// service.
func New(tmpl *template.Template, svc *service.Service) *Frontend {
	return &Frontend{
		tmpl: tmpl,
		svc:  svc,
	}
}

// ServeHTTP serves the template for the queue page.
func (f *Frontend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.TODO()
	res, err := f.svc.LicensesStatus(ctx, &fpb.LicensesStatusRequest{})
	if checkErr(w, err) {
		return
	}

	buf := new(bytes.Buffer)
	err = f.tmpl.Execute(buf, res)
	if checkErr(w, err) {
		return
	}
	_, err = io.Copy(w, buf)
	if err != nil {
		// TODO: do something
		return
	}
}

// checkErr writes an error code and the supplied error to the ResponseWriter if
// said error is non-nil. Returns true if an error was written.
func checkErr(w http.ResponseWriter, err error) bool {
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "%v\n", err)
		return true
	}
	return false
}
