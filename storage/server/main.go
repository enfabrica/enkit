package main

import (
	"flag"
	"net"
	"os"

	rpb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"golang.org/x/exp/slog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/enfabrica/enkit/storage/server/cas"
)

var (
	addr = flag.String("grpc_addr", ":9090", "The address to listen to")
)

func main() {
	flag.Parse()

	lh := slog.NewTextHandler(os.Stderr)
	l := slog.New(lh)
	slog.SetDefault(l)
	rpc := cas.New()
	srv := grpc.NewServer()
	rpb.RegisterContentAddressableStorageServer(srv, rpc)
	reflection.Register(srv)
	lis, err := net.Listen("tcp", *addr)
	failIfErr(err)
	slog.Info("Starting server", "grpc_addr", *addr)
	failIfErr(srv.Serve(lis))
}

func failIfErr(err error) {
	if err != nil {
		slog.Error("FATAL", "err", err)
		os.Exit(1)
	}
}

