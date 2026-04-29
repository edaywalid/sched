package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/edaywalid/sched/engine"
	"github.com/edaywalid/sched/internal/observability"
	"github.com/edaywalid/sched/internal/store"
	"github.com/edaywalid/sched/queue"
	"github.com/joho/godotenv"
)

// shutdownGracePeriod caps how long graceful stop is allowed to run
// before the engine forces the gRPC server down and exits. Kept
// shorter than Docker's default 10s stop grace so `docker stop` works
// without a custom stop_grace_period. Operators with long-running
// activities can raise SCHED_SHUTDOWN_GRACE_SECONDS.
const shutdownGracePeriod = 8 * time.Second

// defaultLeaderLockKey is the pg_advisory_lock key used to elect a
// single active engine across replicas. The value is arbitrary; what
// matters is that every engine in the cluster uses the same number.
// Operators can override via SCHED_LEADER_LOCK_KEY for multi-tenant
// deployments that share a Postgres instance.
const defaultLeaderLockKey int64 = 0x53636845644c6431 // "SchEdLd1"

func main() {
	_ = godotenv.Load()
	logger := observability.NewLogger("engine")

	shutdownTracing, err := observability.InitTracing(context.Background(), "engine")
	if err != nil {
		logger.Warn("init tracing", slog.Any("error", err))
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	enginePort := getEnv("ENGINE_PORT", "50051")
	dsn := getEnv("SCHED_POSTGRES_DSN", os.Getenv("POSTGRES_DSN"))
	redisAddr := getEnv("REDIS_ADDR", "")

	s, err := openStore(dsn)
	if err != nil {
		logger.Error("open store", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = s.Close() }()

	q, err := openQueue(redisAddr)
	if err != nil {
		logger.Error("open queue", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = q.Close() }()

	// Leader election. When the engine has a Postgres store we hold
	// an advisory lock for the lifetime of leadership. In-memory mode
	// (no DSN) is single-process by definition so we skip.
	var lease *store.LeaderLease
	if pg, ok := s.(*store.PostgresStore); ok {
		lockKey := defaultLeaderLockKey
		if raw := os.Getenv("SCHED_LEADER_LOCK_KEY"); raw != "" {
			if v, err := strconv.ParseInt(raw, 0, 64); err == nil {
				lockKey = v
			}
		}
		logger.Info("waiting for leader lease", slog.Int64("lock_key", lockKey))
		l, err := store.AcquireLeaderLease(context.Background(), pg.Pool(), lockKey, 5*time.Second)
		if err != nil {
			logger.Error("acquire leader lease", slog.Any("error", err))
			os.Exit(1)
		}
		lease = l
		logger.Info("became leader")
		defer lease.Release(context.Background())
	}

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
	gs, err := engine.NewGRPCServer(e, q, metrics, engineAddr)
	if err != nil {
		logger.Error("build gRPC server", slog.Any("error", err))
		os.Exit(1)
	}

	// Run the gRPC server in a goroutine so the main goroutine can
	// listen for signals and drive a graceful shutdown.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("starting gRPC server", slog.String("addr", engineAddr))
		serveErr <- gs.Serve()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// leaseLost is the channel that closes when the leader lease is
	// dropped. When that happens we treat it as a shutdown signal so
	// another engine can pick up leadership.
	var leaseLost <-chan struct{}
	if lease != nil {
		leaseLost = lease.Lost()
	}

	drain := func(reason string) {
		logger.Info("draining", slog.String("reason", reason))
		grace := shutdownGracePeriod
		if raw := os.Getenv("SCHED_SHUTDOWN_GRACE_SECONDS"); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil && v > 0 {
				grace = time.Duration(v) * time.Second
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), grace)
		gs.Stop(ctx)
		cancel()
		e.TimerManager().Stop()
		select {
		case <-serveErr:
		case <-time.After(time.Second):
		}
	}

	select {
	case sig := <-signals:
		drain("signal=" + sig.String())
	case <-leaseLost:
		logger.Warn("leader lease lost, shutting down so another engine can take over")
		drain("lease_lost")
		os.Exit(1)
	case err := <-serveErr:
		if err != nil {
			logger.Error("gRPC server exited", slog.Any("error", err))
			os.Exit(1)
		}
	}
	logger.Info("engine shutdown complete")
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
