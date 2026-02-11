package store

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_TimerLifecycle(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	if err := s.CreateWorkflow(ctx, Workflow{WorkflowID: "wf-t", Name: "T"}); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	soon := time.Now().Add(20 * time.Millisecond)
	later := time.Now().Add(10 * time.Second)
	if err := s.InsertTimer(ctx, Timer{TimerID: "t1", WorkflowID: "wf-t", FireAt: soon}); err != nil {
		t.Fatalf("InsertTimer t1: %v", err)
	}
	if err := s.InsertTimer(ctx, Timer{TimerID: "t2", WorkflowID: "wf-t", FireAt: later}); err != nil {
		t.Fatalf("InsertTimer t2: %v", err)
	}

	due, err := s.FetchDueTimers(ctx, 10)
	if err != nil {
		t.Fatalf("FetchDueTimers immediate: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("immediate fetch returned %d timers, want 0", len(due))
	}

	time.Sleep(30 * time.Millisecond)
	due, err = s.FetchDueTimers(ctx, 10)
	if err != nil {
		t.Fatalf("FetchDueTimers: %v", err)
	}
	if len(due) != 1 || due[0].TimerID != "t1" {
		t.Fatalf("due = %+v, want [t1]", due)
	}

	// Second fetch shouldn't return t1 again.
	due, err = s.FetchDueTimers(ctx, 10)
	if err != nil {
		t.Fatalf("FetchDueTimers second: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("second fetch = %+v, want empty (already fired)", due)
	}

	pending, err := s.ListPendingTimers(ctx)
	if err != nil {
		t.Fatalf("ListPendingTimers: %v", err)
	}
	if len(pending) != 1 || pending[0].TimerID != "t2" {
		t.Fatalf("pending = %+v, want [t2]", pending)
	}
}
