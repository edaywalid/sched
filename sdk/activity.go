package sdk

import (
	"context"
	"fmt"

	"github.com/edaywalid/sched/proto"
)

// ActivityContext is passed to every registered activity. Activities
// can call Heartbeat to extend the engine-side visibility timeout for
// long-running work and to learn when the workflow has requested
// cancellation.
type ActivityContext interface {
	// Heartbeat tells the engine the activity is still progressing.
	// Returns true when the workflow has asked for cancellation;
	// activities should honour cancellation by returning quickly.
	Heartbeat(details []byte) (cancelRequested bool, err error)

	// TaskToken returns the engine task token for this attempt.
	// Useful for diagnostics; opaque otherwise.
	TaskToken() string
}

type ActivityFunc func(ctx ActivityContext, input any) (any, error)

// sdkActivityContext is the concrete ActivityContext the SDK hands to
// registered activities at execution time.
type sdkActivityContext struct {
	ctx       context.Context
	client    proto.EngineServiceClient
	taskToken string
}

func (a *sdkActivityContext) TaskToken() string { return a.taskToken }

func (a *sdkActivityContext) Heartbeat(details []byte) (bool, error) {
	resp, err := a.client.RecordActivityHeartbeat(a.ctx, &proto.RecordActivityHeartbeatRequest{
		TaskToken: a.taskToken,
		Details:   details,
	})
	if err != nil {
		return false, fmt.Errorf("heartbeat: %w", err)
	}
	return resp.CancelRequested, nil
}
