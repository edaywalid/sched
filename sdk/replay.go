package sdk

import (
	"encoding/json"

	"github.com/edaywalid/sched/proto"
)

// replayState tracks the workflow's position in its own durable history
// while the workflow function is running.
//
// On each PollWorkflowTask the engine ships the full event history for
// the workflow. The worker invokes the registered workflow function;
// each time the function issues a "command" (QueueActivity, Sleep,
// WaitForSignal) the SDK looks forward from the cursor for the matching
// event. If a match is found the command is treated as already-done
// (no RPC is issued, the cursor advances past the match). If no match
// is found the SDK issues the corresponding gRPC call and leaves the
// cursor where it was, so subsequent commands keep scanning from the
// same point.
//
// This is the smallest piece of event-sourced replay: it makes
// workflow re-runs safe (activities are not rescheduled, signals are
// not double-delivered). Engine-side re-dispatch on activity / signal
// arrival arrives in a follow-up.
type replayState struct {
	history []*proto.WorkflowEvent
	cursor  int
}

func newReplayState(history []*proto.WorkflowEvent) *replayState {
	return &replayState{history: history}
}

// findActivityScheduled searches the history past the cursor for the
// next ActivityScheduled event whose payload's activity_name matches.
// Returns the event and its index on success, or (nil, -1) when no
// match exists.
func (r *replayState) findActivityScheduled(activityName string) (*proto.WorkflowEvent, int) {
	if r == nil {
		return nil, -1
	}
	for i := r.cursor; i < len(r.history); i++ {
		ev := r.history[i]
		if ev.EventType != "ActivityScheduled" {
			continue
		}
		var details struct {
			ActivityName string `json:"activity_name"`
		}
		if err := json.Unmarshal([]byte(ev.Details), &details); err != nil {
			continue
		}
		if details.ActivityName == activityName {
			return ev, i
		}
	}
	return nil, -1
}

// findSignalReceived searches for the next SignalReceived past the
// cursor. Signals do not carry a "name" filter at this layer; the
// workflow is expected to inspect SignalName itself.
func (r *replayState) findSignalReceived() (*proto.WorkflowEvent, int) {
	if r == nil {
		return nil, -1
	}
	for i := r.cursor; i < len(r.history); i++ {
		ev := r.history[i]
		if ev.EventType == "SignalReceived" {
			return ev, i
		}
	}
	return nil, -1
}

// findTimerScheduled returns the next TimerScheduled event past the
// cursor along with the timer_id from its details. Used by Sleep
// during replay: each Sleep call matches sequentially against the
// next outstanding timer in history.
func (r *replayState) findTimerScheduled() (*proto.WorkflowEvent, int, string) {
	if r == nil {
		return nil, -1, ""
	}
	for i := r.cursor; i < len(r.history); i++ {
		ev := r.history[i]
		if ev.EventType != "TimerScheduled" {
			continue
		}
		var details struct {
			TimerID string `json:"timer_id"`
		}
		if err := json.Unmarshal([]byte(ev.Details), &details); err != nil {
			continue
		}
		return ev, i, details.TimerID
	}
	return nil, -1, ""
}

// findTimerFired searches the history from fromIdx forward for a
// TimerFired event whose timer_id matches. Returns (nil, -1) when no
// matching fire event exists yet.
func (r *replayState) findTimerFired(timerID string, fromIdx int) (*proto.WorkflowEvent, int) {
	if r == nil || timerID == "" {
		return nil, -1
	}
	for i := fromIdx + 1; i < len(r.history); i++ {
		ev := r.history[i]
		if ev.EventType != "TimerFired" {
			continue
		}
		var details struct {
			TimerID string `json:"timer_id"`
		}
		if err := json.Unmarshal([]byte(ev.Details), &details); err != nil {
			continue
		}
		if details.TimerID == timerID {
			return ev, i
		}
	}
	return nil, -1
}

// advance moves the cursor past the given index. The SDK calls this
// when it matches a recorded event to a current command; future scans
// start from the new position.
func (r *replayState) advance(idx int) {
	if r != nil && idx+1 > r.cursor {
		r.cursor = idx + 1
	}
}

// yieldErr is the sentinel value the SDK panics with when a workflow's
// blocking command (WaitForSignal today, Sleep later) has no matching
// event in history. The communicator's recover handler treats this
// panic as a controlled yield: the workflow task completes with
// yielded=true so the engine knows to re-dispatch when the awaited
// event arrives.
type yieldErr struct {
	command string
}

func (y yieldErr) Error() string { return "workflow yielded on " + y.command }

// IsYield reports whether r is a yield sentinel. The communicator
// uses this from a recover() to distinguish a controlled yield from
// a real panic that should propagate.
func IsYield(r any) bool {
	_, ok := r.(yieldErr)
	return ok
}
