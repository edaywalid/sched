package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/edaywalid/sched/internal/observability"
	"github.com/edaywalid/sched/sdk"
)

func main() {
	observability.NewLogger("worker")
	shutdownTracing, err := observability.InitTracing(context.Background(), "worker")
	if err != nil {
		log.Printf("init tracing: %v (continuing without tracer)", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	engineAddress := getEnv("ENGINE_ADDRESS", "localhost:50051")
	taskQueue := getEnv("TASK_QUEUE", "default")

	log.Printf("Connecting to engine at %s", engineAddress)

	  
	client, err := sdk.NewClient(engineAddress, taskQueue)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer func() { _ = client.Close() }()

	  
	client.RegisterWorkflow("MonthlyReport", func(ctx sdk.WorkflowContext, input any) (any, error) {
		log.Println("MonthlyReport workflow started!")
		for i := range 3 {
			ctx.QueueActivity("SendEmail", fmt.Sprintf("user%d@example.com", i))
			log.Println("Sleeping...")
			ctx.Sleep(2 * time.Second)
		}
		return "Report completed", nil
	})

	client.RegisterWorkflow("HelloWorld", func(ctx sdk.WorkflowContext, input any) (any, error) {
		log.Println("HelloWorld workflow started!")
		ctx.QueueActivity("SayHello", input)
		return "Hello workflow completed", nil
	})

	  
	client.RegisterActivity("SendEmail", func(ctx sdk.ActivityContext, input any) (any, error) {
		log.Printf("Sending email to: %v\n", input)
		time.Sleep(500 * time.Millisecond)   
		return fmt.Sprintf("Email sent to %v", input), nil
	})

	client.RegisterActivity("SayHello", func(ctx sdk.ActivityContext, input any) (any, error) {
		log.Printf("Hello, %v!\n", input)
		return fmt.Sprintf("Greeted %v", input), nil
	})

	// AlwaysFail is wired to the RetryDemo workflow to exercise the
	// activity retry path. Each invocation returns an error so the
	// engine schedules backoff retries up to RetryPolicy.MaximumAttempts.
	client.RegisterActivity("AlwaysFail", func(ctx sdk.ActivityContext, input any) (any, error) {
		log.Printf("AlwaysFail invoked (input=%v)\n", input)
		return nil, fmt.Errorf("intentional failure for input %v", input)
	})

	// LongActivity demonstrates heartbeats keeping a long-running task
	// alive past the engine's visibility timeout.
	client.RegisterActivity("LongActivity", func(actCtx sdk.ActivityContext, input any) (any, error) {
		log.Printf("LongActivity start (token=%s, input=%v)", actCtx.TaskToken(), input)
		for i := 0; i < 7; i++ {
			time.Sleep(5 * time.Second)
			if cancelled, err := actCtx.Heartbeat([]byte(fmt.Sprintf(`{"tick":%d}`, i))); err != nil {
				log.Printf("LongActivity heartbeat error: %v", err)
			} else if cancelled {
				log.Printf("LongActivity cancelled")
				return nil, fmt.Errorf("cancelled")
			}
			log.Printf("LongActivity tick %d", i)
		}
		return "long activity done", nil
	})

	client.RegisterWorkflow("LongDemo", func(ctx sdk.WorkflowContext, input any) (any, error) {
		ctx.QueueActivity("LongActivity", input)
		return "long demo done", nil
	})

	client.RegisterWorkflow("RetryDemo", func(ctx sdk.WorkflowContext, input any) (any, error) {
		log.Println("RetryDemo workflow started!")
		ctx.QueueActivity("AlwaysFail", input)
		return "retry demo done", nil
	})

	// SignalDemo blocks on WaitForSignal until SignalWorkflow is
	// invoked against this workflow ID. It exercises the Phase 3.2
	// signal API end-to-end.
	client.RegisterWorkflow("SignalDemo", func(ctx sdk.WorkflowContext, input any) (any, error) {
		log.Printf("SignalDemo waiting for signal on workflow %s", ctx.GetWorkflowID())
		name, payload, err := ctx.WaitForSignal(120 * time.Second)
		if err != nil {
			return nil, err
		}
		if name == "" {
			return "timed out", nil
		}
		log.Printf("SignalDemo received signal %q payload=%v", name, payload)
		return fmt.Sprintf("got %s=%v", name, payload), nil
	})

	  
	bgCtx := context.Background()

	log.Printf("Starting worker on task queue: %s", taskQueue)

	  
	autoStartTest := getEnv("AUTO_START_TEST", "false") == "true"

	streaming := getEnv("SCHED_WORKER_STREAMING", "false") == "true"

	if autoStartTest {
		  
		log.Println("⚠️  AUTO_START_TEST=true - Starting test workflows (NOT for production!)")

		go func() {
			if err := runWorker(bgCtx, client, streaming); err != nil {
				log.Printf("Worker error: %v", err)
			}
		}()

		  
		time.Sleep(2 * time.Second)

		  
		workflowID, err := client.StartWorkflow(bgCtx, "MonthlyReport", nil)
		if err != nil {
			log.Printf("Failed to start workflow: %v", err)
		} else {
			log.Printf("✅ Started test workflow: %s", workflowID)
		}

		  
		select {}
	} else {
		log.Printf("Worker mode (streaming=%v): ready", streaming)
		if err := runWorker(bgCtx, client, streaming); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}
}

// runWorker selects between the polling and streaming worker loops
// based on the SCHED_WORKER_STREAMING env var. Streaming uses the
// bidi StreamWorkflowTasks RPC introduced in Phase 5.8; polling is
// the original PollWorkflowTask loop.
func runWorker(ctx context.Context, client *sdk.Client, streaming bool) error {
	if streaming {
		return client.StartStreamingWorker(ctx)
	}
	return client.StartWorker(ctx)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
