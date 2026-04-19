package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/edaywalid/sched/internal/observability"
	"github.com/edaywalid/sched/internal/store"
	"github.com/edaywalid/sched/proto"
	"github.com/edaywalid/sched/queue"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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
	engine  *Engine
	queue   queue.Queue
	metrics *observability.Metrics

	mu              sync.Mutex
	pendingWfTasks  map[string]*pendingWorkflowTask
	pendingActTasks map[string]*pendingActivityTask

	stopReclaim chan struct{}
	reclaimWG   sync.WaitGroup

	// serverCtx is cancelled by Close so long-polling Dequeue calls
	// unblock promptly during graceful shutdown instead of waiting
	// out their full TimeoutSeconds.
	serverCtx    context.Context
	serverCancel context.CancelFunc
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

func NewEngineServer(engine *Engine, q queue.Queue, m *observability.Metrics) *EngineServer {
	ctx, cancel := context.WithCancel(context.Background())
	s := &EngineServer{
		engine:          engine,
		queue:           q,
		metrics:         m,
		pendingWfTasks:  make(map[string]*pendingWorkflowTask),
		pendingActTasks: make(map[string]*pendingActivityTask),
		stopReclaim:     make(chan struct{}),
		serverCtx:       ctx,
		serverCancel:    cancel,
	}
	s.reclaimWG.Add(1)
	go s.reclaimLoop()
	return s
}

// Close stops the reclaim loop and cancels the server-lifetime context
// so long-polling Dequeue calls return promptly during shutdown. The
// underlying queue and store are owned by the caller.
func (s *EngineServer) Close() {
	if s.serverCancel != nil {
		s.serverCancel()
	}
	select {
	case <-s.stopReclaim:
	default:
		close(s.stopReclaim)
	}
	s.reclaimWG.Wait()
}

// pollCtx returns a context that fires either when the per-RPC ctx is
// done OR the server is shutting down. Used by PollWorkflowTask and
// PollActivityTask to abandon their Dequeue early on shutdown.
func (s *EngineServer) pollCtx(rpc context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(rpc)
	stop := make(chan struct{})
	go func() {
		select {
		case <-s.serverCtx.Done():
			cancel()
		case <-stop:
		}
	}()
	return ctx, func() {
		close(stop)
		cancel()
	}
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

	if req.WorkflowExecutionTimeoutSeconds > 0 {
		timeout := time.Duration(req.WorkflowExecutionTimeoutSeconds) * time.Second
		s.scheduleExecutionTimeout(ctx, workflowID, timeout)
	}

	if s.metrics != nil {
		s.metrics.WorkflowsStarted.Inc()
	}
	slog.Info("queued workflow",
		slog.String("workflow_name", req.WorkflowName),
		slog.String("workflow_id", workflowID),
		slog.String("run_id", runID))

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
	return s.dequeueWorkflowTask(ctx, req.TaskQueue, timeout)
}

// dequeueWorkflowTask is the core dequeue-and-build-response logic
// shared by the unary PollWorkflowTask and the streaming
// StreamWorkflowTasks. Returns an empty response with no error on
// timeout so callers can decide whether to retry.
func (s *EngineServer) dequeueWorkflowTask(ctx context.Context, taskQueue string, timeout time.Duration) (*proto.PollWorkflowTaskResponse, error) {
	queueName := workflowQueueName(taskQueue)
	pollStart := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.TaskPollLatency.WithLabelValues("workflow").Observe(time.Since(pollStart).Seconds())
		}
	}()

	pollC, releasePollCtx := s.pollCtx(ctx)
	defer releasePollCtx()
	msg, err := s.queue.Dequeue(pollC, queueName, timeout)
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

	history, err := s.engine.store.GetHistory(ctx, env.WorkflowID)
	if err != nil {
		slog.Warn("fetch history for dispatched workflow task",
			slog.String("workflow_id", env.WorkflowID),
			slog.Any("error", err))
		history = nil
	}
	protoHistory := make([]*proto.WorkflowEvent, 0, len(history))
	for _, ev := range history {
		protoHistory = append(protoHistory, &proto.WorkflowEvent{
			EventType: ev.Type,
			Timestamp: ev.Timestamp.Unix(),
			Details:   string(ev.Details),
		})
	}

	return &proto.PollWorkflowTaskResponse{
		TaskToken:    taskToken,
		WorkflowName: env.WorkflowName,
		Input:        env.Input,
		WorkflowId:   env.WorkflowID,
		RunId:        env.RunID,
		History:      protoHistory,
	}, nil
}

