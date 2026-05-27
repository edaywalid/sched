package store

import (
	"context"
	"errors"
	"os"
	"testing"
)

// TestPostgresStore_RoundTrip exercises CreateWorkflow / AppendEvent /
// CompleteWorkflow / GetHistory / ListWorkflows against a real Postgres.
//
// The test is skipped unless SCHED_TEST_POSTGRES_DSN is set, so `go test
// ./...` stays green without local infra. To run it:
//
//	docker compose up -d postgres
//	make migrate-up
//	SCHED_TEST_POSTGRES_DSN=postgres://sched:sched_password@localhost:5432/sched?sslmode=disable \
//	    go test ./internal/store -run TestPostgresStore -v
func TestPostgresStore_RoundTrip(t *testing.T) {
	dsn := os.Getenv("SCHED_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SCHED_TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	s, err := NewPostgresStore(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}
	defer s.Close()

	// Clean slate for the workflow IDs this test uses.
	if _, err := s.Pool().Exec(ctx, `DELETE FROM workflow_executions WHERE workflow_id IN ('it-1', 'it-2')`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if err := s.CreateWorkflow(ctx, Workflow{WorkflowID: "it-1", RunID: "r1", Name: "T", Input: []byte(`{"x":1}`)}); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	if err := s.AppendEvent(ctx, Event{WorkflowID: "it-1", Type: EventWorkflowStarted, Details: []byte(`{}`)}); err != nil {
		t.Fatalf("AppendEvent #1: %v", err)
	}
	if err := s.AppendEvent(ctx, Event{WorkflowID: "it-1", Type: EventActivityScheduled, Details: []byte(`{"n":"a"}`)}); err != nil {
		t.Fatalf("AppendEvent #2: %v", err)
	}

	history, err := s.GetHistory(ctx, "it-1")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history len = %d, want 2", len(history))
	}
	if history[0].Idx != 1 || history[1].Idx != 2 {
		t.Fatalf("idx not monotonic: %d %d", history[0].Idx, history[1].Idx)
	}

	if err := s.CompleteWorkflow(ctx, "it-1", StatusCompleted, []byte(`"done"`), ""); err != nil {
		t.Fatalf("CompleteWorkflow: %v", err)
	}

	got, err := s.GetWorkflow(ctx, "it-1")
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.Status != StatusCompleted {
		t.Fatalf("status = %q, want COMPLETED", got.Status)
	}
	if got.EndTime == nil {
		t.Fatalf("EndTime not set after complete")
	}

	// ListWorkflows filter.
	if err := s.CreateWorkflow(ctx, Workflow{WorkflowID: "it-2", RunID: "r2", Name: "T"}); err != nil {
		t.Fatalf("CreateWorkflow it-2: %v", err)
	}
	running, err := s.ListWorkflows(ctx, ListFilter{Status: StatusRunning, Limit: 50})
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	foundIt2 := false
	for _, w := range running {
		if w.WorkflowID == "it-2" {
			foundIt2 = true
		}
		if w.WorkflowID == "it-1" {
			t.Fatalf("status filter leaked completed workflow")
		}
	}
	if !foundIt2 {
		t.Fatalf("running list missing it-2: %+v", running)
	}

	if _, err := s.GetWorkflow(ctx, "does-not-exist"); !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("GetWorkflow missing: got %v, want ErrWorkflowNotFound", err)
	}
}
