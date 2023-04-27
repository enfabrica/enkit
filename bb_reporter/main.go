package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	cpb "github.com/buildbarn/bb-remote-execution/pkg/proto/completedactionlogger"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"github.com/enfabrica/enkit/bb_reporter/reporter"
	"github.com/enfabrica/enkit/lib/server"
)

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv, err := reporter.NewService(ctx)
	exitIf(err)

	grpcs := grpc.NewServer()
	cpb.RegisterCompletedActionLoggerServer(grpcs, srv)

	go func() {
		<-ctx.Done()
		glog.Info("Got ctx.Done(); stopping gRPC server")
		grpcs.Stop()
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	glog.Exit(server.Run(mux, grpcs, nil))
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}
