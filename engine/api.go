package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/edaywalid/sched/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

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

	var input interface{}
	if len(req.Input) > 0 {
		if err := json.Unmarshal(req.Input, &input); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input: %w", err)
		}
	}

	s.engine.persistence.CreateWorkflowExecution(workflowID, req.WorkflowName, input)

	inputBytes, _ := json.Marshal(input)
	task := &WorkflowTaskInfo{
		TaskToken:    uuid.New().String(),
		WorkflowID:   workflowID,
		RunID:        runID,
		WorkflowName: req.WorkflowName,
		Input:        inputBytes,
		ResultCh:     make(chan *WorkflowResult),
	}

	go func() {
		s.workflowTasksCh <- task

		result := <-task.ResultCh
		if result.Error != "" {
			s.engine.persistence.CompleteWorkflow(workflowID, nil, fmt.Errorf("%s", result.Error))
		} else {
			var output interface{}
			if len(result.Result) > 0 {
				json.Unmarshal(result.Result, &output)
			}
			s.engine.persistence.CompleteWorkflow(workflowID, output, nil)
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

	// Send result back
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
	s.engine.persistence.AddEvent(req.WorkflowId, EventTypeSignalReceived, map[string]interface{}{
		"signal_name": req.SignalName,
		"input":       req.Input,
	})

	s.mu.Lock()
	if signalCh, ok := s.engine.signals[req.WorkflowId]; ok {
		signalCh <- &Signal{
			Name:  req.SignalName,
			Input: req.Input,
		}
	}
	s.mu.Unlock()

	return &proto.SignalWorkflowResponse{Success: true}, nil
}

func (s *EngineServer) GetWorkflowStatus(ctx context.Context, req *proto.GetWorkflowStatusRequest) (*proto.GetWorkflowStatusResponse, error) {
	status, result, err := s.engine.persistence.GetWorkflowStatus(req.WorkflowId)

	var errStr string
	if err != nil {
		errStr = err.Error()
	}

	var resultBytes []byte
	if result != nil {
		resultBytes, _ = json.Marshal(result)
	}

	return &proto.GetWorkflowStatusResponse{
		Status: status,
		Result: resultBytes,
		Error:  errStr,
	}, nil
}

func (s *EngineServer) ScheduleActivity(ctx context.Context, req *proto.ScheduleActivityRequest) (*proto.ScheduleActivityResponse, error) {
	activityID := uuid.New().String()

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

func (s *EngineServer) scheduleActivityInternal(workflowID, activityName string, input interface{}) (interface{}, error) {
	taskToken := uuid.New().String()

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	task := &ActivityTask{
		TaskToken:    taskToken,
		ActivityName: activityName,
		Input:        inputBytes,
		WorkflowID:   workflowID,
		ResultCh:     make(chan *ActivityResult),
	}

	s.activityTasksCh <- task

	result := <-task.ResultCh
	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}

	var output interface{}
	if len(result.Result) > 0 {
		if err := json.Unmarshal(result.Result, &output); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return output, nil
}

 
func (s *EngineServer) ListWorkflows(ctx context.Context, req *proto.ListWorkflowsRequest) (*proto.ListWorkflowsResponse, error) {
	executions := s.engine.GetAllWorkflowExecutions()

	var workflows []*proto.WorkflowExecutionInfo
	for _, exec := range executions {
		// Filter by status if requested
		if req.StatusFilter != "" && string(exec.Status) != req.StatusFilter {
			continue
		}

		wf := &proto.WorkflowExecutionInfo{
			WorkflowId:   exec.WorkflowID,
			RunId:        exec.WorkflowID, // Using workflow ID as run ID for now
			WorkflowName: exec.WorkflowName,
			Status:       string(exec.Status),
			StartTime:    exec.StartTime.Unix(),
		}

		if exec.EndTime != nil {
			wf.EndTime = exec.EndTime.Unix()
		}

		if exec.Result != nil {
			resultBytes, _ := json.Marshal(exec.Result)
			wf.Result = string(resultBytes)
		}

		if exec.Error != nil {
			wf.Error = exec.Error.Error()
		}

		workflows = append(workflows, wf)
	}

	return &proto.ListWorkflowsResponse{
		Workflows: workflows,
	}, nil
}

func (s *EngineServer) GetWorkflowDetails(ctx context.Context, req *proto.GetWorkflowDetailsRequest) (*proto.GetWorkflowDetailsResponse, error) {
	exec, err := s.engine.GetWorkflowExecution(req.WorkflowId)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	wf := &proto.WorkflowExecutionInfo{
		WorkflowId:   exec.WorkflowID,
		RunId:        exec.WorkflowID,
		WorkflowName: exec.WorkflowName,
		Status:       string(exec.Status),
		StartTime:    exec.StartTime.Unix(),
	}

	if exec.EndTime != nil {
		wf.EndTime = exec.EndTime.Unix()
	}

	if exec.Result != nil {
		resultBytes, _ := json.Marshal(exec.Result)
		wf.Result = string(resultBytes)
	}

	if exec.Error != nil {
		wf.Error = exec.Error.Error()
	}

	// Convert history events
	var history []*proto.WorkflowEvent
	for _, event := range exec.History {
		detailsBytes, _ := json.Marshal(event.Details)
		history = append(history, &proto.WorkflowEvent{
			EventType: event.EventType,
			Timestamp: event.Timestamp.Unix(),
			Details:   string(detailsBytes),
		})
	}

	return &proto.GetWorkflowDetailsResponse{
		Execution: wf,
		History:   history,
	}, nil
}

func (s *EngineServer) GetWorkflowMetrics(ctx context.Context, req *proto.GetWorkflowMetricsRequest) (*proto.GetWorkflowMetricsResponse, error) {
	executions := s.engine.GetAllWorkflowExecutions()

	metrics := &proto.WorkflowMetrics{
		WorkflowsByType: make(map[string]int32),
	}

	var totalExecutionTime int64
	var completedCount int32

	for _, exec := range executions {
		metrics.TotalWorkflows++

		switch string(exec.Status) {
		case "Running":
			metrics.RunningWorkflows++
		case "Completed":
			metrics.CompletedWorkflows++
			completedCount++

			if exec.EndTime != nil {
				executionTime := exec.EndTime.Sub(exec.StartTime).Milliseconds()
				totalExecutionTime += executionTime
			}
		case "Failed":
			metrics.FailedWorkflows++
		}

		// Count by workflow type
		metrics.WorkflowsByType[exec.WorkflowName]++
	}

	if completedCount > 0 {
		metrics.AvgExecutionTimeMs = float64(totalExecutionTime) / float64(completedCount)
	}

	return &proto.GetWorkflowMetricsResponse{
		Metrics: metrics,
	}, nil
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
