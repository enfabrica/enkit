package buildevent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/emptypb"

	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages
	"github.com/enfabrica/enkit/lib/gmap"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var (
	metricLifecycleEventCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bes_publisher",
		Name:      "lifecycle_event_count",
		Help:      "Number of BEP lifecycle events, grouped by how they were handled",
	},
		[]string{
			"event_type",
			"outcome",
		},
	)
	metricBuildEventProtocolEventCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bes_publisher",
		Name:      "bep_event_count",
		Help:      "Number of BEP events, grouped by how they were handled",
	},
		[]string{
			"event_type",
			"outcome",
		},
	)
	metricBuildEventServiceEventCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bes_publisher",
		Name:      "bes_event_count",
		Help:      "Number of BES events, grouped by how they were handled",
	},
		[]string{
			"event_type",
			"outcome",
		},
	)
	metricBuildEventProtocolStreamCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bes_publisher",
		Name:      "bep_stream_count",
		Help:      "Number of BEP streams handled, grouped by result",
	},
		[]string{
			"outcome",
		},
	)
	metricUnknownBuildType = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bes_publisher",
		Name:      "unknown_build_type",
		Help:      "Number of invocations with build_metadata ROLE set to an unrecognized value",
	},
		[]string{
			"role",
		},
	)
)

// randomMs returns a random duration between `low` and `high` milliseconds.
func randomMs(low int, high int) time.Duration {
	return time.Millisecond * time.Duration(rand.Intn(high-low)+low)
}

// oneofType returns a friendly string for a protobuf oneof type to use in
// logging/metrics.
//
// Converts a string like `*proto.BuildEvent_BuildMetadata` to `BuildMetadata`.
func oneofType(msg any) string {
	ret := "<unknown>"
	elems := strings.Split(fmt.Sprintf("%T", msg), "_")
	if len(elems) < 2 {
		return ret
	}
	return elems[1]
}

// bazelEventFrom returns a bazel BES BuildEvent from the given event stream
// message, or an error if the message is of a different type.
func bazelEventFrom(req *bpb.PublishBuildToolEventStreamRequest) (*bes.BuildEvent, error) {
	switch event := req.GetOrderedBuildEvent().GetEvent().Event.(type) {
	default:
		metricBuildEventProtocolEventCount.WithLabelValues(oneofType(event), "dropped").Inc()
		return nil, fmt.Errorf("not handling unknown BEP event type: %T", event)
	case *bpb.BuildEvent_BazelEvent:
		buildEvent := &bes.BuildEvent{}
		if err := ptypes.UnmarshalAny(event.BazelEvent, buildEvent); err != nil {
			metricBuildEventProtocolEventCount.WithLabelValues(oneofType(event), "parse_fail").Inc()
			return nil, fmt.Errorf("failed to unmarshal embedded BazelEvent: %w", err)
		}
		metricBuildEventProtocolEventCount.WithLabelValues(oneofType(event), "ok").Inc()
		return buildEvent, nil
	}
}

// Service implements the Build Event Protocol service.
type Service struct {
	besTopic sender
}

func NewService(besTopic sender) (*Service, error) {
	return &Service{
		besTopic: besTopic,
	}, nil
}

// PublishLifecycleEvent records the BEP lifecycle events seen in a metric, and
// not much else.
func (s *Service) PublishLifecycleEvent(ctx context.Context, req *bpb.PublishLifecycleEventRequest) (*emptypb.Empty, error) {
	glog.V(2).Infof("# BEP LifecycleEvent message:\n%s", prototext.Format(req))
	metricLifecycleEventCount.WithLabelValues(oneofType(req.GetBuildEvent().GetEvent().GetEvent()), "dropped").Inc()
	return &emptypb.Empty{}, nil
}

// PublishBuildToolEventStream handles all the Bazel BES messages seen.
func (s *Service) PublishBuildToolEventStream(stream bpb.PublishBuildEvent_PublishBuildToolEventStreamServer) (retErr error) {
	bs := &buildStream{
		stream:             stream,
		besTopic:           s.besTopic,
		attrs:              map[string]string{},
		errs:               newErrslice(),
		outstandingPublish: sync.WaitGroup{},
	}
	if err := bs.handleMessages(); err != nil {
		glog.Errorf("while handling messages from BEP stream: %v", err)
		metricBuildEventProtocolStreamCount.WithLabelValues("message_handle_error").Inc()
	}
	if err := bs.Close(); err != nil {
		glog.Errorf("while finalizing messages from BEP stream: %v", err)
		metricBuildEventProtocolStreamCount.WithLabelValues("finalize_error").Inc()
	}
	metricBuildEventProtocolStreamCount.WithLabelValues("ok").Inc()
	return nil
}

