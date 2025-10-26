package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

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
	timers      map[string]*Timer
	mu          sync.RWMutex
	persistence *PersistenceLayer
}

func NewTimerManager(persistence *PersistenceLayer) *TimerManager {
	tm := &TimerManager{
		timers:      make(map[string]*Timer),
		persistence: persistence,
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

	tm.persistence.AddEvent(workflowID, EventTypeTimerScheduled, map[string]interface{}{
		"timer_id": timerID,
		"duration": duration.String(),
		"fire_at":  timer.FireAt,
	})

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

// timerLoop checks and fires timers
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

			tm.persistence.AddEvent(timer.WorkflowID, EventTypeTimerFired, map[string]interface{}{
				"timer_id": timerID,
				"fired_at": now,
			})

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

func (tm *TimerManager) RestoreTimers(workflowID string, events []WorkflowEvent) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, event := range events {
		if event.EventType == EventTypeTimerScheduled {
			details := event.Details.(map[string]interface{})
			timerID := details["timer_id"].(string)

			// Check if timer already fired
			fired := false
			for _, e := range events {
				if e.EventType == EventTypeTimerFired {
					firedDetails := e.Details.(map[string]interface{})
					if firedDetails["timer_id"].(string) == timerID {
						fired = true
						break
					}
				}
			}

			if !fired {
				// Restore unfired timer
			}
		}
	}

	return nil
}
