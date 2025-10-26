package main

import (
	"fmt"
	"log"
	"os"

	"github.com/edaywalid/sched/engine"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	enginePort := getEnv("ENGINE_PORT", "50051")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")

	var e *engine.Engine
	var err error

	if redisAddr != "" && redisAddr != "localhost:6379" {
		log.Printf("Using Redis at %s", redisAddr)
		e, err = engine.NewEngineWithRedis(redisAddr)
		if err != nil {
			log.Printf("Failed to connect to Redis, falling back to in-memory queue: %v", err)
			e = engine.NewEngine()
		}
	} else {
		log.Println("Using in-memory queue")
		e = engine.NewEngine()
	}

	engineAddr := fmt.Sprintf(":%s", enginePort)
	log.Printf("Starting Engine gRPC server on %s", engineAddr)
	if err := engine.StartGRPCServer(e, engineAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
