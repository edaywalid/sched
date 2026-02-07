package queue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// defaultConsumerGroup is the consumer-group name created on each
// stream. All workers share the group so XREADGROUP can fan messages
// out across them.
const defaultConsumerGroup = "sched"

// payloadField is the single field used inside each stream entry.
const payloadField = "p"

// RedisQueue is a Queue backed by Redis Streams with consumer groups.
//
// Each queue name maps to one Redis stream. Enqueue appends with XADD;
// Dequeue blocks on XREADGROUP and remembers the entry ID so the
// returned Message.AckToken can be passed back to Ack (XACK). Reclaim
// uses XPENDING + XCLAIM to recover entries whose consumer has been
// idle longer than the supplied timeout.
type RedisQueue struct {
	client       *redis.Client
	consumerName string
	group        string

	mu      sync.Mutex
	groups  map[string]struct{} // streams we have already ensured exist
}

// RedisOptions configures a RedisQueue.
type RedisOptions struct {
	// Addr is the host:port of the Redis server, e.g. "redis:6379".
	Addr string
	// ConsumerName uniquely identifies this worker within the consumer
	// group. Defaults to "<hostname>-<pid>".
	ConsumerName string
	// Group is the consumer-group name. Defaults to "sched".
	Group string
	// Password and DB are forwarded to the Redis client.
	Password string
	DB       int
}

func NewRedisQueue(ctx context.Context, opts RedisOptions) (*RedisQueue, error) {
	if opts.Addr == "" {
		return nil, errors.New("redis queue: Addr is required")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis queue ping: %w", err)
	}

	consumer := opts.ConsumerName
	if consumer == "" {
		host, _ := os.Hostname()
		consumer = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	group := opts.Group
	if group == "" {
		group = defaultConsumerGroup
	}

	return &RedisQueue{
		client:       client,
		consumerName: consumer,
		group:        group,
		groups:       make(map[string]struct{}),
	}, nil
}

// ConsumerName returns the consumer identifier this queue uses when
// reading from streams. Exposed for logging and tests.
func (q *RedisQueue) ConsumerName() string { return q.consumerName }

// ensureGroup creates the consumer group + stream on first use.
// XGROUP CREATE with MKSTREAM is idempotent: re-running it returns
// BUSYGROUP, which we swallow.
func (q *RedisQueue) ensureGroup(ctx context.Context, stream string) error {
	q.mu.Lock()
	if _, ok := q.groups[stream]; ok {
		q.mu.Unlock()
		return nil
	}
	q.mu.Unlock()

	err := q.client.XGroupCreateMkStream(ctx, stream, q.group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("create consumer group %q on %q: %w", q.group, stream, err)
	}

	q.mu.Lock()
	q.groups[stream] = struct{}{}
	q.mu.Unlock()
	return nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, queueName string, payload []byte) error {
	if err := q.ensureGroup(ctx, queueName); err != nil {
		return err
	}
	err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: queueName,
		Values: map[string]any{payloadField: payload},
	}).Err()
	if err != nil {
		return fmt.Errorf("xadd %q: %w", queueName, err)
	}
	return nil
}

func (q *RedisQueue) Dequeue(ctx context.Context, queueName string, timeout time.Duration) (*Message, error) {
	if err := q.ensureGroup(ctx, queueName); err != nil {
		return nil, err
	}
	res, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumerName,
		Streams:  []string{queueName, ">"},
		Count:    1,
		Block:    timeout,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil // blocked the full timeout without seeing anything
	}
	if err != nil {
		return nil, fmt.Errorf("xreadgroup %q: %w", queueName, err)
	}
	for _, stream := range res {
		for _, msg := range stream.Messages {
			payload, _ := extractPayload(msg.Values)
			return &Message{AckToken: msg.ID, Payload: payload}, nil
		}
	}
	return nil, nil
}

func (q *RedisQueue) Ack(ctx context.Context, queueName string, ackToken string) error {
	if ackToken == "" {
		return nil
	}
	if err := q.client.XAck(ctx, queueName, q.group, ackToken).Err(); err != nil {
		return fmt.Errorf("xack %q %q: %w", queueName, ackToken, err)
	}
	// XACK does not delete the entry; trim it so the stream does not
	// grow unbounded. Best-effort: ignore errors.
	_ = q.client.XDel(ctx, queueName, ackToken).Err()
	return nil
}

func (q *RedisQueue) Reclaim(ctx context.Context, queueName string, idleTimeout time.Duration) ([]Message, error) {
	if err := q.ensureGroup(ctx, queueName); err != nil {
		return nil, err
	}
	pending, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: queueName,
		Group:  q.group,
		Idle:   idleTimeout,
		Start:  "-",
		End:    "+",
		Count:  64,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("xpending %q: %w", queueName, err)
	}
	if len(pending) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(pending))
	for _, p := range pending {
		ids = append(ids, p.ID)
	}

	claimed, err := q.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   queueName,
		Group:    q.group,
		Consumer: q.consumerName,
		MinIdle:  idleTimeout,
		Messages: ids,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("xclaim %q: %w", queueName, err)
	}

	out := make([]Message, 0, len(claimed))
	for _, msg := range claimed {
		payload, _ := extractPayload(msg.Values)
		out = append(out, Message{AckToken: msg.ID, Payload: payload})
	}
	return out, nil
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}

func extractPayload(values map[string]any) ([]byte, error) {
	raw, ok := values[payloadField]
	if !ok {
		return nil, errors.New("payload field missing")
	}
	switch v := raw.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("payload has unexpected type %T", raw)
	}
}
