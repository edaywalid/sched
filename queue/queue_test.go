package queue

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryQueue_EnqueueDequeue(t *testing.T) {
	ctx := context.Background()
	q := NewInMemoryQueue()
	defer q.Close()

	if err := q.Enqueue(ctx, "k", []byte("hello")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	msg, err := q.Dequeue(ctx, "k", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if msg == nil {
		t.Fatalf("Dequeue: got nil, want message")
	}
	if string(msg.Payload) != "hello" {
		t.Fatalf("payload = %q, want %q", msg.Payload, "hello")
	}
}

func TestInMemoryQueue_DequeueTimeout(t *testing.T) {
	ctx := context.Background()
	q := NewInMemoryQueue()
	defer q.Close()

	start := time.Now()
	msg, err := q.Dequeue(ctx, "empty", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if msg != nil {
		t.Fatalf("Dequeue on empty: got %v, want nil", msg)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Fatalf("returned before timeout: %v", elapsed)
	}
}

func TestInMemoryQueue_AckReclaimAreNoop(t *testing.T) {
	ctx := context.Background()
	q := NewInMemoryQueue()
	defer q.Close()

	if err := q.Ack(ctx, "k", "anything"); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	msgs, err := q.Reclaim(ctx, "k", time.Second)
	if err != nil {
		t.Fatalf("Reclaim: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("Reclaim = %v, want empty", msgs)
	}
}

func TestInMemoryQueue_DequeueCtxCancel(t *testing.T) {
	q := NewInMemoryQueue()
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := q.Dequeue(ctx, "k", time.Second); err == nil {
		t.Fatalf("Dequeue with cancelled ctx: got nil err")
	}
}
