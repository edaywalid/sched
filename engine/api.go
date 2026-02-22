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
	"github.com/edaywalid/sched/queue"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

// Queue name prefixes. The Phase 2 dispatcher uses one Redis stream per
// (kind, task queue) pair. PollWorkflowTaskRequest / PollActivityTaskRequest
// carry the TaskQueue suffix from the worker; StartWorkflow and
// ScheduleActivity both default to "default" until the proto grows a
// task_queue field on the producer side.
const (
	queuePrefixWorkflow = "tasks:wf:"
	queuePrefixActivity = "tasks:act:"
	defaultTaskQueue    = "default"

	// Visibility timeout for reclaim. If a worker has held a task for
	// longer than this without acking via CompleteWorkflowTask /
	// CompleteActivity, the reclaim loop pulls it back so another
	// worker can pick it up.
	defaultVisibilityTimeout = 30 * time.Second
)

// EngineServer implements the gRPC EngineService.
//
// Workers poll PollWorkflowTask / PollActivityTask, which forward to
// queue.Queue (Redis Streams in production, in-memory channels for
// tests / local). On completion the matching ack token is XACKed and
// the workflow/activity events are persisted in the same call path.
type EngineServer struct {
	proto.UnimplementedEngineServiceServer
	engine *Engine
	queue  queue.Queue

	mu              sync.Mutex
	pendingWfTasks  map[string]*pendingWorkflowTask
	pendingActTasks map[string]*pendingActivityTask

	stopReclaim chan struct{}
	reclaimWG   sync.WaitGroup
}

// pendingWorkflowTask is the bookkeeping the engine keeps between a
// successful Poll and the corresponding Complete. AckToken is the
// stream entry ID; the workflow_id lets us write the right history
// row on completion.
type pendingWorkflowTask struct {
	TaskToken    string
	WorkflowID   string
	RunID        string
	WorkflowName string
	AckToken     string
	QueueName    string
	DequeuedAt   time.Time
}

type pendingActivityTask struct {
	TaskToken    string
	WorkflowID   string
	ActivityName string
	Input        []byte
	Attempt      int32
	MaxAttempts  int32
	Policy       RetryPolicy
	AckToken     string
	QueueName    string
	DequeuedAt   time.Time
}

// workflowEnvelope is the payload pushed onto the workflow task stream.
// Activity envelopes use a similar shape.
type workflowEnvelope struct {
	WorkflowID   string `json:"workflow_id"`
	RunID        string `json:"run_id"`
	WorkflowName string `json:"workflow_name"`
	Input        []byte `json:"input"`
}

type activityEnvelope struct {
	WorkflowID   string      `json:"workflow_id"`
	ActivityName string      `json:"activity_name"`
	Input        []byte      `json:"input"`
	Attempt      int32       `json:"attempt"`
	MaxAttempts  int32       `json:"max_attempts"`
	Policy       RetryPolicy `json:"retry_policy"`
}

func NewEngineServer(engine *Engine, q queue.Queue) *EngineServer {
	s := &EngineServer{
		engine:          engine,
		queue:           q,
		pendingWfTasks:  make(map[string]*pendingWorkflowTask),
		pendingActTasks: make(map[string]*pendingActivityTask),
		stopReclaim:     make(chan struct{}),
	}
	s.reclaimWG.Add(1)
	go s.reclaimLoop()
	return s
}

// Close stops the reclaim loop. The underlying queue and store are
// owned by the caller.
func (s *EngineServer) Close() {
	select {
	case <-s.stopReclaim:
	default:
		close(s.stopReclaim)
	}
	s.reclaimWG.Wait()
}

func workflowQueueName(taskQueue string) string {
	if taskQueue == "" {
		taskQueue = defaultTaskQueue
	}
	return queuePrefixWorkflow + taskQueue
}

