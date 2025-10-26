package engine

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

 
type WorkflowStatus string

const (
	WorkflowStatusRunning   WorkflowStatus = "RUNNING"
	WorkflowStatusCompleted WorkflowStatus = "COMPLETED"
	WorkflowStatusFailed    WorkflowStatus = "FAILED"
	WorkflowStatusTimedOut  WorkflowStatus = "TIMED_OUT"
)

 
type WorkflowExecution struct {
	WorkflowID   string
	WorkflowName string
	Status       WorkflowStatus
	Input        interface{}
	Result       interface{}
	Error        error
	StartTime    time.Time
	EndTime      *time.Time
	History      []WorkflowEvent
	Version      int
}

 
type WorkflowEvent struct {
	EventID   int64
	EventType string
	Timestamp time.Time
	Details   interface{}
}

 
const (
	EventTypeWorkflowStarted   = "WorkflowStarted"
	EventTypeWorkflowCompleted = "WorkflowCompleted"
	EventTypeWorkflowFailed    = "WorkflowFailed"
	EventTypeActivityScheduled = "ActivityScheduled"
	EventTypeActivityCompleted = "ActivityCompleted"
	EventTypeActivityFailed    = "ActivityFailed"
	EventTypeTimerScheduled    = "TimerScheduled"
	EventTypeTimerFired        = "TimerFired"
	EventTypeSignalReceived    = "SignalReceived"
)

 
type PersistenceLayer struct {
	executions map[string]*WorkflowExecution
	mu         sync.RWMutex
}

func NewPersistenceLayer() *PersistenceLayer {
	return &PersistenceLayer{
		executions: make(map[string]*WorkflowExecution),
	}
}

 
func (p *PersistenceLayer) CreateWorkflowExecution(workflowID, workflowName string, input interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	execution := &WorkflowExecution{
		WorkflowID:   workflowID,
		WorkflowName: workflowName,
		Status:       WorkflowStatusRunning,
		Input:        input,
		StartTime:    time.Now(),
		History:      []WorkflowEvent{},
		Version:      1,
	}

	p.executions[workflowID] = execution
	p.addEvent(execution, EventTypeWorkflowStarted, map[string]interface{}{
		"workflow_name": workflowName,
		"input":         input,
	})

	return nil
}

 
func (p *PersistenceLayer) AddEvent(workflowID string, eventType string, details interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	execution, ok := p.executions[workflowID]
	if !ok {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	p.addEvent(execution, eventType, details)
	return nil
}

func (p *PersistenceLayer) addEvent(execution *WorkflowExecution, eventType string, details interface{}) {
	event := WorkflowEvent{
		EventID:   int64(len(execution.History) + 1),
		EventType: eventType,
		Timestamp: time.Now(),
		Details:   details,
	}
	execution.History = append(execution.History, event)
	execution.Version++
}

 
func (p *PersistenceLayer) CompleteWorkflow(workflowID string, result interface{}, err error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	execution, ok := p.executions[workflowID]
	if !ok {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	now := time.Now()
	execution.EndTime = &now
	execution.Result = result
	execution.Error = err

	if err != nil {
		execution.Status = WorkflowStatusFailed
		p.addEvent(execution, EventTypeWorkflowFailed, map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		execution.Status = WorkflowStatusCompleted
		p.addEvent(execution, EventTypeWorkflowCompleted, map[string]interface{}{
			"result": result,
		})
	}

	return nil
}

 
func (p *PersistenceLayer) GetWorkflowExecution(workflowID string) (*WorkflowExecution, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	execution, ok := p.executions[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return execution, nil
}

 
func (p *PersistenceLayer) GetWorkflowStatus(workflowID string) (string, interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	execution, ok := p.executions[workflowID]
	if !ok {
		return "", nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return string(execution.Status), execution.Result, execution.Error
}

 
func (p *PersistenceLayer) GetWorkflowHistory(workflowID string) ([]WorkflowEvent, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	execution, ok := p.executions[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return execution.History, nil
}

 
func (p *PersistenceLayer) ReplayWorkflow(workflowID string) (*WorkflowExecution, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	execution, ok := p.executions[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Create a copy for replay
	replayed := &WorkflowExecution{
		WorkflowID:   execution.WorkflowID,
		WorkflowName: execution.WorkflowName,
		Status:       WorkflowStatusRunning,
		Input:        execution.Input,
		StartTime:    execution.StartTime,
		History:      make([]WorkflowEvent, len(execution.History)),
		Version:      execution.Version,
	}

	copy(replayed.History, execution.History)

	return replayed, nil
}

 
func (p *PersistenceLayer) SaveSnapshot(workflowID string) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	execution, ok := p.executions[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return json.Marshal(execution)
}

 
func (p *PersistenceLayer) LoadSnapshot(workflowID string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var execution WorkflowExecution
	if err := json.Unmarshal(data, &execution); err != nil {
		return fmt.Errorf("failed to unmarshal execution: %w", err)
	}

	p.executions[workflowID] = &execution
	return nil
}

 
func (p *PersistenceLayer) GetAllExecutions() []*WorkflowExecution {
	p.mu.RLock()
	defer p.mu.RUnlock()

	executions := make([]*WorkflowExecution, 0, len(p.executions))
	for _, execution := range p.executions {
		executions = append(executions, execution)
	}
	return executions
}
