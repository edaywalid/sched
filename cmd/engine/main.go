package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/edaywalid/sched/engine"
	"github.com/edaywalid/sched/internal/store"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	enginePort := getEnv("ENGINE_PORT", "50051")
	dsn := getEnv("SCHED_POSTGRES_DSN", os.Getenv("POSTGRES_DSN"))

	s, err := openStore(dsn)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	e := engine.NewEngine(s)

	engineAddr := fmt.Sprintf(":%s", enginePort)
	log.Printf("Starting Engine gRPC server on %s", engineAddr)
	if err := engine.StartGRPCServer(e, engineAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// openStore selects the Store implementation based on whether a Postgres
// DSN is configured. When unset, the engine uses an in-memory store so
// `go run ./cmd/engine` works without any infrastructure (state is lost
// on restart).
func openStore(dsn string) (store.Store, error) {
	if dsn == "" {
		log.Println("SCHED_POSTGRES_DSN not set — using in-memory store (state will NOT survive restart)")
		return store.NewMemoryStore(), nil
	}
	log.Printf("Opening Postgres store")
	return store.NewPostgresStore(context.Background(), dsn)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
