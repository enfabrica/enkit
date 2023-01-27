package buildevent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/emptypb"

	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream" // Allows prototext to automatically decode embedded messages
)

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
)

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
		stream: stream,
	}
	return bs.handleMessages()
}

// buildStream wraps a single stream (for a single build) so that it can
// aggregate state seen across the entire stream, such as invocation ID and
// build type.
type buildStream struct {
	stream bpb.PublishBuildEvent_PublishBuildToolEventStreamServer
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

	// TODO(scott): insert this message into PubSub
	glog.V(2).Infof("# Bazel event:\n%s", prototext.Format(event))
	metricBuildEventServiceEventCount.WithLabelValues(oneofType(event.Payload), "ok").Inc()

	return nil
}
