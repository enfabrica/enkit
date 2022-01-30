// Interface adapter to make a "muxie" (from https://github.com/kataras/muxie) comformant
// to the "github.com/enfabrica/enkit/proxy/amux" interface.
package amuxie

import (
	"github.com/enfabrica/enkit/proxy/amux"
	"github.com/kataras/muxie"
	"net/http"
	"strings"
)

type Mux struct {
	*muxie.Mux
}

func New() *Mux {
	return &Mux{Mux: muxie.NewMux()}
}

func (m *Mux) Host(host string) amux.Mux {
	h := muxie.NewMux()
	m.HandleRequest(muxie.Host(host), h)
	if !strings.HasSuffix(host, ".") {
		m.HandleRequest(muxie.Host(host + "."), h)
	}

	return &Mux{h}
}

func (m *Mux) Handle(path string, handler http.Handler) {
	m.Mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// To manage the "upgrade" header to turn HTTP requests into
		// WebSockets, the ResponseWriter must implement the Hijacker interface
		// (https://pkg.go.dev/net/http#Hijacker).
		//
		// This is normally implemented by type casting the ResponseWriter
		// into a Hijacker. If the cast succeeds, the WebSocket upgrade is
		// performed. If the cast fails, the upgrade fails.
		//
		// The muxie library passes a muxie.Writer as a ResponseWriter,
		// (https://pkg.go.dev/github.com/kataras/muxie#Writer), which is
		// declared as a struct embedding a ResponseWriter interface.
		//
		// This causes the type casting to Hijacker to ALWAYS fail, no matter
		// what the underlying implementation of the ResponseWriter is: the
		// muxie.Writer struct itself does not implement any other interface,
		// even if the embedded ResponseWriter does.
		//
		// The code here replaces the muxie.Writer with the original
		// embedded ResponseWriter, so typecasting can succeed.
		//
		// To better understand the problem, you can look at the demo here
		// showcasing the casting constraints: https://go.dev/play/p/WQJDMVhjKZY
		//
		// A loop is used for defense in depth, as different implementations
		// of middlewares could cause more than one layer of encapsulation.
		for {
			mw, ok := w.(*muxie.Writer)
			if !ok {
				break
			}

			w = mw.ResponseWriter
		}
		handler.ServeHTTP(w, r)
		return
	}))
}
