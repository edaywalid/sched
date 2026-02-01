package engine

import (
	"context"
	"sync"

	"github.com/edaywalid/sched/internal/store"
)

// Engine owns the durable workflow state (via Store) and the in-process
// signal dispatch table. Task queueing lives in EngineServer (api.go);
// Phase 2 will move task queueing to a durable Redis Streams queue.
type Engine struct {
	store     store.Store
	timerMgr  *TimerManager
	signals   map[string]chan *Signal
	signalsMu sync.RWMutex
}

func NewEngine(s store.Store) *Engine {
	return &Engine{
		store:    s,
		timerMgr: NewTimerManager(s),
		signals:  make(map[string]chan *Signal),
	}
}

// Store returns the underlying persistence store.
func (e *Engine) Store() store.Store { return e.store }

// ListWorkflows returns all workflow executions, newest first.
func (e *Engine) ListWorkflows(ctx context.Context, filter store.ListFilter) ([]*store.Workflow, error) {
	return e.store.ListWorkflows(ctx, filter)
}

// GetWorkflow returns a single workflow execution by ID.
func (e *Engine) GetWorkflow(ctx context.Context, workflowID string) (*store.Workflow, error) {
	return e.store.GetWorkflow(ctx, workflowID)
}
