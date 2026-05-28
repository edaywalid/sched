// Package store defines the persistence layer for the workflow engine.
//
// Store is the storage-agnostic contract used by the engine; implementations
// include an in-memory fake for tests and a Postgres-backed implementation
// for production.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrConflict         = errors.New("conflict")
)

type WorkflowStatus string

const (
	StatusRunning   WorkflowStatus = "RUNNING"
	StatusCompleted WorkflowStatus = "COMPLETED"
	StatusFailed    WorkflowStatus = "FAILED"
	StatusTimedOut  WorkflowStatus = "TIMED_OUT"
)

const (
	EventWorkflowStarted        = "WorkflowStarted"
	EventWorkflowCompleted      = "WorkflowCompleted"
	EventWorkflowFailed         = "WorkflowFailed"
	EventActivityScheduled      = "ActivityScheduled"
	EventActivityCompleted      = "ActivityCompleted"
	EventActivityFailed         = "ActivityFailed"
	EventActivityRetryScheduled = "ActivityRetryScheduled"
	EventTimerScheduled         = "TimerScheduled"
	EventTimerFired             = "TimerFired"
	EventSignalReceived         = "SignalReceived"
)

// Workflow is the durable record of a single workflow execution.
//
// Input, Result, and Error are stored as their textual JSON / message form
// so the store can persist them without knowing the workflow's Go type.
type Workflow struct {
	WorkflowID   string
	RunID        string
	Name         string
	Status       WorkflowStatus
	Input        []byte
	Result       []byte
	Error        string
	StartTime    time.Time
	EndTime      *time.Time
	Version      int
}

// Event is a single entry in a workflow's history. Idx is monotonic per
// workflow starting at 1 and uniquely identifies the event within its
// workflow.
type Event struct {
	Idx        int64
	WorkflowID string
	Type       string
	Timestamp  time.Time
	Details    []byte
}

// Timer is a durable timer row. Phase 2 persists timers so the engine
// can recover and fire them after a restart; Phase 3 wires the firing
// callback back into workflow execution.
type Timer struct {
	TimerID    string
	WorkflowID string
	FireAt     time.Time
	Fired      bool
	CreatedAt  time.Time
}

// ListFilter narrows the set of workflows returned by ListWorkflows.
// Empty fields mean "no filter".
type ListFilter struct {
	Status WorkflowStatus
	Limit  int
}

// Store is the storage contract for workflow state and history.
//
// Implementations must be safe for concurrent use. All methods take a
// context so callers can cancel or time out operations.
type Store interface {
	CreateWorkflow(ctx context.Context, wf Workflow) error
	AppendEvent(ctx context.Context, event Event) error
	CompleteWorkflow(ctx context.Context, workflowID string, status WorkflowStatus, result []byte, errMsg string) error

	GetWorkflow(ctx context.Context, workflowID string) (*Workflow, error)
	GetHistory(ctx context.Context, workflowID string) ([]Event, error)
	ListWorkflows(ctx context.Context, filter ListFilter) ([]*Workflow, error)

	// InsertTimer persists a new pending timer.
	InsertTimer(ctx context.Context, timer Timer) error
	// FetchDueTimers claims up to limit timers whose fire_at is in the
	// past and marks them fired in the same transaction. Returns the
	// claimed timers.
	FetchDueTimers(ctx context.Context, limit int) ([]Timer, error)
	// ListPendingTimers returns all unfired timers, ordered by fire_at.
	// Used on engine startup to repopulate the timer manager.
	ListPendingTimers(ctx context.Context) ([]Timer, error)

	Close() error
}
