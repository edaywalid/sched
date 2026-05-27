package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/edaywalid/sched/engine"
	"github.com/edaywalid/sched/internal/store"
	"github.com/edaywalid/sched/queue"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	enginePort := getEnv("ENGINE_PORT", "50051")
	dsn := getEnv("SCHED_POSTGRES_DSN", os.Getenv("POSTGRES_DSN"))
	redisAddr := getEnv("REDIS_ADDR", "")

	s, err := openStore(dsn)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	q, err := openQueue(redisAddr)
	if err != nil {
		log.Fatalf("open queue: %v", err)
	}
	defer q.Close()

	e := engine.NewEngine(s)

	engineAddr := fmt.Sprintf(":%s", enginePort)
	log.Printf("Starting Engine gRPC server on %s", engineAddr)
	if err := engine.StartGRPCServer(e, q, engineAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func openStore(dsn string) (store.Store, error) {
	if dsn == "" {
		log.Println("SCHED_POSTGRES_DSN not set — using in-memory store (state will NOT survive restart)")
		return store.NewMemoryStore(), nil
	}
	log.Printf("Opening Postgres store")
	return store.NewPostgresStore(context.Background(), dsn)
}

func openQueue(redisAddr string) (queue.Queue, error) {
	if redisAddr == "" {
		log.Println("REDIS_ADDR not set — using in-memory queue (single-process only)")
		return queue.NewInMemoryQueue(), nil
	}
	log.Printf("Opening Redis Streams queue at %s", redisAddr)
	return queue.NewRedisQueue(context.Background(), queue.RedisOptions{Addr: redisAddr})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
