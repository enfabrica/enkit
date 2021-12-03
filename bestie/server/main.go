package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/enfabrica/enkit/lib/server"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	//"google.golang.org/grpc/codes"
	//"google.golang.org/grpc/status"
	//"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	//"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/encoding/prototext"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
)

type BuildEventService struct {
}

func (s *BuildEventService) PublishLifecycleEvent(ctx context.Context, req *bpb.PublishLifecycleEventRequest) (*emptypb.Empty, error) {
	fmt.Printf("%s\n\n", prototext.Format(req))
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

		fmt.Printf("%s\n\n", prototext.Format(req))
		// TODO(scott): Print out Bazel BES message once protos are available
		// if bazelEvent, ok := req.GetOrderedBuildEvent().GetEvent().GetEvent().(*bpb.BuildEvent_BazelEvent); ok {
		// 	event := &bespb.BuildEvent{}
		// 	if err := anypb.UnmarshalTo(bazelEvent.BazelEvent, event, proto.UnmarshalOptions{}); err != nil {
		// 		return err
		// 	}
		// 	fmt.Printf("%s\n\n", prototext.Format(event))
		// }

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
