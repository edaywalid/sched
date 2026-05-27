package store

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStore_CreateGetHistory(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	wf := Workflow{WorkflowID: "wf-1", RunID: "run-1", Name: "Test", Input: []byte(`"hello"`)}
	if err := s.CreateWorkflow(ctx, wf); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	if err := s.CreateWorkflow(ctx, wf); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate insert: got %v, want ErrConflict", err)
	}

	got, err := s.GetWorkflow(ctx, "wf-1")
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.Status != StatusRunning {
		t.Fatalf("default status = %q, want %q", got.Status, StatusRunning)
	}

	if err := s.AppendEvent(ctx, Event{WorkflowID: "wf-1", Type: EventWorkflowStarted}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if err := s.AppendEvent(ctx, Event{WorkflowID: "wf-1", Type: EventActivityScheduled}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	history, err := s.GetHistory(ctx, "wf-1")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history len = %d, want 2", len(history))
	}
	if history[0].Idx != 1 || history[1].Idx != 2 {
		t.Fatalf("idx not monotonic: %d, %d", history[0].Idx, history[1].Idx)
	}
}

func TestMemoryStore_CompleteAndList(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	for _, id := range []string{"a", "b", "c"} {
		if err := s.CreateWorkflow(ctx, Workflow{WorkflowID: id, Name: "X"}); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	if err := s.CompleteWorkflow(ctx, "b", StatusCompleted, []byte(`"ok"`), ""); err != nil {
		t.Fatalf("CompleteWorkflow: %v", err)
	}

	running, err := s.ListWorkflows(ctx, ListFilter{Status: StatusRunning})
	if err != nil {
		t.Fatalf("ListWorkflows running: %v", err)
	}
	if len(running) != 2 {
		t.Fatalf("running = %d, want 2", len(running))
	}

	done, err := s.ListWorkflows(ctx, ListFilter{Status: StatusCompleted})
	if err != nil {
		t.Fatalf("ListWorkflows completed: %v", err)
	}
	if len(done) != 1 || done[0].WorkflowID != "b" {
		t.Fatalf("done = %+v, want [b]", done)
	}
	if done[0].EndTime == nil {
		t.Fatalf("EndTime not set on completed workflow")
	}
}

func TestMemoryStore_WorkflowNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	if _, err := s.GetWorkflow(ctx, "missing"); !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("GetWorkflow missing: got %v, want ErrWorkflowNotFound", err)
	}
	if err := s.AppendEvent(ctx, Event{WorkflowID: "missing", Type: EventWorkflowStarted}); !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("AppendEvent missing: got %v, want ErrWorkflowNotFound", err)
	}
}
