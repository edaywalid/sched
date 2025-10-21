package engine

import "fmt"

type Engine struct {
	workflows  map[string]WorkflowFunc
	activities map[string]ActivityFunc
	queues     chan WorkflowTask
}

func NewEngine() Engine {
	return Engine{
		workflows:  make(map[string]WorkflowFunc),
		activities: make(map[string]ActivityFunc),
		queues:     make(chan WorkflowTask, 100),
	}
}

func (e *Engine) RegisterWorkflow(name string, wf WorkflowFunc) {
	e.workflows[name] = wf
}

func (e *Engine) RegisterActivity(name string, af ActivityFunc) {
	e.activities[name] = af
}

func (e *Engine) StartWorkflow(name string, input any) error {
	_, ok := e.workflows[name]
	if !ok {
		return fmt.Errorf("worflow not found")
	}
	e.queues <- WorkflowTask{Name: name, Input: input}
	return nil
}

func (e *Engine) StartExecutor() {
	go func() {
		for wt := range e.queues {
			wf := e.workflows[wt.Name]
			fmt.Println("starting workflow : ", wt.Name)
			wf(NewDefaultWorkflowContext(e), wt.Input)
		}
	}()
}
