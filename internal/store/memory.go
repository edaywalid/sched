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
	timers    map[string]*Timer
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		workflows: make(map[string]*Workflow),
		events:    make(map[string][]Event),
		timers:    make(map[string]*Timer),
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

func (m *MemoryStore) InsertTimer(_ context.Context, t Timer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.timers == nil {
		m.timers = make(map[string]*Timer)
	}
	if _, exists := m.timers[t.TimerID]; exists {
		return ErrConflict
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	stored := t
	m.timers[t.TimerID] = &stored
	return nil
}

func (m *MemoryStore) FetchDueTimers(_ context.Context, limit int) ([]Timer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	due := make([]Timer, 0)
	for _, t := range m.timers {
		if !t.Fired && !t.FireAt.After(now) {
			t.Fired = true
			c := *t
			due = append(due, c)
			if limit > 0 && len(due) >= limit {
				break
			}
		}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].FireAt.Before(due[j].FireAt) })
	return due, nil
}

func (m *MemoryStore) ListPendingTimers(_ context.Context) ([]Timer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Timer, 0)
	for _, t := range m.timers {
		if !t.Fired {
			c := *t
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FireAt.Before(out[j].FireAt) })
	return out, nil
}

func (m *MemoryStore) Close() error { return nil }
