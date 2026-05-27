package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/edaywalid/sched/internal/store"
	"github.com/edaywalid/sched/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

// EngineServer implements the gRPC EngineService.
//
// In Phase 1 task dispatch still uses in-process Go channels keyed by
// task token. Phase 2 replaces these with Redis Streams so workers in
// other processes can pick up work.
type EngineServer struct {
	proto.UnimplementedEngineServiceServer
	engine          *Engine
	workflowTasksCh chan *WorkflowTaskInfo
	activityTasksCh chan *ActivityTask
	pendingWfTasks  map[string]*WorkflowTaskInfo
	pendingActTasks map[string]*ActivityTask
	mu              sync.RWMutex
}

type WorkflowTaskInfo struct {
	TaskToken    string
	WorkflowID   string
	RunID        string
	WorkflowName string
	Input        []byte
	ResultCh     chan *WorkflowResult
}

type WorkflowResult struct {
	Result []byte
	Error  string
}

type ActivityTask struct {
	TaskToken    string
	ActivityName string
	Input        []byte
	WorkflowID   string
	ResultCh     chan *ActivityResult
}

type ActivityResult struct {
	Result []byte
	Error  string
}

func NewEngineServer(engine *Engine) *EngineServer {
	return &EngineServer{
		engine:          engine,
		workflowTasksCh: make(chan *WorkflowTaskInfo, 100),
		activityTasksCh: make(chan *ActivityTask, 100),
		pendingWfTasks:  make(map[string]*WorkflowTaskInfo),
		pendingActTasks: make(map[string]*ActivityTask),
	}
}

func (s *EngineServer) StartWorkflow(ctx context.Context, req *proto.StartWorkflowRequest) (*proto.StartWorkflowResponse, error) {
	workflowID := uuid.New().String()
	runID := uuid.New().String()

	if err := s.engine.store.CreateWorkflow(ctx, store.Workflow{
		WorkflowID: workflowID,
		RunID:      runID,
		Name:       req.WorkflowName,
		Status:     store.StatusRunning,
		Input:      req.Input,
	}); err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}

	startedDetails, _ := json.Marshal(map[string]any{
		"workflow_name": req.WorkflowName,
	})
	if err := s.engine.store.AppendEvent(ctx, store.Event{
		WorkflowID: workflowID,
		Type:       EventTypeWorkflowStarted,
		Details:    startedDetails,
	}); err != nil {
		return nil, fmt.Errorf("append start event: %w", err)
	}

	task := &WorkflowTaskInfo{
		TaskToken:    uuid.New().String(),
		WorkflowID:   workflowID,
		RunID:        runID,
		WorkflowName: req.WorkflowName,
		Input:        req.Input,
		ResultCh:     make(chan *WorkflowResult),
	}

	go func() {
		s.workflowTasksCh <- task

		result := <-task.ResultCh

		// Background context: the caller's RPC has already returned by
		// the time the workflow completes.
		bgCtx := context.Background()
		if result.Error != "" {
			completedDetails, _ := json.Marshal(map[string]any{"error": result.Error})
			_ = s.engine.store.AppendEvent(bgCtx, store.Event{
				WorkflowID: workflowID,
				Type:       EventTypeWorkflowFailed,
				Details:    completedDetails,
			})
			_ = s.engine.store.CompleteWorkflow(bgCtx, workflowID, store.StatusFailed, nil, result.Error)
		} else {
			completedDetails, _ := json.Marshal(map[string]any{"result": json.RawMessage(result.Result)})
			_ = s.engine.store.AppendEvent(bgCtx, store.Event{
				WorkflowID: workflowID,
				Type:       EventTypeWorkflowCompleted,
				Details:    completedDetails,
			})
			_ = s.engine.store.CompleteWorkflow(bgCtx, workflowID, store.StatusCompleted, result.Result, "")
		}
	}()

	log.Printf("Queued workflow %s (ID: %s)", req.WorkflowName, workflowID)

	return &proto.StartWorkflowResponse{
		WorkflowId: workflowID,
		RunId:      runID,
	}, nil
}

