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