// StreamWorkflowTasks is the bidi-streaming counterpart of
// PollWorkflowTask. Lifecycle:
//
//  1. Worker sends a WorkflowStreamSubscribe message with the
//     task_queue it wants to consume from. The server reads it and
//     locks in the queue for the lifetime of the stream.
//  2. After Subscribe, the worker sends one WorkflowStreamReady per
//     task it is ready to take. The server dequeues a task per Ready
//     (blocking up to its internal poll timeout if the queue is empty)
//     and pushes the resulting PollWorkflowTaskResponse down the
//     stream.
//  3. The server-side stream lifetime is tied to the worker's stream
//     and the engine's serverCtx. Either cancelling cleanly tears
//     everything down.
//
// CompleteWorkflowTask remains a unary RPC. The streaming RPC is
// purely a delivery channel.
func (s *EngineServer) StreamWorkflowTasks(stream proto.EngineService_StreamWorkflowTasksServer) error {
	first, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("recv subscribe: %w", err)
	}
	sub := first.GetSubscribe()
	if sub == nil {
		return fmt.Errorf("first stream message must be Subscribe, got %T", first.GetBody())
	}
	taskQueue := sub.TaskQueue
	slog.Info("workflow task stream opened", slog.String("task_queue", taskQueue))

	ctx := stream.Context()
	for {
		msg, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				slog.Info("workflow task stream closed", slog.String("task_queue", taskQueue))
				return nil
			}
			return fmt.Errorf("recv next: %w", err)
		}
		if msg.GetReady() == nil {
			return fmt.Errorf("expected Ready, got %T", msg.GetBody())
		}

		// Block here until a task arrives or the stream context is
		// cancelled. The internal 30s timeout caps idle time on an
		// empty queue so the engine can periodically check
		// serverCtx for graceful shutdown.
		resp, err := s.dequeueWorkflowTask(ctx, taskQueue, 30*time.Second)
		if err != nil {
			return fmt.Errorf("dequeue: %w", err)
		}
		if resp.TaskToken == "" {
			// Idle tick. Wait for the next Ready from the worker.
			// (We could push an empty message but worker semantics
			// today are "Send Ready, expect a task" -- if we push
			// empty the worker would treat it as "no task" and
			// likely send another Ready immediately, busy-looping.)
			continue
		}
		if err := stream.Send(resp); err != nil {
			// Re-enqueue the task we just claimed so it does not
			// get lost. Acking by token is the right shape because
			// the worker never saw the task token.
			s.mu.Lock()
			if pending, ok := s.pendingWfTasks[resp.TaskToken]; ok {
				delete(s.pendingWfTasks, resp.TaskToken)
				bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = s.queue.Ack(bg, pending.QueueName, pending.AckToken)
				envelope, marshalErr := json.Marshal(workflowEnvelope{
					WorkflowID:   pending.WorkflowID,
					RunID:        pending.RunID,
					WorkflowName: pending.WorkflowName,
				})
				if marshalErr == nil {
					_ = s.queue.Enqueue(bg, pending.QueueName, envelope)
				}
				cancel()
			}
			s.mu.Unlock()
			return fmt.Errorf("stream send: %w", err)
		}
	}
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

	// Yielded: workflow function paused on a blocking command without
	// a matching history event. Don't transition to a terminal state;
	// remember the workflow so SignalReceived (and later: timer fire,
	// activity completion) can re-dispatch a new workflow task.
	if req.Yielded {
		_ = s.engine.store.AppendEvent(ctx, store.Event{
			WorkflowID: pending.WorkflowID,
			Type:       EventTypeWorkflowTaskYielded,
		})
		s.engine.MarkAwaitingDispatch(pending.WorkflowID)
		if err := s.queue.Ack(ctx, pending.QueueName, pending.AckToken); err != nil {
			slog.Warn("ack workflow task (yielded)",
				slog.String("task_token", req.TaskToken),
				slog.Any("error", err))
		}
		return &proto.CompleteWorkflowTaskResponse{Success: true}, nil
	}

	if req.Error != "" {
		failed, _ := json.Marshal(map[string]any{"error": req.Error})
		_ = s.engine.store.AppendEvent(ctx, store.Event{
			WorkflowID: pending.WorkflowID,
			Type:       EventTypeWorkflowFailed,
			Details:    failed,
		})
		_ = s.engine.store.CompleteWorkflow(ctx, pending.WorkflowID, store.StatusFailed, nil, req.Error)
		if s.metrics != nil {
			s.metrics.WorkflowsCompleted.WithLabelValues("failed").Inc()
		}
	} else {
		done, _ := json.Marshal(map[string]any{"result": json.RawMessage(req.Result)})
		_ = s.engine.store.AppendEvent(ctx, store.Event{
			WorkflowID: pending.WorkflowID,
			Type:       EventTypeWorkflowCompleted,
			Details:    done,
		})
		_ = s.engine.store.CompleteWorkflow(ctx, pending.WorkflowID, store.StatusCompleted, req.Result, "")
		if s.metrics != nil {
			s.metrics.WorkflowsCompleted.WithLabelValues("completed").Inc()
		}
	}

	if err := s.queue.Ack(ctx, pending.QueueName, pending.AckToken); err != nil {
		slog.Warn("ack workflow task failed",
			slog.String("task_token", req.TaskToken),
			slog.Any("error", err))
	}

	return &proto.CompleteWorkflowTaskResponse{Success: true}, nil
}

