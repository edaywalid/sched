package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/edaywalid/sched/internal/store"
	"github.com/google/uuid"
)

// pollInterval is how often the timer loop wakes to scan Postgres for
// due timers. Short enough to keep timer firing reasonably crisp,
// long enough not to hammer the database when idle.
const pollInterval = 250 * time.Millisecond

// Timer is the in-memory shadow of a durable timers-table row.
// Callbacks are local-only; after a process restart, recovered timers
// have no callback attached — Phase 3 wires the firing effect back
// into workflow replay.
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
	store store.Store

	mu        sync.Mutex
	callbacks map[string]func() // timer_id -> local callback

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewTimerManager(s store.Store) *TimerManager {
	tm := &TimerManager{
		store:     s,
		callbacks: make(map[string]func()),
		stopCh:    make(chan struct{}),
	}
	tm.wg.Add(1)
	go tm.loop()
	return tm
}

// Stop halts the timer loop and waits for the current iteration to
// finish.
func (tm *TimerManager) Stop() {
	select {
	case <-tm.stopCh:
	default:
		close(tm.stopCh)
	}
	tm.wg.Wait()
}

// ScheduleTimer persists a new pending timer and records its callback
// (if any) in memory. ScheduleTimer is synchronous: it returns once
// the row is durable.
func (tm *TimerManager) ScheduleTimer(ctx context.Context, workflowID string, duration time.Duration, callback func()) (string, error) {
	timerID := uuid.New().String()
	now := time.Now()
	fireAt := now.Add(duration)

	if err := tm.store.InsertTimer(ctx, store.Timer{
		TimerID:    timerID,
		WorkflowID: workflowID,
		FireAt:     fireAt,
	}); err != nil {
		return "", fmt.Errorf("persist timer: %w", err)
	}

	tm.mu.Lock()
	if callback != nil {
		tm.callbacks[timerID] = callback
	}
	tm.mu.Unlock()

	details, _ := json.Marshal(map[string]any{
		"timer_id": timerID,
		"duration": duration.String(),
		"fire_at":  fireAt,
	})
	if err := tm.store.AppendEvent(ctx, store.Event{
		WorkflowID: workflowID,
		Type:       EventTypeTimerScheduled,
		Details:    details,
	}); err != nil {
		log.Printf("timer: append scheduled event for %s: %v", timerID, err)
	}

	return timerID, nil
}

// RecoverPendingTimers loads unfired timer rows from the store at
// startup. Recovered timers have no callback because the previous
// process is gone; the timer loop will still mark them fired and
// emit the TimerFired history event, but no in-process side-effect
// runs until Phase 3 reattaches the workflow replay path.
func (tm *TimerManager) RecoverPendingTimers(ctx context.Context) (int, error) {
	pending, err := tm.store.ListPendingTimers(ctx)
	if err != nil {
		return 0, fmt.Errorf("list pending timers: %w", err)
	}
	if len(pending) == 0 {
		return 0, nil
	}
	log.Printf("timer: recovered %d pending timer(s) from store", len(pending))
	return len(pending), nil
}

func (tm *TimerManager) loop() {
	defer tm.wg.Done()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-tm.stopCh:
			return
		case <-ticker.C:
			tm.fireDue()
		}
	}
}

func (tm *TimerManager) fireDue() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	due, err := tm.store.FetchDueTimers(ctx, 64)
	if err != nil {
		// Suppress the noisy log when the store is being torn down in
		// tests; otherwise surface the failure.
		if !errors.Is(err, context.Canceled) {
			log.Printf("timer: fetch due failed: %v", err)
		}
		return
	}
	for _, t := range due {
		details, _ := json.Marshal(map[string]any{
			"timer_id": t.TimerID,
			"fired_at": time.Now(),
		})
		if err := tm.store.AppendEvent(ctx, store.Event{
			WorkflowID: t.WorkflowID,
			Type:       EventTypeTimerFired,
			Details:    details,
		}); err != nil {
			log.Printf("timer: append fired event for %s: %v", t.TimerID, err)
		}

		tm.mu.Lock()
		cb := tm.callbacks[t.TimerID]
		delete(tm.callbacks, t.TimerID)
		tm.mu.Unlock()
		if cb != nil {
			go cb()
		}
	}
}
