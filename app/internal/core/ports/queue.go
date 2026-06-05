package ports

import (
	"context"

	"magicstrike/internal/core/entities"
)

// EventPublisher defines the interface to publish game events to a message broker/queue.
type EventPublisher interface {
	Publish(ctx context.Context, event *entities.Event) error
	PublishBatch(ctx context.Context, events []*entities.Event) error
}

// EventSubscriber defines the interface to consume events from a message broker/queue.
type EventSubscriber interface {
	Subscribe(ctx context.Context, handler func(context.Context, *entities.Event) error) error
}

// DemoJobMessage is the serializable message envelope for demo processing jobs.
type DemoJobMessage struct {
	MatchID    string `json:"match_id"`
	BucketPath string `json:"bucket_path"`
	MD5Hash    string `json:"md5_hash"`
	UploadedAt int64  `json:"uploaded_at"` // Unix timestamp in nanoseconds
}

// DemoQueuePublisher defines the output port for publishing demo processing jobs.
// Implementations MUST guarantee at-least-once delivery (publisher confirms).
type DemoQueuePublisher interface {
	// PublishDemoJob publishes a demo processing job to the queue.
	// Returns ErrQueueUnavailable if the broker cannot be reached.
	PublishDemoJob(ctx context.Context, job *DemoJobMessage) error
}

// DemoQueueSubscriber defines the output port for subscribing to demo processing jobs.
type DemoQueueSubscriber interface {
	// SubscribeDemoJobs registers a handler for incoming demo jobs.
	// The handler is called sequentially for each message; it MUST ACK on success
	// and NACK (with requeue=false for permanent failures, requeue=true for transient)
	// on failure.
	// Blocks until ctx is cancelled. Returns nil on clean shutdown,
	// or ErrQueueUnavailable if the connection is lost.
	SubscribeDemoJobs(ctx context.Context, handler func(context.Context, *DemoJobMessage) error) error
}