// RecordActivityHeartbeat resets the dequeue timestamp on the pending
// activity task so long-running activities are not reclaimed mid-flight.
// The returned cancel_requested is true when CancelWorkflow has marked
// the parent workflow; cooperative activities should return promptly.
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
	return &proto.RecordActivityHeartbeatResponse{
		Success:         true,
		CancelRequested: s.engine.IsCancelRequested(pending.WorkflowID),
	}, nil
}

// CancelWorkflow marks a running workflow for cancellation. Subsequent
// activity heartbeats see cancel_requested=true; long-running activities
// can read the flag from ActivityContext.Heartbeat and return early. The
// engine writes a WorkflowCancelRequested event immediately. If the
// workflow is still running when the workflow task next completes, the
// final state is recorded as CANCELED.
func (s *EngineServer) CancelWorkflow(ctx context.Context, req *proto.CancelWorkflowRequest) (*proto.CancelWorkflowResponse, error) {
	wf, err := s.engine.store.GetWorkflow(ctx, req.WorkflowId)
	if err != nil {
		return nil, fmt.Errorf("lookup workflow: %w", err)
	}
	if wf.Status != store.StatusRunning {
		// Cancel is a no-op against a terminal workflow.
		return &proto.CancelWorkflowResponse{Success: true}, nil
	}

	s.engine.MarkCancelRequested(req.WorkflowId)
	details, _ := json.Marshal(map[string]any{"reason": req.Reason})
	if err := s.engine.store.AppendEvent(ctx, store.Event{
		WorkflowID: req.WorkflowId,
		Type:       EventTypeWorkflowCancelRequested,
		Details:    details,
	}); err != nil {
		slog.Warn("append cancel-requested event", slog.Any("error", err))
	}
	return &proto.CancelWorkflowResponse{Success: true}, nil
}

// scheduleExecutionTimeout arms a durable timer that fires after
// timeout. When the timer fires, if the workflow is still RUNNING the
// engine writes a WorkflowTimedOut event and marks the row TIMED_OUT.
// If the workflow already reached a terminal state the timer is a
// no-op so this is safe even when the workflow finishes promptly.
func (s *EngineServer) scheduleExecutionTimeout(ctx context.Context, workflowID string, timeout time.Duration) {
	cb := func() {
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		wf, err := s.engine.store.GetWorkflow(bg, workflowID)
		if err != nil {
			slog.Warn("execution timeout lookup",
				slog.String("workflow_id", workflowID),
				slog.Any("error", err))
			return
		}
		if wf.Status != store.StatusRunning {
			return
		}
		details, _ := json.Marshal(map[string]any{
			"timeout_seconds": int(timeout.Seconds()),
		})
		_ = s.engine.store.AppendEvent(bg, store.Event{
			WorkflowID: workflowID,
			Type:       EventTypeWorkflowTimedOut,
			Details:    details,
		})
		_ = s.engine.store.CompleteWorkflow(bg, workflowID, store.StatusTimedOut, nil, "workflow execution timeout")
		if s.metrics != nil {
			s.metrics.WorkflowsCompleted.WithLabelValues("timed_out").Inc()
		}
	}
	if _, err := s.engine.timerMgr.ScheduleTimer(ctx, workflowID, timeout, cb); err != nil {
		slog.Error("schedule execution timeout",
			slog.String("workflow_id", workflowID),
			slog.Any("error", err))
	}
}

