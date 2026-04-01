// Package sdk is the worker-side client for the sched workflow engine.
// It registers workflow and activity functions, polls the engine for
// work, and reports results back over gRPC.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/edaywalid/sched/internal/observability"
	"github.com/edaywalid/sched/proto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

  
type Client struct {
	conn       *grpc.ClientConn
	client     proto.EngineServiceClient
	workflows  map[string]WorkflowFunc
	activities map[string]ActivityFunc
	taskQueue  string
}

  
func NewClient(engineAddress string, taskQueue string) (*Client, error) {
	conn, err := grpc.NewClient(engineAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to engine: %w", err)
	}

	client := proto.NewEngineServiceClient(conn)

	return &Client{
		conn:       conn,
		client:     client,
		workflows:  make(map[string]WorkflowFunc),
		activities: make(map[string]ActivityFunc),
		taskQueue:  taskQueue,
	}, nil
}

  
func (c *Client) Close() error {
	return c.conn.Close()
}

  
func (c *Client) RegisterWorkflow(name string, fn WorkflowFunc) {
	c.workflows[name] = fn
}

  
func (c *Client) RegisterActivity(name string, fn ActivityFunc) {
	c.activities[name] = fn
}

  
func (c *Client) StartWorkflow(ctx context.Context, workflowName string, input interface{}) (string, error) {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	resp, err := c.client.StartWorkflow(ctx, &proto.StartWorkflowRequest{
		WorkflowName: workflowName,
		Input:        inputBytes,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start workflow: %w", err)
	}

	return resp.WorkflowId, nil
}

  
func (c *Client) GetWorkflowStatus(ctx context.Context, workflowID string) (string, interface{}, error) {
	resp, err := c.client.GetWorkflowStatus(ctx, &proto.GetWorkflowStatusRequest{
		WorkflowId: workflowID,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to get workflow status: %w", err)
	}

	var result interface{}
	if len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return resp.Status, nil, fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	if resp.Error != "" {
		return resp.Status, result, fmt.Errorf("%s", resp.Error)
	}

	return resp.Status, result, nil
}

  
func (c *Client) SignalWorkflow(ctx context.Context, workflowID, signalName string, input interface{}) error {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	_, err = c.client.SignalWorkflow(ctx, &proto.SignalWorkflowRequest{
		WorkflowId: workflowID,
		SignalName: signalName,
		Input:      inputBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to signal workflow: %w", err)
	}

	return nil
}

  
func (c *Client) StartWorker(ctx context.Context) error {
	log.Printf("Starting worker, polling from task queue: %s", c.taskQueue)

	  
	errCh := make(chan error, 2)

	  
	go func() {
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
				if err := c.pollAndExecuteWorkflow(ctx); err != nil {
					log.Printf("Error polling workflow: %v", err)
					time.Sleep(time.Second)
				}
			}
		}
	}()

	  
	go func() {
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
				if err := c.pollAndExecuteActivity(ctx); err != nil {
					log.Printf("Error polling activity: %v", err)
					time.Sleep(time.Second)
				}
			}
		}
	}()

	  
	return <-errCh
}

