package engine

import (
	"testing"
	"time"
)

func TestRetryPolicy_BackoffFor(t *testing.T) {
	p := RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    60 * time.Second,
		MaximumAttempts:    5,
	}
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 0},                // first call has no backoff
		{2, 1 * time.Second},  // first retry uses InitialInterval
		{3, 2 * time.Second},  // second retry doubles
		{4, 4 * time.Second},  // third retry doubles again
		{5, 8 * time.Second},  // fourth retry
		{8, 60 * time.Second}, // capped at MaximumInterval
	}
	for _, tc := range cases {
		got := p.BackoffFor(tc.attempt)
		if got != tc.want {
			t.Errorf("BackoffFor(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestRetryPolicy_BackoffFor_ZeroDefaults(t *testing.T) {
	// All-zero policy should not panic; coefficient clamps to 1 so all
	// retries share the same delay.
	p := RetryPolicy{}
	if got := p.BackoffFor(2); got != time.Second {
		t.Errorf("BackoffFor(2) on zero policy = %v, want 1s default", got)
	}
}