// buildStream wraps a single stream (for a single build) so that it can
// aggregate state seen across the entire stream, such as invocation ID and
// build type.
type buildStream struct {
	stream   bpb.PublishBuildEvent_PublishBuildToolEventStreamServer
	besTopic sender

	attrs              map[string]string
	errs               *errslice
	outstandingPublish sync.WaitGroup
}

func (b *buildStream) Close() error {
	b.outstandingPublish.Wait()
	return b.errs.Close()
}

// handleMessages handles all the messages on the stream, and then returns an
// error if it encounters a non-EOF error while exhausting the stream.
func (b *buildStream) handleMessages() error {
	for {
		req, err := b.stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := b.handleEvent(req); err != nil {
			continue
		}
	}
}

// handleEvent handles a single event, making sure to ack the event even in the
// case of an error. It returns an error if this event was not handled.
func (b *buildStream) handleEvent(req *bpb.PublishBuildToolEventStreamRequest) error {
	// The upstream sender is expecting an ack to be sent, regardless of whether
	// this message was handled or not.
	defer func() {
		res := &bpb.PublishBuildToolEventStreamResponse{
			StreamId:       req.GetOrderedBuildEvent().StreamId,
			SequenceNumber: req.GetOrderedBuildEvent().SequenceNumber,
		}
		if err := b.stream.Send(res); err != nil {
			glog.Error("failed to send event ack: %v", err)
		}
	}()

	event, err := bazelEventFrom(req)
	if err != nil {
		return err
	}

	glog.V(2).Infof("# Bazel event:\n%s", prototext.Format(event))
	b.updateAttrs(event)
	if err := b.maybePublish(event); err != nil {
		return err
	}
	return nil
}

// updateAttrs updates the attribute set that is sent out with each pubsub
// message.
func (b *buildStream) updateAttrs(event *bes.BuildEvent) {
	switch payload := event.Payload.(type) {
	case *bes.BuildEvent_Started:
		b.attrs["inv_id"] = payload.Started.GetUuid()
	case *bes.BuildEvent_BuildMetadata:
		if role, ok := payload.BuildMetadata.GetMetadata()["ROLE"]; ok {
			switch role {
			case "interactive":
				b.attrs["inv_type"] = "interactive"
			case "presubmit":
				b.attrs["inv_type"] = "presubmit"
			case "CI":
				b.attrs["inv_type"] = "postsubmit"
			default:
				metricUnknownBuildType.WithLabelValues(role).Inc()
			}
		} else {
			metricUnknownBuildType.WithLabelValues("<unset>").Inc()
		}
	case *bes.BuildEvent_Finished:
		b.attrs["result"] = payload.Finished.GetExitCode().GetName()
	}
}

// maybePublish publishes the given event if it is one that we care about;
// otherwise, the event is dropped.
func (b *buildStream) maybePublish(event *bes.BuildEvent) error {
	copy := &bes.BuildEvent{Id: event.Id, Payload: event.Payload}

	switch event.Payload.(type) {
	default:
		metricBuildEventServiceEventCount.WithLabelValues(oneofType(event.Payload), "dropped").Inc()
		return nil
	case *bes.BuildEvent_Started:
	case *bes.BuildEvent_BuildMetadata:
	case *bes.BuildEvent_WorkspaceStatus:
	case *bes.BuildEvent_TestResult:
	case *bes.BuildEvent_Finished:
	case *bes.BuildEvent_BuildMetrics:
	}

	contents, err := protojson.Marshal(copy)
	if err != nil {
		metricBuildEventServiceEventCount.WithLabelValues(oneofType(event.Payload), "marshal_failure").Inc()
		return err
	}

	res := b.besTopic.Publish(b.stream.Context(), &pubsub.Message{
		Data:       contents,
		Attributes: gmap.Copy(b.attrs),
	})

	b.outstandingPublish.Add(1)
	go b.recordErrFrom(res)

	metricBuildEventServiceEventCount.WithLabelValues(oneofType(event.Payload), "ok").Inc()
	return nil
}

// recordErrFrom waits on the fetcher and records any error the fetcher reports.
func (b *buildStream) recordErrFrom(res fetcher) {
	defer b.outstandingPublish.Done()
	_, err := res.Get(b.stream.Context())
	if err != nil {
		b.errs.Append(err)
	}
}
