package engine

import (
	"context"
	"sync"
	"time"

	"github.com/edaywalid/sched/internal/store"
)

// Engine owns the durable workflow state (via Store), the durable timer
// manager, and the in-process signal dispatch table.
//
// SignalWorkflow pushes signals onto signalQueues; WaitForSignal
// drains them. If a waiter is already registered the signal is
// handed off directly; otherwise it is buffered until a waiter
// arrives. Phase 3.3 will replace the in-process queues with a
// history-driven replay model.
type Engine struct {
	store    store.Store
	timerMgr *TimerManager

	signalsMu     sync.Mutex
	signalQueues  map[string][]*Signal      // workflow_id -> buffered signals
	signalWaiters map[string][]chan *Signal // workflow_id -> waiters
}

func NewEngine(s store.Store) *Engine {
	return &Engine{
		store:         s,
		timerMgr:      NewTimerManager(s),
		signalQueues:  make(map[string][]*Signal),
		signalWaiters: make(map[string][]chan *Signal),
	}
}

// DeliverSignal hands the signal to the next waiter for the workflow
// or buffers it if none is registered.
func (e *Engine) DeliverSignal(workflowID string, sig *Signal) {
	e.signalsMu.Lock()
	defer e.signalsMu.Unlock()

	if waiters := e.signalWaiters[workflowID]; len(waiters) > 0 {
		ch := waiters[0]
		e.signalWaiters[workflowID] = waiters[1:]
		ch <- sig
		return
	}
	e.signalQueues[workflowID] = append(e.signalQueues[workflowID], sig)
}

// WaitForSignal blocks until a signal arrives for workflowID or ctx
// is cancelled. Returns (nil, nil) if the wait times out without a
// signal.
func (e *Engine) WaitForSignal(ctx context.Context, workflowID string, timeout time.Duration) (*Signal, error) {
	e.signalsMu.Lock()
	if queued := e.signalQueues[workflowID]; len(queued) > 0 {
		sig := queued[0]
		e.signalQueues[workflowID] = queued[1:]
		e.signalsMu.Unlock()
		return sig, nil
	}
	ch := make(chan *Signal, 1)
	e.signalWaiters[workflowID] = append(e.signalWaiters[workflowID], ch)
	e.signalsMu.Unlock()

	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case sig := <-ch:
		return sig, nil
	case <-timer.C:
		e.removeWaiter(workflowID, ch)
		return nil, nil
	case <-ctx.Done():
		e.removeWaiter(workflowID, ch)
		return nil, ctx.Err()
	}
}

func (e *Engine) removeWaiter(workflowID string, ch chan *Signal) {
	e.signalsMu.Lock()
	defer e.signalsMu.Unlock()
	waiters := e.signalWaiters[workflowID]
	for i, w := range waiters {
		if w == ch {
			e.signalWaiters[workflowID] = append(waiters[:i], waiters[i+1:]...)
			return
		}
	}
}

// Store returns the underlying persistence store.
func (e *Engine) Store() store.Store { return e.store }

// TimerManager exposes the engine's durable timer manager.
func (e *Engine) TimerManager() *TimerManager { return e.timerMgr }

// ListWorkflows returns all workflow executions, newest first.
func (e *Engine) ListWorkflows(ctx context.Context, filter store.ListFilter) ([]*store.Workflow, error) {
	return e.store.ListWorkflows(ctx, filter)
}

// GetWorkflow returns a single workflow execution by ID.
func (e *Engine) GetWorkflow(ctx context.Context, workflowID string) (*store.Workflow, error) {
	return e.store.GetWorkflow(ctx, workflowID)
}
