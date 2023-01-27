package buildevent

import (
	"context"

	"cloud.google.com/go/pubsub"
)

// fetcher wraps the interface exposed by pubsub.MessageResult.
type fetcher interface {
	Get(context.Context) (string, error)
}

// sender wraps the interface exposed by pubsub.Topic.
type sender interface {
	Publish(context.Context, *pubsub.Message) fetcher
}

// Topic wraps a pubsub.Topic so that it can expose the proper return type for
// Publish().
type Topic struct {
	*pubsub.Topic
}

// NewTopic wraps a pubsub.Topic with a *Topic so it can be used as a `sender`.
func NewTopic(t *pubsub.Topic) *Topic {
	return &Topic{t}
}

// Publish effectively casts a pubsub.MessageResult into a fetcher.
func (t *Topic) Publish(ctx context.Context, msg *pubsub.Message) fetcher {
	return t.Topic.Publish(ctx, msg)
}
