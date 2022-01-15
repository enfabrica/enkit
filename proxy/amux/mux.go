// Many of the enproxy libraries require an http.Mux to work.
//
// Different muxex provide different properties. For example, some muxes cannot
// match on domain names, break support for web sockets, or can use more or
// less powerful patterns.
//
// Which mux to use is generally a choice of the application using the enproxy
// libraries.
//
// This package defines a single Mux interface used throughout the enproxy
// codebase with two goals:
//
// 1) Define the minimal requirements of the Mux, both in terms of function
//    signatures and semantics, through interfaces and documentation.
//
// 2) Allow applications to use an arbitrary mux by simply implementing the
//    defined interfaces.
//
// Note that although most muxes in the golang ecosystem provide similar
// concepts, they vary wildly in terms of APIs and implementation.
//
// Subpackages of amux provide adapters for common muxes that over time
// have been used or tested with the enproxy implementation.
package amux

import (
	"net/http"
)

type Mux interface {
	// Host allows to match on the domain name.
	//
	// The host string is expected to be a FQDN. Not a subdomain.
	//
	// The function returns a Mux that can be used to define handlers for
	// specific paths in this specific domain, or add more host matches.
	//
	// The "empty host string" is considered a wildcard matching any FQDN.
	// Any path defined via Handle()
	//
	// The enproxy code does not otherwise use any form of pattern matching.
	// No regular expressions or wildcards are required.
	//
	// However, if the enproxy configuration file contains wildcards or
	// regular expressions, they are passed unmodified to the supplied Mux
	// interface and transparently handled by the configured handlers.
	//
	// If a Mux with support for patterns in Host() is thus provided to
	// enproxy, the corresponding functionalities will work and be available
	// to users of enproxy.
	Host(host string) Mux

	// Handle allows to match on an http path.
	//
	// The string is expected to be a full path, starting with "/".
	//
	// When Handle is called on a newly initialized Mux, eg, one that was
	// not created by a call to Host(), the path is expected to be
	// configured on the empty host domain, "". See the Host()
	// documentation for details.
	//
	// If the path ends with "/", it is assumed to be a prefix match.
	// Any subdirectory of such path will need to be routed to the handler.
	//
	// When the underlying mux invokes the corresponding http.Handler,
	// the supplied ResponseWriter MUST be type castable to the
	// https://pkg.go.dev/net/http#Hijacker in order to support WebSockets,
	// required by enproxy.
	Handle(path string, handler http.Handler)
}
