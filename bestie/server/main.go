package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/enfabrica/enkit/lib/server"
	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages
	"github.com/golang/protobuf/ptypes"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Derive a unique invocation SHA value.
func deriveInvocationSha(invocationId string) string {
	// The SHA value allows combining multiple items in the future
	// (e.g. invocationId, buildId, timestamp) to uniquely identify
	// a bazel test stream without impacting the database schema.
	//
	// We still want to store the actual invocationId in the database
	// to match it up with build log references, etc.
	hash := sha256.Sum256([]byte(invocationId))
	return hex.EncodeToString(hash[:])
}

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

		// Access protobuf message sections of interest.
		obe := req.GetOrderedBuildEvent()
		event := obe.Event
		//bazelEvent := event.GetBazelEvent()

		streamId := obe.StreamId
		invocationId := streamId.InvocationId
		//buildId := streamId.BuildId
		invocationSha := deriveInvocationSha(invocationId)

		switch buildEvent := event.Event.(type) {
		case *bpb.BuildEvent_BazelEvent:
			var bazelBuildEvent bes.BuildEvent
			//fmt.Printf("BuildTool: BuildEvent_BazelEvent: \n%T\n%s\n\n", buildEvent.BazelEvent, buildEvent.BazelEvent)
			if err := ptypes.UnmarshalAny(buildEvent.BazelEvent, &bazelBuildEvent); err != nil {
				return err
			}
			if m := bazelBuildEvent.GetTestResult(); m != nil {
				if err := handleTestResultEvent(bazelBuildEvent, invocationId, invocationSha); err != nil {
					return err
				}
			}
		default:
			fmt.Printf("Ignoring Bazel event type %T\n", buildEvent)
		}

		res := &bpb.PublishBuildToolEventStreamResponse{
			StreamId:       req.GetOrderedBuildEvent().StreamId,
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