func (s *EngineServer) PollActivityTask(ctx context.Context, req *proto.PollActivityTaskRequest) (*proto.PollActivityTaskResponse, error) {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	queueName := activityQueueName(req.TaskQueue)
	pollStart := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.TaskPollLatency.WithLabelValues("activity").Observe(time.Since(pollStart).Seconds())
		}
	}()

	pollC, releasePollCtx := s.pollCtx(ctx)
	defer releasePollCtx()
	msg, err := s.queue.Dequeue(pollC, queueName, timeout)
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

	duration := time.Since(pending.DequeuedAt).Seconds()
	if s.metrics != nil {
		s.metrics.ActivityDuration.Observe(duration)
	}

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
		if s.metrics != nil {
			s.metrics.ActivitiesExecuted.WithLabelValues("failed").Inc()
		}

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
		if s.metrics != nil {
			s.metrics.ActivitiesExecuted.WithLabelValues("completed").Inc()
		}
	}

	if err := s.queue.Ack(ctx, pending.QueueName, pending.AckToken); err != nil {
		slog.Warn("ack activity task failed",
			slog.String("task_token", req.TaskToken),
			slog.Any("error", err))
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

	if s.metrics != nil {
		s.metrics.ActivityRetries.Inc()
	}
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
		slog.Error("activity retry marshal envelope",
			slog.String("workflow_id", pending.WorkflowID),
			slog.Any("error", err))
		return
	}

	cb := func() {
		retryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.queue.Enqueue(retryCtx, queueName, envelope); err != nil {
			slog.Error("activity retry re-enqueue failed",
				slog.String("workflow_id", pending.WorkflowID),
				slog.Any("error", err))
		}
	}

	if _, err := s.engine.timerMgr.ScheduleTimer(ctx, pending.WorkflowID, delay, cb); err != nil {
		slog.Error("activity retry schedule timer",
			slog.String("workflow_id", pending.WorkflowID),
			slog.Any("error", err))
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

	// If a yielded workflow is waiting for this signal, enqueue a new
	// workflow task. The replayed function will see SignalReceived in
	// history and continue past WaitForSignal.
	if s.engine.ClaimAwaitingDispatch(req.WorkflowId) {
		if err := s.redispatchWorkflowTask(ctx, req.WorkflowId); err != nil {
			slog.Warn("re-dispatch on signal",
				slog.String("workflow_id", req.WorkflowId),
				slog.Any("error", err))
		}
	}

	return &proto.SignalWorkflowResponse{Success: true}, nil
}

// RegisterWorkflowTimer is the engine-side endpoint for SDK Sleep.
// The timer is persisted via TimerManager so it survives engine
// restarts (Phase 2). On fire the callback marks the workflow for
// re-dispatch (if it yielded) and enqueues a fresh workflow task so
// the replayed workflow function picks up TimerFired in history.
func (s *EngineServer) RegisterWorkflowTimer(ctx context.Context, req *proto.RegisterWorkflowTimerRequest) (*proto.RegisterWorkflowTimerResponse, error) {
	duration := time.Duration(req.DurationSeconds) * time.Second
	if duration <= 0 {
		return nil, fmt.Errorf("duration_seconds must be positive, got %d", req.DurationSeconds)
	}

	workflowID := req.WorkflowId
	cb := func() {
		fireCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if s.engine.ClaimAwaitingDispatch(workflowID) {
			if err := s.redispatchWorkflowTask(fireCtx, workflowID); err != nil {
				slog.Warn("re-dispatch on timer fire",
					slog.String("workflow_id", workflowID),
					slog.Any("error", err))
			}
		}
	}
	timerID, err := s.engine.timerMgr.ScheduleTimer(ctx, workflowID, duration, cb)
	if err != nil {
		return nil, fmt.Errorf("schedule timer: %w", err)
	}
	return &proto.RegisterWorkflowTimerResponse{TimerId: timerID}, nil
}

