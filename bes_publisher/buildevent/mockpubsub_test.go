package buildevent

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/mock"
)

type mockTopic struct {
	mock.Mock
}

func (m *mockTopic) Publish(ctx context.Context, msg *pubsub.Message) fetcher {
	args := m.Called(ctx, msg)
	return args.Get(0).(fetcher)
}

// newMockPublishResult returns a mockPublishResult with expectations pre-set:
// * Get() returns a specified error after a specified amount of time `delay`
func newMockPublishResult(delay time.Duration, err error) *mockPublishResult {
	m := &mockPublishResult{}
	m.On("Get", mock.Anything).After(delay).Return("", err).Once()
	return m
}

type mockPublishResult struct {
	mock.Mock
}

func (m *mockPublishResult) Get(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}