func activityQueueName(taskQueue string) string {
	if taskQueue == "" {
		taskQueue = defaultTaskQueue
	}
	return queuePrefixActivity + taskQueue
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

	envelope, err := json.Marshal(workflowEnvelope{
		WorkflowID:   workflowID,
		RunID:        runID,
		WorkflowName: req.WorkflowName,
		Input:        req.Input,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal workflow envelope: %w", err)
	}
	if err := s.queue.Enqueue(ctx, workflowQueueName(defaultTaskQueue), envelope); err != nil {
		return nil, fmt.Errorf("enqueue workflow task: %w", err)
	}

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
	queueName := workflowQueueName(req.TaskQueue)

	msg, err := s.queue.Dequeue(ctx, queueName, timeout)
	if err != nil {
		return nil, fmt.Errorf("dequeue workflow task: %w", err)
	}
	if msg == nil {
		return &proto.PollWorkflowTaskResponse{}, nil
	}

	var env workflowEnvelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		_ = s.queue.Ack(ctx, queueName, msg.AckToken)
		return nil, fmt.Errorf("decode workflow envelope: %w", err)
	}

	taskToken := uuid.New().String()
	s.mu.Lock()
	s.pendingWfTasks[taskToken] = &pendingWorkflowTask{
		TaskToken:    taskToken,
		WorkflowID:   env.WorkflowID,
		RunID:        env.RunID,
		WorkflowName: env.WorkflowName,
		AckToken:     msg.AckToken,
		QueueName:    queueName,
		DequeuedAt:   time.Now(),
	}
	s.mu.Unlock()

	return &proto.PollWorkflowTaskResponse{
		TaskToken:    taskToken,
		WorkflowName: env.WorkflowName,
		Input:        env.Input,
		WorkflowId:   env.WorkflowID,
		RunId:        env.RunID,
	}, nil
}

func (s *EngineServer) CompleteWorkflowTask(ctx context.Context, req *proto.CompleteWorkflowTaskRequest) (*proto.CompleteWorkflowTaskResponse, error) {
	s.mu.Lock()
	pending, ok := s.pendingWfTasks[req.TaskToken]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("workflow task not found: %s", req.TaskToken)
	}
	delete(s.pendingWfTasks, req.TaskToken)
	s.mu.Unlock()

	if req.Error != "" {
		failed, _ := json.Marshal(map[string]any{"error": req.Error})
		_ = s.engine.store.AppendEvent(ctx, store.Event{
			WorkflowID: pending.WorkflowID,
			Type:       EventTypeWorkflowFailed,
			Details:    failed,
		})
		_ = s.engine.store.CompleteWorkflow(ctx, pending.WorkflowID, store.StatusFailed, nil, req.Error)
	} else {
		done, _ := json.Marshal(map[string]any{"result": json.RawMessage(req.Result)})
		_ = s.engine.store.AppendEvent(ctx, store.Event{
			WorkflowID: pending.WorkflowID,
			Type:       EventTypeWorkflowCompleted,
			Details:    done,
		})
		_ = s.engine.store.CompleteWorkflow(ctx, pending.WorkflowID, store.StatusCompleted, req.Result, "")
	}

	if err := s.queue.Ack(ctx, pending.QueueName, pending.AckToken); err != nil {
		log.Printf("ack workflow task %s: %v", req.TaskToken, err)
	}

	return &proto.CompleteWorkflowTaskResponse{Success: true}, nil
}

// RecordActivityHeartbeat resets the dequeue timestamp on the pending
// activity task so long-running activities are not reclaimed mid-flight.
// Returns cancel_requested=false until Phase 3 adds workflow-driven
// cancellation.
func (s *EngineServer) RecordActivityHeartbeat(ctx context.Context, req *proto.RecordActivityHeartbeatRequest) (*proto.RecordActivityHeartbeatResponse, error) {
	s.mu.Lock()
	pending, ok := s.pendingActTasks[req.TaskToken]
	if ok {
		pending.DequeuedAt = time.Now()
	}
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("activity task not found: %s", req.TaskToken)
	}
	return &proto.RecordActivityHeartbeatResponse{Success: true}, nil
}

