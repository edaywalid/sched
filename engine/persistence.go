package engine

import "github.com/edaywalid/sched/internal/store"

// Re-export event type constants so callers inside the engine package can
// reference them without an extra import. The canonical definitions live
// in internal/store.
const (
	EventTypeWorkflowStarted   = store.EventWorkflowStarted
	EventTypeWorkflowCompleted = store.EventWorkflowCompleted
	EventTypeWorkflowFailed    = store.EventWorkflowFailed
	EventTypeActivityScheduled = store.EventActivityScheduled
	EventTypeActivityCompleted = store.EventActivityCompleted
	EventTypeActivityFailed    = store.EventActivityFailed
	EventTypeTimerScheduled    = store.EventTimerScheduled
	EventTypeTimerFired        = store.EventTimerFired
	EventTypeSignalReceived    = store.EventSignalReceived
)
