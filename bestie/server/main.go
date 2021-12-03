package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/enfabrica/enkit/lib/server"
	_ "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/encoding/prototext"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
)

type BuildEventService struct {
}

func (s *BuildEventService) PublishLifecycleEvent(ctx context.Context, req *bpb.PublishLifecycleEventRequest) (*emptypb.Empty, error) {
	fmt.Printf("# BEP LifecycleEvent message:\n%s\n\n", prototext.Format(req))
	return &emptypb.Empty{}, nil
}

func (s *BuildEventService) PublishBuildToolEventStream(stream bpb.PublishBuildEvent_PublishBuildToolEventStreamServer) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		fmt.Printf("# BEP BuildToolEvent message:\n%s\n\n", prototext.Format(req))

		res := &bpb.PublishBuildToolEventStreamResponse{
			StreamId: req.GetOrderedBuildEvent().StreamId,
			SequenceNumber: req.GetOrderedBuildEvent().SequenceNumber,
		}
		if err := stream.Send(res); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	grpcs := grpc.NewServer()
	bpb.RegisterPublishBuildEventServer(grpcs, &BuildEventService{})

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server.CloudRun(mux, grpcs)
}
