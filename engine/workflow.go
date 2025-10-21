package engine

import "fmt"

type WorkflowContext interface {
	QueueActivity(name string, input any)
}

type WorkflowFunc func(ctx WorkflowContext, input any) (any, error)

type DefaultWorkflowContext struct {
	engine *Engine
}

func NewDefaultWorkflowContext(engine *Engine) WorkflowContext {
	return &DefaultWorkflowContext{engine: engine}
}

func (ctx *DefaultWorkflowContext) QueueActivity(name string, input any) {
	af, ok := ctx.engine.activities[name]
	if !ok {
		fmt.Println("unknown activity : ", name)
		return
	}
	fmt.Println("running activity : ", name)
	af(nil, input)
}
