package main

import (
	"flag"
	"net/http"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/grpc"

	"github.com/enfabrica/enkit/bes_publisher/buildevent"
	"github.com/enfabrica/enkit/lib/server"
)

var (
	// BES can send large messages; if the default message size isn't raised,
	// these large messages will be dropped.
	maxMessageSize = flag.Int(
		"grpc_max_message_size_bytes",
		50*1024*1024,
		"Maximum receive message size in bytes accepted by gRPC methods",
	)
)

func main() {
	flag.Parse()

	grpcs := grpc.NewServer(
		grpc.MaxRecvMsgSize(*maxMessageSize),
	)
	bpb.RegisterPublishBuildEventServer(grpcs, &buildevent.Service{})

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	glog.Exit(server.Run(mux, grpcs, nil))
}
