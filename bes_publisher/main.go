package main

import (
	"context"
	"flag"
	"net/http"

	"cloud.google.com/go/pubsub"
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
	gcpProjectID = flag.String(
		"gcp_project_id",
		"",
		"GCP project with PubSub resources to use",
	)
	besPubsubTopic = flag.String(
		"bes_pubsub_topic",
		"",
		"Name of topic to publish BES messages on",
	)
)

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}

func main() {
	flag.Parse()
	ctx := context.Background()

	pubsubClient, err := pubsub.NewClient(ctx, *gcpProjectID)
	exitIf(err)
	topic := buildevent.NewTopic(pubsubClient.Topic(*besPubsubTopic))

	srv, err := buildevent.NewService(topic)
	exitIf(err)

	grpcs := grpc.NewServer(
		grpc.MaxRecvMsgSize(*maxMessageSize),
	)
	bpb.RegisterPublishBuildEventServer(grpcs, srv)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	glog.Exit(server.Run(mux, grpcs, nil))
}
