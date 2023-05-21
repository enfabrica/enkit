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
	projectID           = flag.String("gcp_project_id", "", "Project ID of GCP project with BigQuery resources")
	bigqueryDataset     = flag.String("bigquery_dataset", "", "Name of the BigQuery dataset in which values should be inserted")
	bigqueryTable       = flag.String("bigquery_table", "", "Name of the BigQuery table in which values should be inserted")
)

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	table, err := reporter.NewBigquery[reporter.ActionRecord](ctx, *projectID, *bigqueryDataset, *bigqueryTable)
	exitIf(err)

	srv, err := reporter.NewService(ctx, table, *batchSize, time.Duration(*batchTimeoutSeconds)*time.Second)
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

	glog.Exit(server.Run(ctx, mux, grpcs, nil))
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}
