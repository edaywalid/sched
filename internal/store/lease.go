package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LeaderLease holds a Postgres session-scoped advisory lock on behalf
// of one engine instance. While the lease is held the holder is the
// cluster's leader; the rest of the engine subsystem (timers,
// reclaim, gRPC server) only runs on the leader.
//
// The lock is tied to the underlying pgx connection. When the
// connection drops the lock auto-releases, so another engine can
// claim leadership. LeaderLease.Watch surfaces that as a channel send
// so the binary main can exit on lease loss.
type LeaderLease struct {
	conn     *pgxpool.Conn
	lockKey  int64
	lost     chan struct{}
	cancelCh chan struct{}
}

// AcquireLeaderLease blocks until pg_try_advisory_lock(lockKey)
// succeeds, polling every retryEvery in between. lockKey is an
// arbitrary int64 that must be the same across all instances of the
// cluster; the convention is hash("sched-engine") or a config'd value.
//
// Returns a LeaderLease whose Release closes the connection and lets
// another waiter take over.
func AcquireLeaderLease(ctx context.Context, pool *pgxpool.Pool, lockKey int64, retryEvery time.Duration) (*LeaderLease, error) {
	if retryEvery <= 0 {
		retryEvery = 5 * time.Second
	}
	for {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return nil, fmt.Errorf("acquire pgx conn: %w", err)
		}
		var ok bool
		row := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey)
		if err := row.Scan(&ok); err != nil {
			conn.Release()
			return nil, fmt.Errorf("pg_try_advisory_lock: %w", err)
		}
		if ok {
			lease := &LeaderLease{
				conn:     conn,
				lockKey:  lockKey,
				lost:     make(chan struct{}),
				cancelCh: make(chan struct{}),
			}
			go lease.watchConnection(ctx)
			return lease, nil
		}
		// Lock held elsewhere. Release the pooled connection and
		// wait.
		conn.Release()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryEvery):
		}
	}
}

// Lost returns a channel that closes when the lease is no longer
// held. Callers should treat the close as a fatal: another engine is
// about to (or already has) taken over leadership.
func (l *LeaderLease) Lost() <-chan struct{} { return l.lost }

// Release explicitly drops the lease. After Release the Lost channel
// is closed.
func (l *LeaderLease) Release(ctx context.Context) {
	select {
	case <-l.cancelCh:
		return
	default:
		close(l.cancelCh)
	}
	// Best-effort explicit unlock; closing the conn would also do it
	// implicitly by releasing the session.
	if l.conn != nil {
		_, _ = l.conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, l.lockKey)
		l.conn.Release()
	}
	select {
	case <-l.lost:
	default:
		close(l.lost)
	}
}

// watchConnection pings the dedicated lock connection every second.
// If the ping fails (connection died, network blip, server restart)
// the lease is considered lost and the Lost channel fires.
func (l *LeaderLease) watchConnection(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-l.cancelCh:
			return
		case <-ctx.Done():
			l.markLost()
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := l.conn.Conn().Ping(pingCtx)
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				l.markLost()
				return
			}
		}
	}
}

func (l *LeaderLease) markLost() {
	select {
	case <-l.lost:
	default:
		close(l.lost)
	}
}
