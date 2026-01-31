package store

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory Store implementation intended for unit tests
// and local development. State is lost when the process exits.
type MemoryStore struct {
	mu        sync.RWMutex
	workflows map[string]*Workflow
	events    map[string][]Event
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		workflows: make(map[string]*Workflow),
		events:    make(map[string][]Event),
	}
}

func (m *MemoryStore) CreateWorkflow(_ context.Context, wf Workflow) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.workflows[wf.WorkflowID]; exists {
		return ErrConflict
	}
	if wf.StartTime.IsZero() {
		wf.StartTime = time.Now()
	}
	if wf.Version == 0 {
		wf.Version = 1
	}
	if wf.Status == "" {
		wf.Status = StatusRunning
	}

	stored := wf
	m.workflows[wf.WorkflowID] = &stored
	return nil
}

func (m *MemoryStore) AppendEvent(_ context.Context, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wf, ok := m.workflows[event.WorkflowID]
	if !ok {
		return ErrWorkflowNotFound
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	history := m.events[event.WorkflowID]
	event.Idx = int64(len(history) + 1)
	m.events[event.WorkflowID] = append(history, event)
	wf.Version++
	return nil
}

func (m *MemoryStore) CompleteWorkflow(_ context.Context, workflowID string, status WorkflowStatus, result []byte, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wf, ok := m.workflows[workflowID]
	if !ok {
		return ErrWorkflowNotFound
	}
	now := time.Now()
	wf.Status = status
	wf.Result = result
	wf.Error = errMsg
	wf.EndTime = &now
	wf.Version++
	return nil
}

func (m *MemoryStore) GetWorkflow(_ context.Context, workflowID string) (*Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wf, ok := m.workflows[workflowID]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	copy := *wf
	return &copy, nil
}

func (m *MemoryStore) GetHistory(_ context.Context, workflowID string) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.workflows[workflowID]; !ok {
		return nil, ErrWorkflowNotFound
	}
	history := m.events[workflowID]
	out := make([]Event, len(history))
	copy(out, history)
	return out, nil
}

func (m *MemoryStore) ListWorkflows(_ context.Context, filter ListFilter) ([]*Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Workflow, 0, len(m.workflows))
	for _, wf := range m.workflows {
		if filter.Status != "" && wf.Status != filter.Status {
			continue
		}
		c := *wf
		out = append(out, &c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartTime.After(out[j].StartTime)
	})
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (m *MemoryStore) Close() error { return nil }
