// Package queue is the task-queue abstraction the engine uses to hand
// workflow and activity tasks off to workers.
//
// A queue is identified by its name. Producers Enqueue raw bytes;
// consumers Dequeue Messages and must Ack them once the work is done.
// If a consumer crashes between Dequeue and Ack, Reclaim surfaces the
// stale messages so another consumer can finish the work.
//
// Two implementations live in this package:
//
//   - InMemoryQueue: process-local channels, used by tests and as a
//     fallback when no Redis is configured. Ack is a no-op because
//     there's no second consumer that could pick up the work.
//
//   - RedisQueue (added in Phase 2.2): Redis Streams with consumer
//     groups. Ack is XACK; Reclaim uses XPENDING + XCLAIM to recover
//     entries whose consumer has been idle longer than the visibility
//     timeout.
package queue

import (
	"context"
	"sync"
	"time"
)

// Message is a single item dequeued from a queue.
//
// AckToken is opaque to callers; for Redis Streams it is the stream
// entry ID, for the in-memory queue it is empty. The same token must
// be passed back to Ack.
type Message struct {
	AckToken string
	Payload  []byte
}

// Queue is the contract producers and consumers share. Implementations
// must be safe for concurrent use.
type Queue interface {
	// Enqueue appends payload to queueName. The byte slice is treated as
	// opaque; callers serialise their own structures.
	Enqueue(ctx context.Context, queueName string, payload []byte) error

	// Dequeue blocks for up to timeout waiting for a message on
	// queueName. A nil Message with a nil error means the wait timed
	// out without a message arriving.
	Dequeue(ctx context.Context, queueName string, timeout time.Duration) (*Message, error)

	// Ack marks a previously-dequeued message as processed. Calling Ack
	// with an unknown or already-acked token is not an error.
	Ack(ctx context.Context, queueName string, ackToken string) error

	// Reclaim returns messages that were dequeued by a consumer that
	// has since been idle longer than idleTimeout. Implementations that
	// do not have the notion of a separate consumer (e.g. InMemoryQueue)
	// return an empty slice.
	Reclaim(ctx context.Context, queueName string, idleTimeout time.Duration) ([]Message, error)

	// Close releases any resources held by the queue.
	Close() error
}

// InMemoryQueue is an in-process Queue backed by buffered Go channels.
// It is safe for concurrent use but offers no durability and no
// cross-process delivery — Ack and Reclaim are no-ops.
type InMemoryQueue struct {
	mu     sync.Mutex
	queues map[string]chan []byte
}

func NewInMemoryQueue() *InMemoryQueue {
	return &InMemoryQueue{queues: make(map[string]chan []byte)}
}

func (q *InMemoryQueue) channel(name string) chan []byte {
	q.mu.Lock()
	defer q.mu.Unlock()
	if ch, ok := q.queues[name]; ok {
		return ch
	}
	ch := make(chan []byte, 100)
	q.queues[name] = ch
	return ch
}

func (q *InMemoryQueue) Enqueue(ctx context.Context, queueName string, payload []byte) error {
	ch := q.channel(queueName)
	select {
	case ch <- payload:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *InMemoryQueue) Dequeue(ctx context.Context, queueName string, timeout time.Duration) (*Message, error) {
	ch := q.channel(queueName)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case payload := <-ch:
		return &Message{Payload: payload}, nil
	case <-timer.C:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Ack is a no-op for the in-memory queue: once a message has been read
// from a channel there is no possibility of redelivery.
func (q *InMemoryQueue) Ack(context.Context, string, string) error { return nil }

// Reclaim is a no-op for the in-memory queue: there is no second
// consumer that could pick up an abandoned message.
func (q *InMemoryQueue) Reclaim(context.Context, string, time.Duration) ([]Message, error) {
	return nil, nil
}

func (q *InMemoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, ch := range q.queues {
		close(ch)
	}
	q.queues = nil
	return nil
}
