package buildevent

import (
	"context"
	"io"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/testutil"
	bes "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
)

func TestServicePublishLifecycleEvent(t *testing.T) {
	testCases := []struct {
		desc    string
		req     *bpb.PublishLifecycleEventRequest
		wantErr string
	}{
		{
			desc: "no error on any call",
			req:  &bpb.PublishLifecycleEventRequest{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			service := &Service{}

			_, gotErr := service.PublishLifecycleEvent(ctx, tc.req)

			errdiff.Check(t, gotErr, tc.wantErr)
		})
	}
}

func anypbOrDie(msg proto.Message) *anypb.Any {
	a, err := anypb.New(msg)
	if err != nil {
		panic(err)
	}
	return a
}

func wrapBesMessages(msgs []*bes.BuildEvent) []*bpb.PublishBuildToolEventStreamRequest {
	var wrapped []*bpb.PublishBuildToolEventStreamRequest
	for i, msg := range msgs {
		wrapped = append(wrapped, &bpb.PublishBuildToolEventStreamRequest{
			OrderedBuildEvent: &bpb.OrderedBuildEvent{
				SequenceNumber: int64(i),
				Event: &bpb.BuildEvent{
					Event: &bpb.BuildEvent_BazelEvent{
						BazelEvent: anypbOrDie(msg),
					},
				},
			},
		})
	}
	return wrapped
}

func TestPublishBuildToolEventStream(t *testing.T) {
	testCases := []struct {
		desc          string
		events        []*bes.BuildEvent
		streamSendErr error
		streamRecvErr error

		wantMessages []*pubsub.Message
		wantErr      string
	}{
		{
			desc:    "no events",
			events:  []*bes.BuildEvent{},
			wantErr: "",
		},
		{
			desc: "build started",
			events: []*bes.BuildEvent{
				&bes.BuildEvent{
					Payload: &bes.BuildEvent_Started{
						Started: &bes.BuildStarted{
							Uuid: "d9b5cec0-c1e6-428c-8674-a74194b27447",
						},
					},
				},
			},
			wantMessages: []*pubsub.Message{
				&pubsub.Message{
					Data: []byte(`{"started":{"uuid":"d9b5cec0-c1e6-428c-8674-a74194b27447"}}`),
					Attributes: map[string]string{
						"inv_id": "d9b5cec0-c1e6-428c-8674-a74194b27447",
					},
				},
			},
			wantErr: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			bepEvents := wrapBesMessages(tc.events)

			topic := &mockTopic{}
			service, err := NewService(topic)
			require.NoError(t, err)

			stream := &mockStream{}
			stream.On("Context").Return(ctx)
			stream.On("Send", mock.Anything).Return(tc.streamSendErr)
			for _, event := range bepEvents {
				stream.On("Recv").Return(event, nil).Once()
			}
			if tc.streamRecvErr != nil {
				stream.On("Recv").Return(nil, tc.streamRecvErr).Once()
			} else {
				stream.On("Recv").Return(nil, io.EOF).Once()
			}

			var sentMessages []*pubsub.Message
			topic.On("Publish", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				sent := args.Get(1).(*pubsub.Message)
				sentMessages = append(sentMessages, sent)
			}).Return(newMockPublishResult(randomMs(10, 100), nil))

			gotErr := service.PublishBuildToolEventStream(stream)

			errdiff.Check(t, gotErr, tc.wantErr)
			testutil.AssertCmp(t, sentMessages, tc.wantMessages, cmpopts.IgnoreUnexported(pubsub.Message{}))
		})
	}
}
