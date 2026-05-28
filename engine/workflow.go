package engine

import (
	"math"
	"time"
)

// RetryPolicy controls how the engine schedules retries when an
// activity fails. BackoffCoefficient is multiplied into the previous
// delay (capped by MaximumInterval) to produce the next delay.
//
// Attempt counts start at 1: the first call is attempt=1, the first
// retry is attempt=2, and so on. MaximumAttempts is inclusive — with
// MaximumAttempts=3 the engine tries the activity up to three times
// total before giving up.
type RetryPolicy struct {
	InitialInterval    time.Duration `json:"initial_interval"`
	BackoffCoefficient float64       `json:"backoff_coefficient"`
	MaximumInterval    time.Duration `json:"maximum_interval"`
	MaximumAttempts    int           `json:"maximum_attempts"`
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    60 * time.Second,
		MaximumAttempts:    3,
	}
}

// BackoffFor returns the delay before the given attempt number runs.
// attempt is the attempt about to start (so the first retry passes
// attempt=2). The result is clamped to [InitialInterval, MaximumInterval].
func (p RetryPolicy) BackoffFor(attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}
	initial := p.InitialInterval
	if initial <= 0 {
		initial = time.Second
	}
	coeff := p.BackoffCoefficient
	if coeff <= 1 {
		coeff = 1
	}
	maxInt := p.MaximumInterval
	if maxInt <= 0 {
		maxInt = math.MaxInt64 // effectively unbounded
	}
	delay := float64(initial) * math.Pow(coeff, float64(attempt-2))
	if d := time.Duration(delay); d > maxInt {
		return maxInt
	}
	return time.Duration(delay)
}