func (s *EngineServer) PollWorkflowTask(ctx context.Context, req *proto.PollWorkflowTaskRequest) (*proto.PollWorkflowTaskResponse, error) {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	select {
	case task := <-s.workflowTasksCh:
		s.mu.Lock()
		s.pendingWfTasks[task.TaskToken] = task
		s.mu.Unlock()

		return &proto.PollWorkflowTaskResponse{
			TaskToken:    task.TaskToken,
			WorkflowName: task.WorkflowName,
			Input:        task.Input,
			WorkflowId:   task.WorkflowID,
			RunId:        task.RunID,
		}, nil
	case <-time.After(timeout):
		return &proto.PollWorkflowTaskResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *EngineServer) CompleteWorkflowTask(ctx context.Context, req *proto.CompleteWorkflowTaskRequest) (*proto.CompleteWorkflowTaskResponse, error) {
	s.mu.Lock()
	task, ok := s.pendingWfTasks[req.TaskToken]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("workflow task not found: %s", req.TaskToken)
	}
	delete(s.pendingWfTasks, req.TaskToken)
	s.mu.Unlock()

	task.ResultCh <- &WorkflowResult{
		Result: req.Result,
		Error:  req.Error,
	}

	return &proto.CompleteWorkflowTaskResponse{Success: true}, nil
}

func (s *EngineServer) CompleteActivity(ctx context.Context, req *proto.CompleteActivityRequest) (*proto.CompleteActivityResponse, error) {
	s.mu.Lock()
	task, ok := s.pendingActTasks[req.TaskToken]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("task not found: %s", req.TaskToken)
	}
	delete(s.pendingActTasks, req.TaskToken)
	s.mu.Unlock()

	task.ResultCh <- &ActivityResult{
		Result: req.Result,
		Error:  req.Error,
	}

	return &proto.CompleteActivityResponse{Success: true}, nil
}

func (s *EngineServer) PollActivityTask(ctx context.Context, req *proto.PollActivityTaskRequest) (*proto.PollActivityTaskResponse, error) {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	select {
	case task := <-s.activityTasksCh:
		s.mu.Lock()
		s.pendingActTasks[task.TaskToken] = task
		s.mu.Unlock()

		return &proto.PollActivityTaskResponse{
			TaskToken:    task.TaskToken,
			ActivityName: task.ActivityName,
			Input:        task.Input,
			WorkflowId:   task.WorkflowID,
		}, nil
	case <-time.After(timeout):
		return &proto.PollActivityTaskResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *EngineServer) SignalWorkflow(ctx context.Context, req *proto.SignalWorkflowRequest) (*proto.SignalWorkflowResponse, error) {
	details, _ := json.Marshal(map[string]any{
		"signal_name": req.SignalName,
		"input":       json.RawMessage(req.Input),
	})
	if err := s.engine.store.AppendEvent(ctx, store.Event{
		WorkflowID: req.WorkflowId,
		Type:       EventTypeSignalReceived,
		Details:    details,
	}); err != nil {
		return nil, fmt.Errorf("record signal: %w", err)
	}

	// Phase 3 fully wires the signal channel into the workflow task
	// loop. For now we only persist the event and deliver to any
	// in-process listener if present.
	s.engine.signalsMu.RLock()
	signalCh, ok := s.engine.signals[req.WorkflowId]
	s.engine.signalsMu.RUnlock()
	if ok {
		select {
		case signalCh <- &Signal{Name: req.SignalName, Input: req.Input}:
		default:
		}
	}

	return &proto.SignalWorkflowResponse{Success: true}, nil
}

func (s *EngineServer) GetWorkflowStatus(ctx context.Context, req *proto.GetWorkflowStatusRequest) (*proto.GetWorkflowStatusResponse, error) {
	wf, err := s.engine.store.GetWorkflow(ctx, req.WorkflowId)
	if err != nil {
		return nil, err
	}
	return &proto.GetWorkflowStatusResponse{
		Status: string(wf.Status),
		Result: wf.Result,
		Error:  wf.Error,
	}, nil
}

func (s *EngineServer) ScheduleActivity(ctx context.Context, req *proto.ScheduleActivityRequest) (*proto.ScheduleActivityResponse, error) {
	activityID := uuid.New().String()

	scheduledDetails, _ := json.Marshal(map[string]any{
		"activity_id":   activityID,
		"activity_name": req.ActivityName,
	})
	if err := s.engine.store.AppendEvent(ctx, store.Event{
		WorkflowID: req.WorkflowId,
		Type:       EventTypeActivityScheduled,
		Details:    scheduledDetails,
	}); err != nil {
		return nil, fmt.Errorf("record scheduled activity: %w", err)
	}

	task := &ActivityTask{
		TaskToken:    uuid.New().String(),
		ActivityName: req.ActivityName,
		Input:        req.Input,
		WorkflowID:   req.WorkflowId,
		ResultCh:     make(chan *ActivityResult),
	}

	s.activityTasksCh <- task

	return &proto.ScheduleActivityResponse{
		ActivityId: activityID,
	}, nil
}

func (s *EngineServer) ListWorkflows(ctx context.Context, req *proto.ListWorkflowsRequest) (*proto.ListWorkflowsResponse, error) {
	filter := store.ListFilter{Limit: int(req.PageSize)}
	if req.StatusFilter != "" {
		filter.Status = store.WorkflowStatus(req.StatusFilter)
	}
	executions, err := s.engine.ListWorkflows(ctx, filter)
	if err != nil {
		return nil, err
	}

	workflows := make([]*proto.WorkflowExecutionInfo, 0, len(executions))
	for _, exec := range executions {
		workflows = append(workflows, workflowToProto(exec))
	}
	return &proto.ListWorkflowsResponse{Workflows: workflows}, nil
}

func (s *EngineServer) GetWorkflowDetails(ctx context.Context, req *proto.GetWorkflowDetailsRequest) (*proto.GetWorkflowDetailsResponse, error) {
	exec, err := s.engine.GetWorkflow(ctx, req.WorkflowId)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}
	history, err := s.engine.store.GetHistory(ctx, req.WorkflowId)
	if err != nil {
		return nil, fmt.Errorf("fetch history: %w", err)
	}

	events := make([]*proto.WorkflowEvent, 0, len(history))
	for _, ev := range history {
		events = append(events, &proto.WorkflowEvent{
			EventType: ev.Type,
			Timestamp: ev.Timestamp.Unix(),
			Details:   string(ev.Details),
		})
	}

	return &proto.GetWorkflowDetailsResponse{
		Execution: workflowToProto(exec),
		History:   events,
	}, nil
}