func (s *EngineServer) PollActivityTask(ctx context.Context, req *proto.PollActivityTaskRequest) (*proto.PollActivityTaskResponse, error) {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	queueName := activityQueueName(req.TaskQueue)

	msg, err := s.queue.Dequeue(ctx, queueName, timeout)
	if err != nil {
		return nil, fmt.Errorf("dequeue activity task: %w", err)
	}
	if msg == nil {
		return &proto.PollActivityTaskResponse{}, nil
	}

	var env activityEnvelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		_ = s.queue.Ack(ctx, queueName, msg.AckToken)
		return nil, fmt.Errorf("decode activity envelope: %w", err)
	}

	taskToken := uuid.New().String()
	s.mu.Lock()
	s.pendingActTasks[taskToken] = &pendingActivityTask{
		TaskToken:    taskToken,
		WorkflowID:   env.WorkflowID,
		ActivityName: env.ActivityName,
		Input:        env.Input,
		Attempt:      env.Attempt,
		MaxAttempts:  env.MaxAttempts,
		Policy:       env.Policy,
		AckToken:     msg.AckToken,
		QueueName:    queueName,
		DequeuedAt:   time.Now(),
	}
	s.mu.Unlock()

	return &proto.PollActivityTaskResponse{
		TaskToken:    taskToken,
		ActivityName: env.ActivityName,
		Input:        env.Input,
		WorkflowId:   env.WorkflowID,
	}, nil
}

func (s *EngineServer) CompleteActivity(ctx context.Context, req *proto.CompleteActivityRequest) (*proto.CompleteActivityResponse, error) {
	s.mu.Lock()
	pending, ok := s.pendingActTasks[req.TaskToken]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("task not found: %s", req.TaskToken)
	}
	delete(s.pendingActTasks, req.TaskToken)
	s.mu.Unlock()

	if req.Error != "" {
		failed, _ := json.Marshal(map[string]any{
			"activity_name": pending.ActivityName,
			"attempt":       pending.Attempt,
			"error":         req.Error,
		})
		_ = s.engine.store.AppendEvent(ctx, store.Event{
			WorkflowID: pending.WorkflowID,
			Type:       EventTypeActivityFailed,
			Details:    failed,
		})

		if pending.Attempt < pending.MaxAttempts {
			s.scheduleActivityRetry(ctx, pending)
		}
	} else {
		done, _ := json.Marshal(map[string]any{
			"activity_name": pending.ActivityName,
			"attempt":       pending.Attempt,
			"result":        json.RawMessage(req.Result),
		})
		_ = s.engine.store.AppendEvent(ctx, store.Event{
			WorkflowID: pending.WorkflowID,
			Type:       EventTypeActivityCompleted,
			Details:    done,
		})
	}

	if err := s.queue.Ack(ctx, pending.QueueName, pending.AckToken); err != nil {
		log.Printf("ack activity task %s: %v", req.TaskToken, err)
	}

	return &proto.CompleteActivityResponse{Success: true}, nil
}

// scheduleActivityRetry computes the next-attempt backoff per the
// activity's RetryPolicy, schedules a durable timer, and arranges for
// the timer callback to re-enqueue the activity envelope with
// attempt+1.
func (s *EngineServer) scheduleActivityRetry(ctx context.Context, pending *pendingActivityTask) {
	nextAttempt := pending.Attempt + 1
	delay := pending.Policy.BackoffFor(int(nextAttempt))

	retryEvent, _ := json.Marshal(map[string]any{
		"activity_name": pending.ActivityName,
		"attempt":       nextAttempt,
		"delay":         delay.String(),
	})
	_ = s.engine.store.AppendEvent(ctx, store.Event{
		WorkflowID: pending.WorkflowID,
		Type:       EventTypeActivityRetryScheduled,
		Details:    retryEvent,
	})

	queueName := pending.QueueName
	envelope, err := json.Marshal(activityEnvelope{
		WorkflowID:   pending.WorkflowID,
		ActivityName: pending.ActivityName,
		Input:        pending.Input,
		Attempt:      nextAttempt,
		MaxAttempts:  pending.MaxAttempts,
		Policy:       pending.Policy,
	})
	if err != nil {
		log.Printf("activity retry: marshal envelope: %v", err)
		return
	}

	cb := func() {
		retryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.queue.Enqueue(retryCtx, queueName, envelope); err != nil {
			log.Printf("activity retry: re-enqueue failed: %v", err)
		}
	}

	if _, err := s.engine.timerMgr.ScheduleTimer(ctx, pending.WorkflowID, delay, cb); err != nil {
		log.Printf("activity retry: schedule timer failed: %v", err)
	}
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

	policy := DefaultRetryPolicy()
	envelope, err := json.Marshal(activityEnvelope{
		WorkflowID:   req.WorkflowId,
		ActivityName: req.ActivityName,
		Input:        req.Input,
		Attempt:      1,
		MaxAttempts:  int32(policy.MaximumAttempts),
		Policy:       policy,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal activity envelope: %w", err)
	}
	if err := s.queue.Enqueue(ctx, activityQueueName(defaultTaskQueue), envelope); err != nil {
		return nil, fmt.Errorf("enqueue activity task: %w", err)
	}

	return &proto.ScheduleActivityResponse{ActivityId: activityID}, nil
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

	s.engine.DeliverSignal(req.WorkflowId, &Signal{
		Name:  req.SignalName,
		Input: req.Input,
	})

	return &proto.SignalWorkflowResponse{Success: true}, nil
}

func (s *EngineServer) WaitForSignal(ctx context.Context, req *proto.WaitForSignalRequest) (*proto.WaitForSignalResponse, error) {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	sig, err := s.engine.WaitForSignal(ctx, req.WorkflowId, timeout)
	if err != nil {
		return nil, err
	}
	if sig == nil {
		// Timed out — return an empty response so the worker can
		// distinguish from a delivered signal and decide whether to
		// poll again.
		return &proto.WaitForSignalResponse{}, nil
	}
	return &proto.WaitForSignalResponse{
		SignalName: sig.Name,
		Input:      sig.Input,
	}, nil
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

// reclaimLoop runs in the background and surfaces messages whose
// consumer (this engine) has been holding them longer than the
// visibility timeout. In practice this happens when a worker dies
// mid-execution: the engine never gets the Complete RPC, so the entry
// stays in pendingWfTasks/pendingActTasks. After the timeout the
// reclaim cycle re-enqueues the work so another worker can pick it up.
//
// Phase 2 reclaim is conservative: it inspects only the engine's own
// in-process map. Cross-engine reclaim (when shards migrate between
// replicas) lands in Phase 4.
func (s *EngineServer) reclaimLoop() {
	defer s.reclaimWG.Done()

	ticker := time.NewTicker(defaultVisibilityTimeout / 2)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopReclaim:
			return
		case <-ticker.C:
			s.reclaimExpired()
		}
	}
}