// redispatchWorkflowTask enqueues a fresh workflow envelope for a
// workflow that has previously yielded. The worker that picks it up
// re-runs the workflow function against the now-larger history.
func (s *EngineServer) redispatchWorkflowTask(ctx context.Context, workflowID string) error {
	wf, err := s.engine.store.GetWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("lookup workflow for re-dispatch: %w", err)
	}
	envelope, err := json.Marshal(workflowEnvelope{
		WorkflowID:   wf.WorkflowID,
		RunID:        wf.RunID,
		WorkflowName: wf.Name,
		Input:        wf.Input,
	})
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	return s.queue.Enqueue(ctx, workflowQueueName(defaultTaskQueue), envelope)
}

func (s *EngineServer) WaitForSignal(ctx context.Context, req *proto.WaitForSignalRequest) (*proto.WaitForSignalResponse, error) {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	waitCtx, release := s.pollCtx(ctx)
	defer release()
	sig, err := s.engine.WaitForSignal(waitCtx, req.WorkflowId, timeout)
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
		slog.Warn("reclaim re-enqueueing abandoned workflow task",
			slog.String("task_token", e.taskToken))
		if err := s.queue.Ack(ctx, e.queueName, e.ackToken); err != nil {
			slog.Error("reclaim ack failed", slog.Any("error", err))
		}
		if err := s.queue.Enqueue(ctx, e.queueName, e.envelope); err != nil {
			slog.Error("reclaim re-enqueue failed", slog.Any("error", err))
		}
	}
	for _, e := range activities {
		slog.Warn("reclaim re-enqueueing abandoned activity task",
			slog.String("task_token", e.taskToken))
		if err := s.queue.Ack(ctx, e.queueName, e.ackToken); err != nil {
			slog.Error("reclaim ack failed", slog.Any("error", err))
		}
		if err := s.queue.Enqueue(ctx, e.queueName, e.envelope); err != nil {
			slog.Error("reclaim re-enqueue failed", slog.Any("error", err))
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

// GRPCServer bundles the gRPC server and the EngineServer it hosts so
// the binary main can drive a graceful shutdown. Serve runs until the
// listener returns an error (e.g. GracefulStop is called from another
// goroutine).
type GRPCServer struct {
	grpcServer   *grpc.Server
	engineServer *EngineServer
	listener     net.Listener
}

// NewGRPCServer constructs the gRPC server without starting it.
// metrics may be nil in tests; in that case counters are skipped.
func NewGRPCServer(engine *Engine, q queue.Queue, m *observability.Metrics, address string) (*GRPCServer, error) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}
	gs := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	engineServer := NewEngineServer(engine, q, m)
	proto.RegisterEngineServiceServer(gs, engineServer)
	return &GRPCServer{grpcServer: gs, engineServer: engineServer, listener: lis}, nil
}

// Serve blocks until the listener errors or GracefulStop is called.
func (s *GRPCServer) Serve() error {
	slog.Info("engine gRPC server listening", slog.String("addr", s.listener.Addr().String()))
	return s.grpcServer.Serve(s.listener)
}

// Stop runs a graceful shutdown:
//
//  1. Cancel the EngineServer's serverCtx so any long-polling Dequeue
//     calls return immediately. Without this step GracefulStop would
//     sit waiting on workers' 60s long-polls for up to the full
//     visibility window.
//  2. Call GracefulStop on the gRPC server. With pollCtx already
//     cancelled the in-flight RPCs return ASAP and GracefulStop
//     finishes quickly.
//  3. If the supplied context expires first, force a hard Stop.
//  4. Wait for the reclaim loop to finish via EngineServer.Close.
func (s *GRPCServer) Stop(ctx context.Context) {
	if s.engineServer.serverCancel != nil {
		s.engineServer.serverCancel()
	}
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		slog.Info("gRPC server graceful stop complete")
	case <-ctx.Done():
		slog.Warn("gRPC graceful stop deadline expired, forcing", slog.Any("error", ctx.Err()))
		s.grpcServer.Stop()
	}
	s.engineServer.Close()
}

// StartGRPCServer is the legacy entry point that hangs around for
// tests and existing callers. It builds the server and blocks in
// Serve until the listener errors. New code should use NewGRPCServer
// to retain the Stop handle.
func StartGRPCServer(engine *Engine, q queue.Queue, m *observability.Metrics, address string) error {
	gs, err := NewGRPCServer(engine, q, m, address)
	if err != nil {
		return err
	}
	return gs.Serve()
}