func (s *EngineServer) GetWorkflowMetrics(ctx context.Context, req *proto.GetWorkflowMetricsRequest) (*proto.GetWorkflowMetricsResponse, error) {
	executions, err := s.engine.ListWorkflows(ctx, store.ListFilter{Limit: 10000})
	if err != nil {
		return nil, err
	}

	metrics := &proto.WorkflowMetrics{
		WorkflowsByType: make(map[string]int32),
	}

	var totalExecutionTime int64
	var completedCount int32

	for _, exec := range executions {
		metrics.TotalWorkflows++

		switch exec.Status {
		case store.StatusRunning:
			metrics.RunningWorkflows++
		case store.StatusCompleted:
			metrics.CompletedWorkflows++
			completedCount++
			if exec.EndTime != nil {
				totalExecutionTime += exec.EndTime.Sub(exec.StartTime).Milliseconds()
			}
		case store.StatusFailed:
			metrics.FailedWorkflows++
		}

		metrics.WorkflowsByType[exec.Name]++
	}

	if completedCount > 0 {
		metrics.AvgExecutionTimeMs = float64(totalExecutionTime) / float64(completedCount)
	}

	return &proto.GetWorkflowMetricsResponse{Metrics: metrics}, nil
}

func workflowToProto(w *store.Workflow) *proto.WorkflowExecutionInfo {
	info := &proto.WorkflowExecutionInfo{
		WorkflowId:   w.WorkflowID,
		RunId:        w.RunID,
		WorkflowName: w.Name,
		Status:       string(w.Status),
		StartTime:    w.StartTime.Unix(),
		Result:       string(w.Result),
		Error:        w.Error,
	}
	if w.EndTime != nil {
		info.EndTime = w.EndTime.Unix()
	}
	return info
}

func StartGRPCServer(engine *Engine, address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	engineServer := NewEngineServer(engine)
	proto.RegisterEngineServiceServer(grpcServer, engineServer)

	log.Printf("Engine gRPC server listening on %s", address)
	return grpcServer.Serve(lis)
}
