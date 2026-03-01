package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/edaywalid/sched/engine"
	"github.com/edaywalid/sched/internal/observability"
	"github.com/edaywalid/sched/internal/store"
	"github.com/edaywalid/sched/queue"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	logger := observability.NewLogger("engine")

	enginePort := getEnv("ENGINE_PORT", "50051")
	dsn := getEnv("SCHED_POSTGRES_DSN", os.Getenv("POSTGRES_DSN"))
	redisAddr := getEnv("REDIS_ADDR", "")

	s, err := openStore(dsn)
	if err != nil {
		logger.Error("open store", slog.Any("error", err))
		os.Exit(1)
	}
	defer s.Close()

	q, err := openQueue(redisAddr)
	if err != nil {
		logger.Error("open queue", slog.Any("error", err))
		os.Exit(1)
	}
	defer q.Close()

	e := engine.NewEngine(s)
	metrics := observability.NewMetrics()

	if n, err := e.TimerManager().RecoverPendingTimers(context.Background()); err != nil {
		logger.Warn("recover pending timers", slog.Any("error", err))
	} else if n > 0 {
		logger.Info("recovered pending timers", slog.Int("count", n))
	}

	metricsPort, _ := strconv.Atoi(getEnv("SCHED_METRICS_PORT", "9090"))
	shutdownMetrics, metricsErr := observability.StartMetricsServer(metricsPort)
	defer shutdownMetrics()
	go func() {
		if err, ok := <-metricsErr; ok && err != nil {
			logger.Error("metrics server", slog.Any("error", err))
		}
	}()
	logger.Info("metrics server listening", slog.Int("port", metricsPort))

	engineAddr := fmt.Sprintf(":%s", enginePort)
	logger.Info("starting gRPC server", slog.String("addr", engineAddr))
	if err := engine.StartGRPCServer(e, q, metrics, engineAddr); err != nil {
		logger.Error("gRPC server exited", slog.Any("error", err))
		os.Exit(1)
	}
}

func openStore(dsn string) (store.Store, error) {
	if dsn == "" {
		slog.Warn("SCHED_POSTGRES_DSN not set, using in-memory store (state will NOT survive restart)")
		return store.NewMemoryStore(), nil
	}
	slog.Info("opening Postgres store")
	return store.NewPostgresStore(context.Background(), dsn)
}

func openQueue(redisAddr string) (queue.Queue, error) {
	if redisAddr == "" {
		slog.Warn("REDIS_ADDR not set, using in-memory queue (single-process only)")
		return queue.NewInMemoryQueue(), nil
	}
	slog.Info("opening Redis Streams queue", slog.String("addr", redisAddr))
	return queue.NewRedisQueue(context.Background(), queue.RedisOptions{Addr: redisAddr})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
