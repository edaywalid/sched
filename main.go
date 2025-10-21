package main

import (
	"fmt"
	"time"

	"github.com/edaywalid/sched/engine"
)

func main() {
	e := engine.NewEngine()

	e.RegisterActivity("SendEmail", func(ctx engine.ActivityContext, input any) (any, error) {
		fmt.Println("sending mail")
		return nil, nil
	})

	e.RegisterWorkflow("MonthlyReport", func(ctx engine.WorkflowContext, input any) (any, error) {
		for i := range 3 {
			ctx.QueueActivity("SendEmail", fmt.Sprintf("user%d@example.com", i))
			fmt.Println("Sleeping...")
			time.Sleep(time.Second * 2)
		}
		return nil, nil
	})

	e.StartExecutor()

	e.StartWorkflow("MonthlyReport", nil)

	time.Sleep(10 * time.Second)

}
