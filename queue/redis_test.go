package queue

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestRedisQueue_RoundTrip exercises the streams-backed queue against a
// real Redis. It's skipped unless SCHED_TEST_REDIS_ADDR is set, so the
// default `go test ./...` stays infra-free.
//
//	docker run --rm -d --name sched-test-redis -p 56379:6379 redis:7-alpine
//	SCHED_TEST_REDIS_ADDR=localhost:56379 go test ./queue -run TestRedisQueue -v
func TestRedisQueue_RoundTrip(t *testing.T) {
	addr := os.Getenv("SCHED_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("SCHED_TEST_REDIS_ADDR not set")
	}

	ctx := context.Background()
	q, err := NewRedisQueue(ctx, RedisOptions{
		Addr:         addr,
		ConsumerName: "test-consumer",
		Group:        "sched-test",
	})
	if err != nil {
		t.Fatalf("NewRedisQueue: %v", err)
	}
	defer q.Close()

	stream := "queue-test-" + time.Now().Format("150405.000")

	if err := q.Enqueue(ctx, stream, []byte("payload-1")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := q.Enqueue(ctx, stream, []byte("payload-2")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	msg, err := q.Dequeue(ctx, stream, 2*time.Second)
	if err != nil {
		t.Fatalf("Dequeue #1: %v", err)
	}
	if msg == nil || string(msg.Payload) != "payload-1" {
		t.Fatalf("Dequeue #1 = %v, want payload-1", msg)
	}
	if msg.AckToken == "" {
		t.Fatalf("Dequeue: AckToken empty")
	}
	firstToken := msg.AckToken

	// Ack the first message; second remains pending.
	if err := q.Ack(ctx, stream, firstToken); err != nil {
		t.Fatalf("Ack: %v", err)
	}

	// Dequeue the second message but do not Ack it.
	msg2, err := q.Dequeue(ctx, stream, 2*time.Second)
	if err != nil {
		t.Fatalf("Dequeue #2: %v", err)
	}
	if msg2 == nil || string(msg2.Payload) != "payload-2" {
		t.Fatalf("Dequeue #2 = %v, want payload-2", msg2)
	}

	// Reclaim should not see it yet (idle is well under 250ms).
	reclaimed, err := q.Reclaim(ctx, stream, 5*time.Second)
	if err != nil {
		t.Fatalf("Reclaim too-soon: %v", err)
	}
	if len(reclaimed) != 0 {
		t.Fatalf("Reclaim too-soon = %v, want empty", reclaimed)
	}

	// Wait past the idle threshold, then Reclaim with a different
	// consumer name should pick it up.
	time.Sleep(300 * time.Millisecond)
	q2, err := NewRedisQueue(ctx, RedisOptions{
		Addr:         addr,
		ConsumerName: "test-consumer-2",
		Group:        "sched-test",
	})
	if err != nil {
		t.Fatalf("NewRedisQueue q2: %v", err)
	}
	defer q2.Close()

	reclaimed, err = q2.Reclaim(ctx, stream, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Reclaim: %v", err)
	}
	if len(reclaimed) != 1 {
		t.Fatalf("Reclaim len = %d, want 1", len(reclaimed))
	}
	if string(reclaimed[0].Payload) != "payload-2" {
		t.Fatalf("Reclaim payload = %q, want payload-2", reclaimed[0].Payload)
	}
	if reclaimed[0].AckToken == "" {
		t.Fatalf("Reclaim: AckToken empty")
	}

	if err := q2.Ack(ctx, stream, reclaimed[0].AckToken); err != nil {
		t.Fatalf("Ack reclaimed: %v", err)
	}

	// Dequeue timeout returns (nil, nil).
	noMsg, err := q.Dequeue(ctx, stream, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Dequeue empty: %v", err)
	}
	if noMsg != nil {
		t.Fatalf("Dequeue empty = %v, want nil", noMsg)
	}
}
