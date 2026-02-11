-- name: InsertTimer :exec
INSERT INTO timers (timer_id, workflow_id, fire_at, fired)
VALUES ($1, $2, $3, FALSE);

-- name: FetchDueTimers :many
-- FOR UPDATE SKIP LOCKED lets multiple engine replicas poll the same
-- table without stepping on each other. Each row is claimed by exactly
-- one caller until the transaction commits.
SELECT timer_id, workflow_id, fire_at, fired, created_at
FROM timers
WHERE NOT fired
  AND fire_at <= NOW()
ORDER BY fire_at
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkTimerFired :exec
UPDATE timers
SET fired = TRUE
WHERE timer_id = $1;

-- name: GetPendingTimers :many
-- Used by TimerManager on startup to repopulate in-memory tracking
-- for unfired timers persisted by a previous run.
SELECT timer_id, workflow_id, fire_at, fired, created_at
FROM timers
WHERE NOT fired
ORDER BY fire_at;
