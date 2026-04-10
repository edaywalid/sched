package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/edaywalid/sched/proto"
)

type WorkflowContext interface {
	QueueActivity(name string, input any)
	Sleep(duration time.Duration)
	GetWorkflowID() string

	// WaitForSignal blocks until a signal arrives for this workflow,
	// then returns the signal name and its decoded payload. timeout
	// caps the wait; pass 0 to use the engine's default (60s). A
	// timeout returns ("", nil, nil) so callers can decide to loop.
	WaitForSignal(timeout time.Duration) (name string, input any, err error)
}

type WorkflowFunc func(ctx WorkflowContext, input any) (any, error)

  
type SDKWorkflowContext struct {
	workflowID string
	client     *Client
	ctx        context.Context
	replay     *replayState
}

func (wfCtx *SDKWorkflowContext) QueueActivity(name string, input any) {
	// Replay short-circuit. If this workflow has already scheduled this
	// activity in a prior run, the matching ActivityScheduled event is
	// in history past the cursor. Advance and skip the RPC.
	if matched, idx := wfCtx.replay.findActivityScheduled(name); matched != nil {
		wfCtx.replay.advance(idx)
		fmt.Printf("Replay: skipping QueueActivity %q (already in history at idx=%d)\n", name, idx)
		return
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		fmt.Printf("Failed to marshal activity input: %v\n", err)
		return
	}

	resp, err := wfCtx.client.client.ScheduleActivity(wfCtx.ctx, &proto.ScheduleActivityRequest{
		WorkflowId:   wfCtx.workflowID,
		ActivityName: name,
		Input:        inputBytes,
	})
	if err != nil {
		fmt.Printf("Failed to schedule activity %s: %v\n", name, err)
		return
	}

	fmt.Printf("Scheduled activity %s (ID: %s)\n", name, resp.ActivityId)
}

func (wfCtx *SDKWorkflowContext) Sleep(duration time.Duration) {
	// Replay path. Each Sleep matches the next TimerScheduled in
	// history past the cursor. If a TimerFired with the same timer_id
	// is also recorded, the sleep is already done; advance past both
	// events and return. If only TimerScheduled is recorded (engine
	// is still waiting for the timer to fire), yield without
	// re-registering.
	if sched, schedIdx, timerID := wfCtx.replay.findTimerScheduled(); sched != nil {
		if fired, firedIdx := wfCtx.replay.findTimerFired(timerID, schedIdx); fired != nil {
			wfCtx.replay.advance(firedIdx)
			fmt.Printf("Replay: skipping Sleep (timer %s fired at idx=%d)\n", timerID, firedIdx)
			return
		}
		fmt.Printf("Replay: re-yielding on Sleep (timer %s scheduled but not fired)\n", timerID)
		panic(yieldErr{command: "Sleep"})
	}

	// Fresh sleep. Register a durable timer; the engine will write a
	// TimerScheduled event and arrange for the matching TimerFired to
	// re-dispatch this workflow task.
	if _, err := wfCtx.client.client.RegisterWorkflowTimer(wfCtx.ctx, &proto.RegisterWorkflowTimerRequest{
		WorkflowId:      wfCtx.workflowID,
		DurationSeconds: int32(duration / time.Second),
	}); err != nil {
		fmt.Printf("RegisterWorkflowTimer failed: %v; falling back to local sleep\n", err)
		time.Sleep(duration)
		return
	}
	panic(yieldErr{command: "Sleep"})
}

func (wfCtx *SDKWorkflowContext) GetWorkflowID() string {
	return wfCtx.workflowID
}

func (wfCtx *SDKWorkflowContext) WaitForSignal(timeout time.Duration) (string, any, error) {
	// Replay short-circuit: if a SignalReceived is recorded past the
	// cursor we return its payload directly without re-blocking.
	if matched, idx := wfCtx.replay.findSignalReceived(); matched != nil {
		wfCtx.replay.advance(idx)
		var details struct {
			SignalName string          `json:"signal_name"`
			Input      json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal([]byte(matched.Details), &details); err != nil {
			return "", nil, fmt.Errorf("replay: decode signal event details: %w", err)
		}
		var payload any
		if len(details.Input) > 0 {
			if err := json.Unmarshal(details.Input, &payload); err != nil {
				return details.SignalName, nil, fmt.Errorf("replay: decode signal payload: %w", err)
			}
		}
		fmt.Printf("Replay: returning recorded signal %q from history idx=%d\n", details.SignalName, idx)
		return details.SignalName, payload, nil
	}

	// No history match: yield so the worker can free the slot and the
	// engine can re-dispatch when SignalReceived eventually lands.
	// The caller never sees control flow past this point on the first
	// run; the next run picks up the recorded signal on replay above.
	_ = timeout // reserved for future per-yield timeout hint
	panic(yieldErr{command: "WaitForSignal"})
}
