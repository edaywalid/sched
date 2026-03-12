package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/edaywalid/sched/cmd/dashboard/templates"
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
	// Connect to engine via gRPC. The otelgrpc client handler propagates
	// trace context so a span started in the dashboard request handler
	// shows up as the parent of the engine-side span.
	conn, err := grpc.NewClient(engineAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to engine: %w", err)
	}

	client := proto.NewEngineServiceClient(conn)

	return &DashboardServer{
		engineClient: client,
		conn:         conn,
	}, nil
}

func (s *DashboardServer) Close() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

// Handlers using templ

func (s *DashboardServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch workflows and metrics
	workflowsResp, err := s.engineClient.ListWorkflows(ctx, &proto.ListWorkflowsRequest{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch workflows: %v", err), http.StatusInternalServerError)
		return
	}

	metricsResp, err := s.engineClient.GetWorkflowMetrics(ctx, &proto.GetWorkflowMetricsRequest{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch metrics: %v", err), http.StatusInternalServerError)
		return
	}

	// Render with templ
	component := templates.Index(metricsResp.Metrics, workflowsResp.Workflows)
	_ = component.Render(ctx, w)
}

func (s *DashboardServer) handleMetricsHTML(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metricsResp, err := s.engineClient.GetWorkflowMetrics(ctx, &proto.GetWorkflowMetricsRequest{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch metrics: %v", err), http.StatusInternalServerError)
		return
	}

	component := templates.MetricsGrid(metricsResp.Metrics)
	_ = component.Render(ctx, w)
}

func (s *DashboardServer) handleWorkflowsHTML(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statusFilter := r.URL.Query().Get("status")

	workflowsResp, err := s.engineClient.ListWorkflows(ctx, &proto.ListWorkflowsRequest{
		StatusFilter: statusFilter,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch workflows: %v", err), http.StatusInternalServerError)
		return
	}

	component := templates.WorkflowList(workflowsResp.Workflows)
	_ = component.Render(ctx, w)
}

func (s *DashboardServer) handleWorkflowDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	workflowID := r.URL.Path[len("/workflow/"):]
	if workflowID == "" {
		http.Error(w, "Missing workflow ID", http.StatusBadRequest)
		return
	}

	detailsResp, err := s.engineClient.GetWorkflowDetails(ctx, &proto.GetWorkflowDetailsRequest{
		WorkflowId: workflowID,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch workflow details: %v", err), http.StatusNotFound)
		return
	}

	component := templates.WorkflowDetail(detailsResp)
	_ = component.Render(ctx, w)
}

func (s *DashboardServer) handleStartWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<div class="error-message">Invalid form data: %v</div>`, err)
		return
	}

	workflowName := strings.TrimSpace(r.FormValue("workflow_name"))
	inputStr := strings.TrimSpace(r.FormValue("input"))

	if workflowName == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<div class="error-message">Workflow name is required</div>`)
		return
	}

	// Parse input - try JSON first, if fails use as plain string
	var input []byte
	if inputStr != "" {
		// Check if it looks like JSON
		if strings.HasPrefix(inputStr, "{") || strings.HasPrefix(inputStr, "[") {
			var jsonObj interface{}
			if err := json.Unmarshal([]byte(inputStr), &jsonObj); err != nil {
				// Not valid JSON, wrap as string
				input, _ = json.Marshal(inputStr)
			} else {
				input = []byte(inputStr)
			}
		} else {
			// Plain string, wrap in JSON
			input, _ = json.Marshal(inputStr)
		}
	}

	// Start the workflow
	resp, err := s.engineClient.StartWorkflow(ctx, &proto.StartWorkflowRequest{
		WorkflowName: workflowName,
		Input:        input,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<div class="error-message">Failed to start workflow: %v</div>`, err)
		return
	}

	// Return success message
	fmt.Fprintf(w, `<div class="success-message">Workflow started successfully! ID: %s</div>`, resp.WorkflowId)

	// Trigger refresh of workflows list
	w.Header().Set("HX-Trigger", "refresh")
}

func (s *DashboardServer) Start(address string) error {
	// Routes
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/api/metrics-html", s.handleMetricsHTML)
	http.HandleFunc("/api/workflows-html", s.handleWorkflowsHTML)
	http.HandleFunc("/api/start-workflow", s.handleStartWorkflow)
	http.HandleFunc("/workflow/", s.handleWorkflowDetail)

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	log.Printf("Dashboard server starting on %s", address)
	return http.ListenAndServe(address, nil)
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
