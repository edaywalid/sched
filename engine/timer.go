package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/edaywalid/sched/internal/store"
	"github.com/google/uuid"
)

// Timer is an in-memory timer record. Phase 2 introduces durable timers
// persisted in the `timers` table and recovery on engine startup.
type Timer struct {
	TimerID    string
	WorkflowID string
	Duration   time.Duration
	CreatedAt  time.Time
	FireAt     time.Time
	Fired      bool
	Callback   func()
}

type TimerManager struct {
	timers map[string]*Timer
	mu     sync.RWMutex
	store  store.Store
}

func NewTimerManager(s store.Store) *TimerManager {
	tm := &TimerManager{
		timers: make(map[string]*Timer),
		store:  s,
	}
	go tm.timerLoop()
	return tm
}

func (tm *TimerManager) ScheduleTimer(workflowID string, duration time.Duration, callback func()) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	timerID := uuid.New().String()
	now := time.Now()

	timer := &Timer{
		TimerID:    timerID,
		WorkflowID: workflowID,
		Duration:   duration,
		CreatedAt:  now,
		FireAt:     now.Add(duration),
		Fired:      false,
		Callback:   callback,
	}

	tm.timers[timerID] = timer

	details, _ := json.Marshal(map[string]any{
		"timer_id": timerID,
		"duration": duration.String(),
		"fire_at":  timer.FireAt,
	})
	if err := tm.store.AppendEvent(context.Background(), store.Event{
		WorkflowID: workflowID,
		Type:       EventTypeTimerScheduled,
		Details:    details,
	}); err != nil {
		// Persistence failure: log and continue. Phase 2 makes timers
		// fully durable, at which point this becomes a fatal error.
		fmt.Printf("timer: failed to record scheduled event: %v\n", err)
	}

	return timerID, nil
}

func (tm *TimerManager) CancelTimer(timerID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	timer, ok := tm.timers[timerID]
	if !ok {
		return fmt.Errorf("timer not found: %s", timerID)
	}
	if timer.Fired {
		return fmt.Errorf("timer already fired: %s", timerID)
	}

	delete(tm.timers, timerID)
	return nil
}

func (tm *TimerManager) timerLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		tm.checkAndFireTimers()
	}
}

func (tm *TimerManager) checkAndFireTimers() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	for timerID, timer := range tm.timers {
		if !timer.Fired && now.After(timer.FireAt) {
			timer.Fired = true

			details, _ := json.Marshal(map[string]any{
				"timer_id": timerID,
				"fired_at": now,
			})
			if err := tm.store.AppendEvent(context.Background(), store.Event{
				WorkflowID: timer.WorkflowID,
				Type:       EventTypeTimerFired,
				Details:    details,
			}); err != nil {
				fmt.Printf("timer: failed to record fired event: %v\n", err)
			}

			if timer.Callback != nil {
				go timer.Callback()
			}

			delete(tm.timers, timerID)
		}
	}
}

func (tm *TimerManager) GetPendingTimers(workflowID string) []*Timer {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var pending []*Timer
	for _, timer := range tm.timers {
		if timer.WorkflowID == workflowID && !timer.Fired {
			pending = append(pending, timer)
		}
	}
	return pending
}