func (c *Client) pollAndExecuteWorkflow(ctx context.Context) error {
	  
	resp, err := c.client.PollWorkflowTask(ctx, &proto.PollWorkflowTaskRequest{
		TaskQueue:      c.taskQueue,
		TimeoutSeconds: 60,
	})
	if err != nil {
		return fmt.Errorf("failed to poll workflow task: %w", err)
	}

	  
	if resp.TaskToken == "" {
		return nil
	}

	log.Printf("Received workflow task: %s (ID: %s, history=%d)", resp.WorkflowName, resp.WorkflowId, len(resp.History))

	  
	workflow, ok := c.workflows[resp.WorkflowName]
	if !ok {
		  
		_, err := c.client.CompleteWorkflowTask(ctx, &proto.CompleteWorkflowTaskRequest{
			TaskToken: resp.TaskToken,
			Error:     fmt.Sprintf("workflow not found: %s", resp.WorkflowName),
		})
		return err
	}

	var input interface{}
	if len(resp.Input) > 0 {
		if err := json.Unmarshal(resp.Input, &input); err != nil {
			_, err := c.client.CompleteWorkflowTask(ctx, &proto.CompleteWorkflowTaskRequest{
				TaskToken: resp.TaskToken,
				Error:     fmt.Sprintf("failed to unmarshal input: %v", err),
			})
			return err
		}
	}

	  
	tracer := otel.Tracer(observability.TracerName)
	wfCtx, wfSpan := tracer.Start(ctx, "workflow."+resp.WorkflowName)
	wfSpan.SetAttributes(
		attribute.String("workflow.id", resp.WorkflowId),
		attribute.String("workflow.run_id", resp.RunId),
		attribute.String("workflow.name", resp.WorkflowName),
	)

	wfRunCtx := &SDKWorkflowContext{
		workflowID: resp.WorkflowId,
		client:     c,
		ctx:        wfCtx,
		replay:     newReplayState(resp.History),
	}

	result, err := workflow(wfRunCtx, input)
	if err != nil {
		wfSpan.RecordError(err)
		wfSpan.SetStatus(codes.Error, err.Error())
	}
	wfSpan.End()

	  
	var resultBytes []byte
	var errStr string

	if result != nil {
		resultBytes, _ = json.Marshal(result)
	}
	if err != nil {
		errStr = err.Error()
	}

	_, err = c.client.CompleteWorkflowTask(ctx, &proto.CompleteWorkflowTaskRequest{
		TaskToken: resp.TaskToken,
		Result:    resultBytes,
		Error:     errStr,
	})
	if err != nil {
		return fmt.Errorf("failed to complete workflow: %w", err)
	}

	log.Printf("Completed workflow: %s", resp.WorkflowName)
	return nil
}

func (c *Client) pollAndExecuteActivity(ctx context.Context) error {
	resp, err := c.client.PollActivityTask(ctx, &proto.PollActivityTaskRequest{
		TaskQueue:      c.taskQueue,
		TimeoutSeconds: 60,
	})
	if err != nil {
		return fmt.Errorf("failed to poll activity task: %w", err)
	}

	if resp.TaskToken == "" {
		return nil
	}

	log.Printf("Received activity task: %s (workflow: %s)", resp.ActivityName, resp.WorkflowId)

	activity, ok := c.activities[resp.ActivityName]
	if !ok {
		_, err := c.client.CompleteActivity(ctx, &proto.CompleteActivityRequest{
			TaskToken: resp.TaskToken,
			Error:     fmt.Sprintf("activity not found: %s", resp.ActivityName),
		})
		return err
	}

	var input interface{}
	if len(resp.Input) > 0 {
		if err := json.Unmarshal(resp.Input, &input); err != nil {
			_, err := c.client.CompleteActivity(ctx, &proto.CompleteActivityRequest{
				TaskToken: resp.TaskToken,
				Error:     fmt.Sprintf("failed to unmarshal input: %v", err),
			})
			return err
		}
	}

	tracer := otel.Tracer(observability.TracerName)
	actSpanCtx, actSpan := tracer.Start(ctx, "activity."+resp.ActivityName)
	actSpan.SetAttributes(
		attribute.String("workflow.id", resp.WorkflowId),
		attribute.String("activity.name", resp.ActivityName),
		attribute.String("activity.task_token", resp.TaskToken),
	)

	actCtx := &sdkActivityContext{
		ctx:       actSpanCtx,
		client:    c.client,
		taskToken: resp.TaskToken,
	}
	result, err := activity(actCtx, input)
	if err != nil {
		actSpan.RecordError(err)
		actSpan.SetStatus(codes.Error, err.Error())
	}
	actSpan.End()

	var resultBytes []byte
	var errStr string

	if result != nil {
		resultBytes, _ = json.Marshal(result)
	}
	if err != nil {
		errStr = err.Error()
	}

	_, err = c.client.CompleteActivity(ctx, &proto.CompleteActivityRequest{
		TaskToken: resp.TaskToken,
		Result:    resultBytes,
		Error:     errStr,
	})
	if err != nil {
		return fmt.Errorf("failed to complete activity: %w", err)
	}

	log.Printf("Completed activity: %s", resp.ActivityName)
	return nil
}
