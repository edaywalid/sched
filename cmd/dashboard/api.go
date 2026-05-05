package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/edaywalid/sched/proto"
)

type workflowSummary struct {
	WorkflowID   string `json:"workflowId"`
	RunID        string `json:"runId"`
	WorkflowName string `json:"workflowName"`
	Status       string `json:"status"`
	StartTime    int64  `json:"startTime"`
	EndTime      int64  `json:"endTime"`
	Result       string `json:"result"`
	Error        string `json:"error"`
}

type workflowEvent struct {
	EventType string `json:"eventType"`
	Timestamp int64  `json:"timestamp"`
	Details   string `json:"details"`
}

type workflowDetails struct {
	Execution workflowSummary `json:"execution"`
	History   []workflowEvent `json:"history"`
}

type workflowsResp struct {
	Workflows []workflowSummary `json:"workflows"`
}

type metricsResp struct {
	TotalWorkflows     int32            `json:"totalWorkflows"`
	RunningWorkflows   int32            `json:"runningWorkflows"`
	CompletedWorkflows int32            `json:"completedWorkflows"`
	FailedWorkflows    int32            `json:"failedWorkflows"`
	AvgExecutionTimeMs float64          `json:"avgExecutionTimeMs"`
	WorkflowsByType    map[string]int32 `json:"workflowsByType"`
}

type startWorkflowReq struct {
	WorkflowName             string `json:"workflowName"`
	Input                    string `json:"input"`
	ExecutionTimeoutSeconds  int32  `json:"executionTimeoutSeconds"`
}

type startWorkflowResp struct {
	WorkflowID string `json:"workflowId"`
	RunID      string `json:"runId"`
}

type cancelWorkflowReq struct {
	Reason string `json:"reason"`
}

func (s *DashboardServer) registerAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workflows", s.handleListWorkflows)
	mux.HandleFunc("POST /api/workflows", s.handleStartWorkflowJSON)
	mux.HandleFunc("GET /api/workflows/{id}", s.handleGetWorkflow)
	mux.HandleFunc("POST /api/workflows/{id}/cancel", s.handleCancelWorkflowJSON)
	mux.HandleFunc("GET /api/metrics", s.handleMetricsJSON)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func (s *DashboardServer) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	statusFilter := r.URL.Query().Get("status")
	limit := int32(0)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = int32(v)
		}
	}

	resp, err := s.engineClient.ListWorkflows(ctx, &proto.ListWorkflowsRequest{
		StatusFilter: statusFilter,
		PageSize:     limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	out := workflowsResp{Workflows: make([]workflowSummary, 0, len(resp.Workflows))}
	for _, wf := range resp.Workflows {
		out.Workflows = append(out.Workflows, toSummary(wf))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *DashboardServer) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing workflow id"))
		return
	}

	resp, err := s.engineClient.GetWorkflowDetails(ctx, &proto.GetWorkflowDetailsRequest{
		WorkflowId: id,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	events := make([]workflowEvent, 0, len(resp.History))
	for _, ev := range resp.History {
		events = append(events, workflowEvent{
			EventType: ev.EventType,
			Timestamp: ev.Timestamp * 1000,
			Details:   ev.Details,
		})
	}
	writeJSON(w, http.StatusOK, workflowDetails{
		Execution: toSummary(resp.Execution),
		History:   events,
	})
}

func (s *DashboardServer) handleStartWorkflowJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req startWorkflowReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.WorkflowName) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflowName is required"))
		return
	}

	var input []byte
	trimmed := strings.TrimSpace(req.Input)
	if trimmed != "" {
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			if json.Valid([]byte(trimmed)) {
				input = []byte(trimmed)
			} else {
				input, _ = json.Marshal(trimmed)
			}
		} else {
			input, _ = json.Marshal(trimmed)
		}
	}

	resp, err := s.engineClient.StartWorkflow(ctx, &proto.StartWorkflowRequest{
		WorkflowName:                    req.WorkflowName,
		Input:                           input,
		WorkflowExecutionTimeoutSeconds: req.ExecutionTimeoutSeconds,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, startWorkflowResp{
		WorkflowID: resp.WorkflowId,
		RunID:      resp.RunId,
	})
}

func (s *DashboardServer) handleCancelWorkflowJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing workflow id"))
		return
	}
	var req cancelWorkflowReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "cancelled from dashboard"
	}
	if _, err := s.engineClient.CancelWorkflow(ctx, &proto.CancelWorkflowRequest{
		WorkflowId: id,
		Reason:     req.Reason,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *DashboardServer) handleMetricsJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	resp, err := s.engineClient.GetWorkflowMetrics(ctx, &proto.GetWorkflowMetricsRequest{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	m := resp.Metrics
	writeJSON(w, http.StatusOK, metricsResp{
		TotalWorkflows:     m.TotalWorkflows,
		RunningWorkflows:   m.RunningWorkflows,
		CompletedWorkflows: m.CompletedWorkflows,
		FailedWorkflows:    m.FailedWorkflows,
		AvgExecutionTimeMs: m.AvgExecutionTimeMs,
		WorkflowsByType:    m.WorkflowsByType,
	})
}

func toSummary(wf *proto.WorkflowExecutionInfo) workflowSummary {
	if wf == nil {
		return workflowSummary{}
	}
	return workflowSummary{
		WorkflowID:   wf.WorkflowId,
		RunID:        wf.RunId,
		WorkflowName: wf.WorkflowName,
		Status:       wf.Status,
		StartTime:    wf.StartTime * 1000,
		EndTime:      wf.EndTime * 1000,
		Result:       wf.Result,
		Error:        wf.Error,
	}
}

func requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 10*time.Second)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
