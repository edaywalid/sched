package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/edaywalid/sched/queue"
)

type Engine struct {
	workflowTasks chan WorkflowTask
	queue         queue.Queue
	persistence   *PersistenceLayer
	timerMgr      *TimerManager
	signals       map[string]chan *Signal
	signalsMu     sync.RWMutex
}

func NewEngine() *Engine {
	persistence := NewPersistenceLayer()
	return &Engine{
		workflowTasks: make(chan WorkflowTask, 100),
		queue:         queue.NewInMemoryQueue(), // Default to in-memory
		persistence:   persistence,
		timerMgr:      NewTimerManager(persistence),
		signals:       make(map[string]chan *Signal),
	}
}

func NewEngineWithRedis(redisAddr string) (*Engine, error) {
	q, err := queue.NewRedisQueue(redisAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis queue: %w", err)
	}

	persistence := NewPersistenceLayer()
	return &Engine{
		workflowTasks: make(chan WorkflowTask, 100),
		queue:         q,
		persistence:   persistence,
		timerMgr:      NewTimerManager(persistence),
		signals:       make(map[string]chan *Signal),
	}, nil
}

// StartWorkflow queues a workflow task for workers to pick up
func (e *Engine) StartWorkflow(name string, input any) error {
	workflowID := fmt.Sprintf("wf-%d", time.Now().UnixNano())

	e.persistence.CreateWorkflowExecution(workflowID, name, input)

	e.workflowTasks <- WorkflowTask{
		WorkflowID: workflowID,
		Name:       name,
		Input:      input,
	}

	return nil
}

func (e *Engine) GetAllWorkflowExecutions() []*WorkflowExecution {
	return e.persistence.GetAllExecutions()
}

func (e *Engine) GetWorkflowExecution(workflowID string) (*WorkflowExecution, error) {
	return e.persistence.GetWorkflowExecution(workflowID)
}
