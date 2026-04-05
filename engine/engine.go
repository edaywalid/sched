// Package engine owns the workflow orchestrator: durable state via
// store.Store, task dispatch via queue.Queue, durable timers, and the
// gRPC handlers that workers and clients talk to.
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
//
// cancelRequested is a per-workflow flag set by CancelWorkflow and
// surfaced through RecordActivityHeartbeat so long-running activities
// can bail out cooperatively.
type Engine struct {
	store    store.Store
	timerMgr *TimerManager

	signalsMu     sync.Mutex
	signalQueues  map[string][]*Signal      // workflow_id -> buffered signals
	signalWaiters map[string][]chan *Signal // workflow_id -> waiters

	cancelMu        sync.RWMutex
	cancelRequested map[string]struct{} // workflow_id set

	// awaitingMu guards the set of workflows that have yielded and are
	// waiting for an external event (currently just SignalReceived).
	// When such an event lands the engine re-enqueues a workflow task
	// for the workflow and clears the bit.
	awaitingMu       sync.Mutex
	awaitingDispatch map[string]struct{}
}

func NewEngine(s store.Store) *Engine {
	return &Engine{
		store:            s,
		timerMgr:         NewTimerManager(s),
		signalQueues:     make(map[string][]*Signal),
		signalWaiters:    make(map[string][]chan *Signal),
		cancelRequested:  make(map[string]struct{}),
		awaitingDispatch: make(map[string]struct{}),
	}
}

// MarkAwaitingDispatch records that a workflow has yielded and should
// have a new workflow task enqueued when relevant history events
// arrive. Idempotent.
func (e *Engine) MarkAwaitingDispatch(workflowID string) {
	e.awaitingMu.Lock()
	e.awaitingDispatch[workflowID] = struct{}{}
	e.awaitingMu.Unlock()
}

// ClaimAwaitingDispatch atomically reports whether the workflow was
// waiting for a re-dispatch and removes it from the set if so. Used by
// the signal/activity completion paths to decide whether to enqueue a
// new workflow task.
func (e *Engine) ClaimAwaitingDispatch(workflowID string) bool {
	e.awaitingMu.Lock()
	defer e.awaitingMu.Unlock()
	if _, ok := e.awaitingDispatch[workflowID]; ok {
		delete(e.awaitingDispatch, workflowID)
		return true
	}
	return false
}

// MarkCancelRequested records that a workflow has been asked to cancel.
// Subsequent RecordActivityHeartbeat calls for any running activity in
// this workflow return cancel_requested=true so the worker can bail.
func (e *Engine) MarkCancelRequested(workflowID string) {
	e.cancelMu.Lock()
	defer e.cancelMu.Unlock()
	e.cancelRequested[workflowID] = struct{}{}
}

// IsCancelRequested reports whether MarkCancelRequested has been called
// for this workflow.
func (e *Engine) IsCancelRequested(workflowID string) bool {
	e.cancelMu.RLock()
	defer e.cancelMu.RUnlock()
	_, ok := e.cancelRequested[workflowID]
	return ok
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
