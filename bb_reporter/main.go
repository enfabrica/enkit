package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cpb "github.com/buildbarn/bb-remote-execution/pkg/proto/completedactionlogger"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"github.com/enfabrica/enkit/bb_reporter/reporter"
	"github.com/enfabrica/enkit/lib/server"
)

var (
	batchSize           = flag.Int("batch_size", 100, "Number of messages that should be batched to each insert to BigQuery")
	batchTimeoutSeconds = flag.Int("batch_timeout_seconds", 2, "Max number of seconds between each insert flush")
)

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv, err := reporter.NewService(ctx, *batchSize, time.Duration(*batchTimeoutSeconds)*time.Second)
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