func (s *EngineServer) reclaimExpired() {
	now := time.Now()
	type expired struct {
		taskToken string
		queueName string
		ackToken  string
		envelope  []byte
	}
	var workflows, activities []expired

	s.mu.Lock()
	for tok, p := range s.pendingWfTasks {
		if now.Sub(p.DequeuedAt) < defaultVisibilityTimeout {
			continue
		}
		env, _ := json.Marshal(workflowEnvelope{
			WorkflowID:   p.WorkflowID,
			RunID:        p.RunID,
			WorkflowName: p.WorkflowName,
		})
		workflows = append(workflows, expired{tok, p.QueueName, p.AckToken, env})
		delete(s.pendingWfTasks, tok)
	}
	for tok, p := range s.pendingActTasks {
		if now.Sub(p.DequeuedAt) < defaultVisibilityTimeout {
			continue
		}
		env, _ := json.Marshal(activityEnvelope{
			WorkflowID:   p.WorkflowID,
			ActivityName: p.ActivityName,
		})
		activities = append(activities, expired{tok, p.QueueName, p.AckToken, env})
		delete(s.pendingActTasks, tok)
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, e := range workflows {
		log.Printf("reclaim: re-enqueueing abandoned workflow task %s", e.taskToken)
		if err := s.queue.Ack(ctx, e.queueName, e.ackToken); err != nil {
			log.Printf("reclaim: ack failed: %v", err)
		}
		if err := s.queue.Enqueue(ctx, e.queueName, e.envelope); err != nil {
			log.Printf("reclaim: re-enqueue failed: %v", err)
		}
	}
	for _, e := range activities {
		log.Printf("reclaim: re-enqueueing abandoned activity task %s", e.taskToken)
		if err := s.queue.Ack(ctx, e.queueName, e.ackToken); err != nil {
			log.Printf("reclaim: ack failed: %v", err)
		}
		if err := s.queue.Enqueue(ctx, e.queueName, e.envelope); err != nil {
			log.Printf("reclaim: re-enqueue failed: %v", err)
		}
	}
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

// StartGRPCServer wires the engine and queue into a gRPC server.
func StartGRPCServer(engine *Engine, q queue.Queue, address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	engineServer := NewEngineServer(engine, q)
	proto.RegisterEngineServiceServer(grpcServer, engineServer)

	log.Printf("Engine gRPC server listening on %s", address)
	return grpcServer.Serve(lis)
}
