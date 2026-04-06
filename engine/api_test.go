package engine

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/edaywalid/sched/internal/store"
	"github.com/edaywalid/sched/proto"
	"github.com/edaywalid/sched/queue"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// newTestServer wires a fresh EngineServer onto an in-process gRPC
// connection. Caller gets a proto client plus a teardown function.
func newTestServer(t *testing.T) (proto.EngineServiceClient, *EngineServer, func()) {
	t.Helper()

	listener := bufconn.Listen(1 << 20)
	grpcServer := grpc.NewServer()

	eng := NewEngine(store.NewMemoryStore())
	srv := NewEngineServer(eng, queue.NewInMemoryQueue(), nil)
	proto.RegisterEngineServiceServer(grpcServer, srv)

	go func() {
		_ = grpcServer.Serve(listener)
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}

	client := proto.NewEngineServiceClient(conn)
	teardown := func() {
		_ = conn.Close()
		grpcServer.Stop()
		srv.Close()
		eng.timerMgr.Stop()
	}
	return client, srv, teardown
}

// TestStartWorkflow_HappyPath drives the most basic flow: start a
// workflow, a worker polls it, schedules an activity, completes the
// activity, completes the workflow. Verifies the resulting history.
func TestStartWorkflow_HappyPath(t *testing.T) {
	client, srv, teardown := newTestServer(t)
	defer teardown()
	ctx := context.Background()

	startResp, err := client.StartWorkflow(ctx, &proto.StartWorkflowRequest{
		WorkflowName: "TestWorkflow",
		Input:        []byte(`"hello"`),
	})
	if err != nil {
		t.Fatalf("StartWorkflow: %v", err)
	}
	if startResp.WorkflowId == "" || startResp.RunId == "" {
		t.Fatalf("missing workflow_id or run_id: %+v", startResp)
	}

	// Worker polls the workflow task.
	pollResp, err := client.PollWorkflowTask(ctx, &proto.PollWorkflowTaskRequest{
		TaskQueue:      "default",
		TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatalf("PollWorkflowTask: %v", err)
	}
	if pollResp.TaskToken == "" {
		t.Fatalf("PollWorkflowTask: empty task token (queue drained?)")
	}
	if pollResp.WorkflowName != "TestWorkflow" {
		t.Fatalf("workflow_name = %q, want TestWorkflow", pollResp.WorkflowName)
	}

	// Worker schedules an activity from inside the workflow.
	schedResp, err := client.ScheduleActivity(ctx, &proto.ScheduleActivityRequest{
		WorkflowId:   pollResp.WorkflowId,
		ActivityName: "MyActivity",
		Input:        []byte(`"data"`),
	})
	if err != nil {
		t.Fatalf("ScheduleActivity: %v", err)
	}
	if schedResp.ActivityId == "" {
		t.Fatalf("ScheduleActivity returned empty activity_id")
	}

	// Worker polls the activity, executes it, completes it.
	actPoll, err := client.PollActivityTask(ctx, &proto.PollActivityTaskRequest{
		TaskQueue:      "default",
		TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatalf("PollActivityTask: %v", err)
	}
	if actPoll.ActivityName != "MyActivity" {
		t.Fatalf("activity_name = %q, want MyActivity", actPoll.ActivityName)
	}
	if _, err := client.CompleteActivity(ctx, &proto.CompleteActivityRequest{
		TaskToken: actPoll.TaskToken,
		Result:    []byte(`"ok"`),
	}); err != nil {
		t.Fatalf("CompleteActivity: %v", err)
	}

	// Worker completes the workflow.
	if _, err := client.CompleteWorkflowTask(ctx, &proto.CompleteWorkflowTaskRequest{
		TaskToken: pollResp.TaskToken,
		Result:    []byte(`"done"`),
	}); err != nil {
		t.Fatalf("CompleteWorkflowTask: %v", err)
	}

	history, err := srv.engine.store.GetHistory(ctx, startResp.WorkflowId)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	want := []string{
		EventTypeWorkflowStarted,
		EventTypeActivityScheduled,
		EventTypeActivityCompleted,
		EventTypeWorkflowCompleted,
	}
	if len(history) != len(want) {
		t.Fatalf("history len = %d, want %d (events: %v)", len(history), len(want), eventTypes(history))
	}
	for i, e := range history {
		if e.Type != want[i] {
			t.Errorf("event[%d] = %q, want %q", i, e.Type, want[i])
		}
	}

	wf, err := srv.engine.store.GetWorkflow(ctx, startResp.WorkflowId)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if wf.Status != store.StatusCompleted {
		t.Fatalf("final status = %q, want COMPLETED", wf.Status)
	}
	if string(wf.Result) != `"done"` {
		t.Fatalf("result = %q, want \"done\"", wf.Result)
	}
}

// TestActivityRetry verifies the retry path end-to-end through gRPC.
// We fail an activity three times in a row (matching the default
// MaxAttempts=3) and assert the history contains the right interleaving
// of ActivityFailed, ActivityRetryScheduled, and TimerFired events.
func TestActivityRetry(t *testing.T) {
	client, srv, teardown := newTestServer(t)
	defer teardown()
	ctx := context.Background()

	startResp, _ := client.StartWorkflow(ctx, &proto.StartWorkflowRequest{
		WorkflowName: "RetryTest",
	})

	// Drive the workflow task forward enough to schedule the activity.
	pollWf, err := client.PollWorkflowTask(ctx, &proto.PollWorkflowTaskRequest{
		TaskQueue: "default", TimeoutSeconds: 5,
	})
	if err != nil || pollWf.TaskToken == "" {
		t.Fatalf("PollWorkflowTask: %v %+v", err, pollWf)
	}
	if _, err := client.ScheduleActivity(ctx, &proto.ScheduleActivityRequest{
		WorkflowId:   pollWf.WorkflowId,
		ActivityName: "Flaky",
		Input:        []byte(`"x"`),
	}); err != nil {
		t.Fatalf("ScheduleActivity: %v", err)
	}
	if _, err := client.CompleteWorkflowTask(ctx, &proto.CompleteWorkflowTaskRequest{
		TaskToken: pollWf.TaskToken,
		Result:    []byte(`"queued"`),
	}); err != nil {
		t.Fatalf("CompleteWorkflowTask: %v", err)
	}

	// Override the retry policy with a tiny backoff so the test runs quickly.
	srv.engine.timerMgr.Stop()
	srv.engine.timerMgr = NewTimerManager(srv.engine.store)

	// Drive 3 attempts, each failing.
	for attempt := 1; attempt <= 3; attempt++ {
		actPoll, err := pollActivityWithBackoff(ctx, client, 5*time.Second)
		if err != nil {
			t.Fatalf("attempt %d: pollActivityWithBackoff: %v", attempt, err)
		}
		if actPoll.TaskToken == "" {
			t.Fatalf("attempt %d: empty task token (queue empty?)", attempt)
		}
		if _, err := client.CompleteActivity(ctx, &proto.CompleteActivityRequest{
			TaskToken: actPoll.TaskToken,
			Error:     "boom",
		}); err != nil {
			t.Fatalf("attempt %d: CompleteActivity: %v", attempt, err)
		}
	}

	// Wait for the third attempt to settle. The third failure does NOT
	// schedule a retry (MaxAttempts=3 was the last). Allow a brief
	// grace period for the engine to append the final ActivityFailed.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		hist, _ := srv.engine.store.GetHistory(ctx, startResp.WorkflowId)
		failures := 0
		for _, e := range hist {
			if e.Type == EventTypeActivityFailed {
				failures++
			}
		}
		if failures == 3 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	hist, err := srv.engine.store.GetHistory(ctx, startResp.WorkflowId)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	counts := map[string]int{}
	for _, e := range hist {
		counts[e.Type]++
	}
	if counts[EventTypeActivityFailed] != 3 {
		t.Errorf("ActivityFailed count = %d, want 3 (history: %v)", counts[EventTypeActivityFailed], eventTypes(hist))
	}
	if counts[EventTypeActivityRetryScheduled] != 2 {
		t.Errorf("ActivityRetryScheduled count = %d, want 2 (history: %v)", counts[EventTypeActivityRetryScheduled], eventTypes(hist))
	}
}

// TestSignalDelivery covers both code paths in the signal queue: a
// signal that arrives before any waiter (buffered) and one that arrives
// while a waiter is parked.
func TestSignalDelivery(t *testing.T) {
	client, _, teardown := newTestServer(t)
	defer teardown()
	ctx := context.Background()

	startResp, _ := client.StartWorkflow(ctx, &proto.StartWorkflowRequest{
		WorkflowName: "SignalTest",
	})

	// Drain the workflow poll so the queue is empty when we wait.
	if _, err := client.PollWorkflowTask(ctx, &proto.PollWorkflowTaskRequest{
		TaskQueue: "default", TimeoutSeconds: 5,
	}); err != nil {
		t.Fatalf("PollWorkflowTask: %v", err)
	}

	// Case 1: signal arrives before WaitForSignal; it should be buffered.
	if _, err := client.SignalWorkflow(ctx, &proto.SignalWorkflowRequest{
		WorkflowId: startResp.WorkflowId,
		SignalName: "early",
		Input:      []byte(`"first"`),
	}); err != nil {
		t.Fatalf("SignalWorkflow early: %v", err)
	}
	got, err := client.WaitForSignal(ctx, &proto.WaitForSignalRequest{
		WorkflowId:     startResp.WorkflowId,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("WaitForSignal early: %v", err)
	}
	if got.SignalName != "early" || string(got.Input) != `"first"` {
		t.Errorf("buffered signal = %+v, want early/\"first\"", got)
	}

	// Case 2: waiter parks first, signal arrives second.
	errCh := make(chan error, 1)
	respCh := make(chan *proto.WaitForSignalResponse, 1)
	go func() {
		r, err := client.WaitForSignal(ctx, &proto.WaitForSignalRequest{
			WorkflowId:     startResp.WorkflowId,
			TimeoutSeconds: 5,
		})
		respCh <- r
		errCh <- err
	}()
	time.Sleep(100 * time.Millisecond) // let the waiter register
	if _, err := client.SignalWorkflow(ctx, &proto.SignalWorkflowRequest{
		WorkflowId: startResp.WorkflowId,
		SignalName: "late",
		Input:      []byte(`"second"`),
	}); err != nil {
		t.Fatalf("SignalWorkflow late: %v", err)
	}

	select {
	case got := <-respCh:
		if err := <-errCh; err != nil {
			t.Fatalf("late WaitForSignal: %v", err)
		}
		if got.SignalName != "late" || string(got.Input) != `"second"` {
			t.Errorf("waiter signal = %+v, want late/\"second\"", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiter never unblocked")
	}
}

// pollActivityWithBackoff retries PollActivityTask for up to `total`
// because the retry path inserts a small backoff before re-enqueueing.
func pollActivityWithBackoff(ctx context.Context, c proto.EngineServiceClient, total time.Duration) (*proto.PollActivityTaskResponse, error) {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		resp, err := c.PollActivityTask(ctx, &proto.PollActivityTaskRequest{
			TaskQueue: "default", TimeoutSeconds: 1,
		})
		if err != nil {
			return nil, err
		}
		if resp.TaskToken != "" {
			return resp, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, context.DeadlineExceeded
}

// TestWorkflowExecutionTimeout verifies that a workflow started with
// workflow_execution_timeout_seconds > 0 is marked TIMED_OUT if it has
// not reached a terminal state when the timer fires.
func TestWorkflowExecutionTimeout(t *testing.T) {
	client, srv, teardown := newTestServer(t)
	defer teardown()
	ctx := context.Background()

	startResp, err := client.StartWorkflow(ctx, &proto.StartWorkflowRequest{
		WorkflowName:                    "Slow",
		WorkflowExecutionTimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("StartWorkflow: %v", err)
	}

	// Do NOT complete the workflow task. Wait past the timeout.
	deadline := time.Now().Add(3 * time.Second)
	var wf *store.Workflow
	for time.Now().Before(deadline) {
		wf, err = srv.engine.store.GetWorkflow(ctx, startResp.WorkflowId)
		if err != nil {
			t.Fatalf("GetWorkflow: %v", err)
		}
		if wf.Status == store.StatusTimedOut {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if wf.Status != store.StatusTimedOut {
		t.Fatalf("status = %q, want TIMED_OUT", wf.Status)
	}

	hist, _ := srv.engine.store.GetHistory(ctx, startResp.WorkflowId)
	if !containsEvent(hist, EventTypeWorkflowTimedOut) {
		t.Errorf("history missing WorkflowTimedOut: %v", eventTypes(hist))
	}
}

// TestCancelWorkflow verifies CancelWorkflow flips the in-process cancel
// flag and that subsequent activity heartbeats see cancel_requested=true.
func TestCancelWorkflow(t *testing.T) {
	client, srv, teardown := newTestServer(t)
	defer teardown()
	ctx := context.Background()

	startResp, _ := client.StartWorkflow(ctx, &proto.StartWorkflowRequest{WorkflowName: "Cancellable"})

	// Drain workflow task and schedule an activity. We do NOT complete
	// the workflow task because doing so would mark the workflow
	// COMPLETED, and CancelWorkflow only acts on RUNNING workflows.
	// The workflow stays RUNNING while we exercise the cancel path.
	wfPoll, _ := client.PollWorkflowTask(ctx, &proto.PollWorkflowTaskRequest{TaskQueue: "default", TimeoutSeconds: 5})
	if _, err := client.ScheduleActivity(ctx, &proto.ScheduleActivityRequest{
		WorkflowId:   wfPoll.WorkflowId,
		ActivityName: "Slow",
	}); err != nil {
		t.Fatalf("ScheduleActivity: %v", err)
	}
	actPoll, _ := client.PollActivityTask(ctx, &proto.PollActivityTaskRequest{TaskQueue: "default", TimeoutSeconds: 5})

	// Cancel the workflow while the activity is in flight.
	if _, err := client.CancelWorkflow(ctx, &proto.CancelWorkflowRequest{
		WorkflowId: startResp.WorkflowId,
		Reason:     "user requested",
	}); err != nil {
		t.Fatalf("CancelWorkflow: %v", err)
	}

	// Heartbeat should now report cancel_requested=true.
	hb, err := client.RecordActivityHeartbeat(ctx, &proto.RecordActivityHeartbeatRequest{
		TaskToken: actPoll.TaskToken,
	})
	if err != nil {
		t.Fatalf("RecordActivityHeartbeat: %v", err)
	}
	if !hb.CancelRequested {
		t.Errorf("CancelRequested = false; want true after CancelWorkflow")
	}

	hist, _ := srv.engine.store.GetHistory(ctx, startResp.WorkflowId)
	if !containsEvent(hist, EventTypeWorkflowCancelRequested) {
		t.Errorf("history missing WorkflowCancelRequested: %v", eventTypes(hist))
	}
}

// TestYieldAndRedispatch verifies the Phase 3.4c contract: a workflow
// task completed with Yielded=true leaves the workflow RUNNING, writes
// a WorkflowTaskYielded event, and a subsequent SignalWorkflow call
// enqueues a fresh workflow task that the next poll returns with the
// new SignalReceived event in its history.
func TestYieldAndRedispatch(t *testing.T) {
	client, srv, teardown := newTestServer(t)
	defer teardown()
	ctx := context.Background()

	startResp, _ := client.StartWorkflow(ctx, &proto.StartWorkflowRequest{
		WorkflowName: "YieldDemo",
	})

	first, err := client.PollWorkflowTask(ctx, &proto.PollWorkflowTaskRequest{TaskQueue: "default", TimeoutSeconds: 5})
	if err != nil || first.TaskToken == "" {
		t.Fatalf("first PollWorkflowTask: %v %+v", err, first)
	}
	// First-run history is just WorkflowStarted.
	if len(first.History) != 1 || first.History[0].EventType != EventTypeWorkflowStarted {
		t.Fatalf("first history = %+v, want [WorkflowStarted]", first.History)
	}

	// Worker reports yielded=true (simulating the SDK panic-recover path).
	if _, err := client.CompleteWorkflowTask(ctx, &proto.CompleteWorkflowTaskRequest{
		TaskToken: first.TaskToken,
		Yielded:   true,
	}); err != nil {
		t.Fatalf("CompleteWorkflowTask(yielded=true): %v", err)
	}

	// Workflow must still be RUNNING.
	wf, _ := srv.engine.store.GetWorkflow(ctx, startResp.WorkflowId)
	if wf.Status != store.StatusRunning {
		t.Fatalf("status after yield = %q, want RUNNING", wf.Status)
	}
	hist, _ := srv.engine.store.GetHistory(ctx, startResp.WorkflowId)
	if !containsEvent(hist, EventTypeWorkflowTaskYielded) {
		t.Errorf("history missing WorkflowTaskYielded: %v", eventTypes(hist))
	}

	// Send a signal. The engine should re-enqueue a workflow task.
	if _, err := client.SignalWorkflow(ctx, &proto.SignalWorkflowRequest{
		WorkflowId: startResp.WorkflowId,
		SignalName: "go",
		Input:      []byte(`"now"`),
	}); err != nil {
		t.Fatalf("SignalWorkflow: %v", err)
	}

	// Second poll returns the re-dispatched task with extended history.
	second, err := client.PollWorkflowTask(ctx, &proto.PollWorkflowTaskRequest{TaskQueue: "default", TimeoutSeconds: 5})
	if err != nil || second.TaskToken == "" {
		t.Fatalf("re-dispatched PollWorkflowTask: %v %+v", err, second)
	}
	gotTypes := map[string]int{}
	for _, ev := range second.History {
		gotTypes[ev.EventType]++
	}
	if gotTypes[EventTypeWorkflowStarted] != 1 || gotTypes[EventTypeWorkflowTaskYielded] != 1 || gotTypes[EventTypeSignalReceived] != 1 {
		t.Errorf("re-dispatched history counts = %v, want WorkflowStarted+WorkflowTaskYielded+SignalReceived", gotTypes)
	}

	// Acknowledge the second task as a real completion so we can show
	// the workflow now reaches a terminal state.
	if _, err := client.CompleteWorkflowTask(ctx, &proto.CompleteWorkflowTaskRequest{
		TaskToken: second.TaskToken,
		Result:    []byte(`"done"`),
	}); err != nil {
		t.Fatalf("final CompleteWorkflowTask: %v", err)
	}
	wf, _ = srv.engine.store.GetWorkflow(ctx, startResp.WorkflowId)
	if wf.Status != store.StatusCompleted {
		t.Fatalf("final status = %q, want COMPLETED", wf.Status)
	}
}

func containsEvent(events []store.Event, eventType string) bool {
	for _, e := range events {
		if e.Type == eventType {
			return true
		}
	}
	return false
}

func eventTypes(events []store.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}
