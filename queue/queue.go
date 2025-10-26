package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Queue interface {
	Enqueue(ctx context.Context, queueName string, task interface{}) error
	Dequeue(ctx context.Context, queueName string, timeout time.Duration) ([]byte, error)
	Close() error
}

type InMemoryQueue struct {
	queues map[string]chan []byte
}

func NewInMemoryQueue() *InMemoryQueue {
	return &InMemoryQueue{
		queues: make(map[string]chan []byte),
	}
}

func (q *InMemoryQueue) Enqueue(ctx context.Context, queueName string, task interface{}) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	if _, ok := q.queues[queueName]; !ok {
		q.queues[queueName] = make(chan []byte, 100)
	}

	select {
	case q.queues[queueName] <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *InMemoryQueue) Dequeue(ctx context.Context, queueName string, timeout time.Duration) ([]byte, error) {
	if _, ok := q.queues[queueName]; !ok {
		q.queues[queueName] = make(chan []byte, 100)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case data := <-q.queues[queueName]:
		return data, nil
	case <-timeoutCtx.Done():
		return nil, nil 
	}
}

func (q *InMemoryQueue) Close() error {
	for _, ch := range q.queues {
		close(ch)
	}
	return nil
}

type RedisQueue struct {
	client *redis.Client
}

func NewRedisQueue(redisAddr string) (*RedisQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password
		DB:       0,  // default DB
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisQueue{client: client}, nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, queueName string, task interface{}) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	return q.client.RPush(ctx, queueName, data).Err()
}

func (q *RedisQueue) Dequeue(ctx context.Context, queueName string, timeout time.Duration) ([]byte, error) {
	result, err := q.client.BLPop(ctx, timeout, queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil 
		}
		return nil, fmt.Errorf("failed to dequeue: %w", err)
	}

	if len(result) < 2 {
		return nil, nil
	}

	return []byte(result[1]), nil
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}
