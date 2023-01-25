package buildevent

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/mock"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"

	"github.com/enfabrica/enkit/lib/errdiff"
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

func TestPublishBuildToolEventStream(t *testing.T) {
	testCases := []struct {
		desc          string
		events        []*bpb.PublishBuildToolEventStreamRequest
		streamSendErr error
		streamRecvErr error

		wantErr string
	}{
		{
			desc:    "no events",
			events:  []*bpb.PublishBuildToolEventStreamRequest{},
			wantErr: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			service := &Service{}

			stream := &mockStream{}
			stream.On("Send", mock.Anything).Return(tc.streamSendErr)
			for _, event := range tc.events {
				stream.On("Recv").Return(event, nil).Once()
			}
			if tc.streamRecvErr != nil {
				stream.On("Recv").Return(nil, tc.streamRecvErr).Once()
			} else {
				stream.On("Recv").Return(nil, io.EOF).Once()
			}

			gotErr := service.PublishBuildToolEventStream(stream)

			errdiff.Check(t, gotErr, tc.wantErr)
		})
	}
}
