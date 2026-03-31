package sdk

import (
	"testing"

	"github.com/edaywalid/sched/proto"
)

func ev(eventType, details string) *proto.WorkflowEvent {
	return &proto.WorkflowEvent{EventType: eventType, Details: details}
}

func TestReplayState_FindActivityScheduled(t *testing.T) {
	r := newReplayState([]*proto.WorkflowEvent{
		ev("WorkflowStarted", `{"workflow_name":"X"}`),
		ev("ActivityScheduled", `{"activity_id":"1","activity_name":"SendEmail"}`),
		ev("ActivityCompleted", `{"activity_name":"SendEmail"}`),
		ev("ActivityScheduled", `{"activity_id":"2","activity_name":"SendNotification"}`),
	})

	got, idx := r.findActivityScheduled("SendEmail")
	if got == nil || idx != 1 {
		t.Fatalf("first SendEmail = %+v idx=%d, want idx=1", got, idx)
	}
	r.advance(idx)

	// Next SendEmail search past cursor finds nothing.
	if got, _ := r.findActivityScheduled("SendEmail"); got != nil {
		t.Errorf("second SendEmail search returned %+v, want nil", got)
	}

	// SendNotification still visible past cursor.
	if got, idx := r.findActivityScheduled("SendNotification"); got == nil || idx != 3 {
		t.Errorf("SendNotification idx = %d, want 3", idx)
	}
}

func TestReplayState_FindSignalReceived(t *testing.T) {
	r := newReplayState([]*proto.WorkflowEvent{
		ev("WorkflowStarted", "{}"),
		ev("SignalReceived", `{"signal_name":"approve","input":"yes"}`),
		ev("ActivityScheduled", `{"activity_name":"Notify"}`),
		ev("SignalReceived", `{"signal_name":"resume"}`),
	})

	got, idx := r.findSignalReceived()
	if got == nil || idx != 1 {
		t.Fatalf("first signal idx = %d, want 1", idx)
	}
	r.advance(idx)

	got2, idx2 := r.findSignalReceived()
	if got2 == nil || idx2 != 3 {
		t.Fatalf("second signal idx = %d, want 3", idx2)
	}
}

func TestReplayState_NilSafe(t *testing.T) {
	var r *replayState
	if got, idx := r.findActivityScheduled("x"); got != nil || idx != -1 {
		t.Errorf("nil-receiver findActivityScheduled = %v %d", got, idx)
	}
	r.advance(0) // must not panic
}
