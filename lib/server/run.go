package server

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/soheilhy/cmux"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

//
// Run() starts a server supporting the following protocols:
//
// ✔ grpc
// ✔ grpc-web via websockets
// ✔ HTTP 1.1
// ✗ HTTP 2.0 (hijacked by grpc support)
func Run(mux http.Handler, grpcs *grpc.Server) {
	if mux == nil {
		mux = http.NewServeMux()
	}
	if grpcs == nil {
		grpcs = grpc.NewServer()
	}
	reflection.Register(grpcs)

	port := os.Getenv("PORT")
	if port == "" {
		port = "6433"
	}

	log.Printf("Opening port %s - will be available at http://127.0.0.1:%s/", port, port)
	listener, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		log.Fatalf("failed to listen: %s", err)
	}

	// Create all listeners.
	cml := cmux.New(listener)
	grpcl := cml.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpl := cml.Match(cmux.Any())

	grpcw := grpcweb.WrapServer(grpcs, grpcweb.WithAllowNonRootResource(true), grpcweb.WithWebsockets(true), grpcweb.WithOriginFunc(func(string) bool { return true }))

	https := &http.Server{Handler: http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if grpcw.IsGrpcWebRequest(req) {
			grpcw.ServeHTTP(resp, req)
		} else {
			mux.ServeHTTP(resp, req)
		}
	})}
	go grpcs.Serve(grpcl)
	go https.Serve(httpl)

	if err := cml.Serve(); err != nil {
		log.Fatalf("Serve failed with error: %s", err)
	}
}

// CloudRun starts an HTTP and gRPC server handling requests for all on the same
// port, determined by the env var $PORT. If no HTTP mux or gRPC server is
// provided (is nil), one with default routes/services will be started,
// respectively.
//
// CloudRun() is a helper for servers that run in Google Cloud run, and thus
// starts a server that supports the following protocols:
//
// ✔ grpc
// ✗ grpc-web via websockets
// ✗ HTTP 1.1
// ✔ HTTP 2.0
//
// TODO(INFRA-211): Merge with above Run() function if possible, and add tests
// that verify the protocol compatibility table above either way.
func CloudRun(mux http.Handler, grpcs *grpc.Server) {
	if mux == nil {
		mux = http.NewServeMux()
	}
	if grpcs == nil {
		grpcs = grpc.NewServer()
	}
	reflection.Register(grpcs)

	port := os.Getenv("PORT")
	if port == "" {
		port = "6433"
	}

	log.Printf("Opening port %s - will be available at http://127.0.0.1:%s/", port, port)
	listener, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		log.Fatalf("failed to listen: %s", err)
	}

	mux = grpcHTTP2Mux(mux, grpcs)

	http2s := &http2.Server{}
	https := &http.Server{
		Handler: h2c.NewHandler(mux, http2s),
	}

	if err := https.Serve(listener); err != nil {
		log.Fatalf("Serve failed with error: %s", err)
	}
}

func grpcHTTP2Mux(h http.Handler, grpc http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && r.Header.Get("content-type") == "application/grpc" {
			grpc.ServeHTTP(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}
