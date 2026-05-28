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
}

func (wfCtx *SDKWorkflowContext) QueueActivity(name string, input any) {
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
	time.Sleep(duration)
}

func (wfCtx *SDKWorkflowContext) GetWorkflowID() string {
	return wfCtx.workflowID
}

func (wfCtx *SDKWorkflowContext) WaitForSignal(timeout time.Duration) (string, any, error) {
	resp, err := wfCtx.client.client.WaitForSignal(wfCtx.ctx, &proto.WaitForSignalRequest{
		WorkflowId:     wfCtx.workflowID,
		TimeoutSeconds: int32(timeout / time.Second),
	})
	if err != nil {
		return "", nil, fmt.Errorf("wait for signal: %w", err)
	}
	if resp.SignalName == "" {
		// Server-side wait timed out.
		return "", nil, nil
	}
	var payload any
	if len(resp.Input) > 0 {
		if err := json.Unmarshal(resp.Input, &payload); err != nil {
			return resp.SignalName, nil, fmt.Errorf("decode signal payload: %w", err)
		}
	}
	return resp.SignalName, payload, nil
}
