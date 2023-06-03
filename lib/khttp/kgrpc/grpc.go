package kgrpc

import (
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net/http"
	"strings"
)

// GRPCHandler creates an handler able to dispatch http requests as well as grpc requests.
//
// The first argument, h, is an http.Handler in charge of handling traditional
// HTTP or HTTP2 requests, typically an http.ServeMux or your favourite mux.
//
// The second argument, grpcs, is a grpc server.
//
// The returned http.Handler will check the request 'content-type', and dispatch
// it accordingly to the correct mux. The returned handler supports both plain grpc,
// as well as the grpcweb protocol.
func GRPCHandler(h http.Handler, grpcs *grpc.Server) http.Handler {
	reflection.Register(grpcs)

	// grpcOrHttp handler will check if a request is a grpc or plain http request.
	//
	// If it is a grpc request, it will forward it to the grpc handler.
	// If it is not, it will forward it to the plain http handler.
	grpcOrHttp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md defines as valid:
		//   "content-type" "application/grpc" [("+proto" / "+json" / {custom})]
		//
		// We require the "+" here to avoid accepting application/grpc-web as valid.
		ctype := r.Header.Get("content-type")
		if r.ProtoMajor == 2 && (strings.HasPrefix(ctype, "application/grpc+") || ctype == "application/grpc") {
			grpcs.ServeHTTP(w, r)
			return
		}

		h.ServeHTTP(w, r)
	})

	// WrapHandler will check if a request is a grpcweb request.
	// If it is, it will "turn it" into a normal grpc request, and invoke our handler.
	// If it is not, it will invoke our handler directly (grpcOrHttp).
	//
	// It is considered grpc-web IF:
	// - method == POST && content-type == "application/grpc-web" (IsGRPCWebRequest)
	//   _OR_
	// - method == OPTIONS && Access-Control-Request-Headers contain "x-grpc-web" (IsAcceptableGRPCCorsRequest)
	//   _OR_
	// - there's an Upgrade == "websocket" header && Sec-Websocket-Protocol contains "grpc-websockets"
	grpcw := grpcweb.WrapHandler(
		grpcOrHttp, grpcweb.WithAllowNonRootResource(true), grpcweb.WithWebsockets(true),
		grpcweb.WithOriginFunc(func(string) bool { return true }),
		grpcweb.WithEndpointsFunc(func() []string {
			return grpcweb.ListGRPCResources(grpcs)
		}),
	)

	return grpcw
}

// NewServer creates a new khttp.Server capable of handling gRPC requests.
//
// It is a convenience wrapper around GRPCHandler and khttp.New, defaulting some of the
// settings that are necessary for gRPC to work properly.
func NewServer(handler http.Handler, grpcs *grpc.Server, mods ...khttp.Modifier) (*khttp.Server, error) {
	return khttp.New(GRPCHandler(handler, grpcs), append(mods, khttp.WithH2C())...)
}

// RunServer is just like khttp.Run, but for gRPC servers.
//
// It is a convenience wrapper around NewServer and equivalent to khttp.Run.
func RunServer(handler http.Handler, grpcs *grpc.Server, mods ...khttp.Modifier) error {
	server, err := NewServer(handler, grpcs, mods...)
	if err != nil {
		return err
	}

	return server.Run()
}
