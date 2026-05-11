package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/edaywalid/sched/internal/observability"
	"github.com/edaywalid/sched/proto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type DashboardServer struct {
	engineClient proto.EngineServiceClient
	conn         *grpc.ClientConn
}

func NewDashboardServer(engineAddress string) (*DashboardServer, error) {
	conn, err := grpc.NewClient(engineAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to engine: %w", err)
	}
	return &DashboardServer{
		engineClient: proto.NewEngineServiceClient(conn),
		conn:         conn,
	}, nil
}

func (s *DashboardServer) Close() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *DashboardServer) Start(address string) error {
	mux := http.NewServeMux()
	s.registerAPI(mux)

	if static, ok := webFS(); ok {
		mux.HandleFunc("/", spaHandler(static))
	} else {
		mux.HandleFunc("/", placeholderHandler())
	}

	handler := withCORS(mux)
	log.Printf("Dashboard server starting on %s", address)
	return http.ListenAndServe(address, handler)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	observability.NewLogger("dashboard")

	shutdownTracing, err := observability.InitTracing(context.Background(), "dashboard")
	if err != nil {
		log.Printf("init tracing: %v (continuing without tracer)", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	engineAddress := getEnv("ENGINE_ADDRESS", "localhost:50051")
	dashboardPort := getEnv("DASHBOARD_PORT", "8080")

	log.Printf("Connecting to engine at %s", engineAddress)

	dashboard, err := NewDashboardServer(engineAddress)
	if err != nil {
		log.Fatalf("Failed to create dashboard: %v", err)
	}
	defer dashboard.Close()

	address := fmt.Sprintf(":%s", dashboardPort)
	if err := dashboard.Start(address); err != nil {
		log.Fatalf("Failed to start dashboard: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
