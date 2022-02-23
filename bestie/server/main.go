package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/server"
	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages

	"github.com/golang/protobuf/ptypes"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	logger      = log.New(os.Stdout, "bestie: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC|log.Lshortfile|log.Lmsgprefix)
	isDebugMode bool // Set this to true to enable certain debug behaviors (e.g. special log messages).
)

func debugPrintf(format string, str ...interface{}) {
	if isDebugMode {
		logger.Printf(format, str...)
	}
}

func debugPrintln(str ...interface{}) {
	if isDebugMode {
		logger.Println(str...)
	}
}

type BuildEventService struct {
}

func (s *BuildEventService) PublishLifecycleEvent(ctx context.Context, req *bpb.PublishLifecycleEventRequest) (*emptypb.Empty, error) {
	logger.Printf("# BEP LifecycleEvent message:\n%s\n\n", prototext.Format(req))
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

		logger.Printf("# BEP BuildToolEvent message:\n%s\n\n", prototext.Format(req))

		// Access protobuf message sections of interest.
		obe := req.GetOrderedBuildEvent()
		event := obe.GetEvent()
		streamId := obe.GetStreamId()
		//bazelEvent := event.GetBazelEvent()

		// See BuildEvent.Event in build_events.pb.go for list of event types supported.
		switch buildEvent := event.Event.(type) {
		case *bpb.BuildEvent_BazelEvent:
			var bazelBuildEvent bes.BuildEvent
			if err := ptypes.UnmarshalAny(buildEvent.BazelEvent, &bazelBuildEvent); err != nil {
				return err
			}
			bazelEventId := bazelBuildEvent.GetId()
			if ok := bazelEventId.GetBuildFinished(); ok != nil {
				cidBuildsTotal.increment()
			}
			cidEventsTotal.updateWithLabel(getEventLabel(bazelEventId.Id), 1)
			if m := bazelBuildEvent.GetTestResult(); m != nil {
				if err := handleTestResultEvent(bazelBuildEvent, streamId); err != nil {
					logger.Printf("Error handling Bazel event %T: %s\n\n", bazelEventId.Id, err)
					return err
				}
			}
		default:
			debugPrintf("Ignoring Bazel event type %T\n\n", buildEvent)
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

// Command line arguments.
var (
	argBaseUrl   = flag.String("base_url", "", "Base URL for accessing output artifacts in the build cluster (required)")
	argDataset   = flag.String("dataset", "", "BigQuery dataset name (required) -- staging, production")
	argDebug     = flag.Bool("debug", false, "Enable debug mode within the server")
	argTableName = flag.String("table_name", "testmetrics", "BigQuery table name")
)

func checkCommandArgs() error {
	var errs []error
	// The --baseurl command line arg is required.
	// Note: This value is ignored for local invocations of the BES Endpoint and can be set to anything.
	if len(*argBaseUrl) == 0 {
		errs = append(errs, fmt.Errorf("--base_url must be specified"))
	}
	// The --dataset command line arg is required.
	if len(*argDataset) == 0 {
		errs = append(errs, fmt.Errorf("--dataset must be specified"))
	}
	if len(errs) > 0 {
		return multierror.New(errs)
	}

	// Set/override the default values.
	deploymentBaseUrl = *argBaseUrl
	isDebugMode = *argDebug
	bigQueryTableDefault.dataset = *argDataset
	bigQueryTableDefault.tableName = *argTableName

	return nil
}

func main() {
	ServiceStats.init()

	flag.Parse()
	if err := checkCommandArgs(); err != nil {
		log.Fatalf("Invalid command: %s", err)
	}

	grpcs := grpc.NewServer()
	bpb.RegisterPublishBuildEventServer(grpcs, &BuildEventService{})

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server.Run(mux, grpcs)
}
