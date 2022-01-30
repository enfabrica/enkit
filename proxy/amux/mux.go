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
	// specific paths in this specific domain, or add more FQDN matches.
	//
	// If the specified FQDN does not terminate with a ".", the Mux is
	// expected to match the FQDN both with a terminating "." and without. 
	//
	// The "empty host string" is considered a wildcard matching any FQDN,
	// meaning that the route will be used in any case there is no more
	// specific host match. If a default route for all hosts is needed,
	// the route must be added on all hosts.
	//
	// The enproxy code does not otherwise expect any form of pattern
	// matching in host names. No regular expressions or wildcards are
	// required.
	//
	// However, if the enproxy configuration file contains wildcards or
	// regular expressions, those are passed unmodified to the supplied Mux
	// interface and transparently handled by the configured handlers.
	//
	// If a Mux with support for patterns in Host() is thus provided to
	// enproxy, the corresponding functionalities will work and be
	// available to users of enproxy.
	Host(host string) Mux

	// Handle allows to match on an http path.
	//
	// The string is expected to be a full path, starting with "/".
	// The empty string is not allowed, and will result in undefined behavior.
	//
	// When Handle is called on a newly initialized Mux, eg, one that was
	// not created by a call to Host(), the path is treated as being configured
	// on the empty host domain, "". See the Host() documentation for details.
	//
	// The match is always exact, unless it ends with a "*". A trailing "*"
	// indicates that any suffix of that path is accepted.
	//
	// enproxy only requires trailing "*" support after a /, indicating all
	// sub-paths of a specific directory.
	//
	// Just like the Host() directive, however, patterns are forwarded
	// without further validation or transformation to the Mux, so any
	// additional functionality of the Mux can be directly exposed in the
	// config.
	//
	// When the underlying mux invokes the corresponding http.Handler,
	// the supplied ResponseWriter MUST be type castable to the
	// https://pkg.go.dev/net/http#Hijacker in order to support WebSockets,
	// required by enproxy.
	Handle(path string, handler http.Handler)
}
