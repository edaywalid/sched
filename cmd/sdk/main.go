package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/edaywalid/sched/sdk"
)

func main() {
	  
	engineAddress := getEnv("ENGINE_ADDRESS", "localhost:50051")
	taskQueue := getEnv("TASK_QUEUE", "default")

	log.Printf("Connecting to engine at %s", engineAddress)

	  
	client, err := sdk.NewClient(engineAddress, taskQueue)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	  
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

	client.RegisterWorkflow("RetryDemo", func(ctx sdk.WorkflowContext, input any) (any, error) {
		log.Println("RetryDemo workflow started!")
		ctx.QueueActivity("AlwaysFail", input)
		return "retry demo done", nil
	})

	  
	bgCtx := context.Background()

	log.Printf("Starting worker on task queue: %s", taskQueue)

	  
	autoStartTest := getEnv("AUTO_START_TEST", "false") == "true"

	if autoStartTest {
		  
		log.Println("⚠️  AUTO_START_TEST=true - Starting test workflows (NOT for production!)")

		go func() {
			if err := client.StartWorker(bgCtx); err != nil {
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
		  
		log.Println("🔧 Worker mode: Ready to execute workflows (use client to start workflows)")
		if err := client.StartWorker(bgCtx); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
