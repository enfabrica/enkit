package buildevent

import (
	"context"

	"github.com/stretchr/testify/mock"
	bpb "google.golang.org/genproto/googleapis/devtools/build/v1"
	"google.golang.org/grpc/metadata"
)

type mockStream struct {
	mock.Mock
}

func (m *mockStream) Send(res *bpb.PublishBuildToolEventStreamResponse) error {
	args := m.Called(res)
	return args.Error(0)
}

func (m *mockStream) Recv() (*bpb.PublishBuildToolEventStreamRequest, error) {
	args := m.Called()
	err := args.Error(1)
	if err != nil {
		return nil, err
	}
	return args.Get(0).(*bpb.PublishBuildToolEventStreamRequest), nil
}

func (m *mockStream) SetHeader(meta metadata.MD) error {
	args := m.Called(meta)
	return args.Error(0)
}

func (m *mockStream) SendHeader(meta metadata.MD) error {
	args := m.Called(meta)
	return args.Error(0)
}

func (m *mockStream) SetTrailer(meta metadata.MD) {
	_ = m.Called(meta)
}

func (m *mockStream) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *mockStream) SendMsg(msg interface{}) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockStream) RecvMsg(msg interface{}) error {
	args := m.Called()
	return args.Error(0)
}
