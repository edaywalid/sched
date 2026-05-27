-- name: AppendEvent :one
-- AppendEvent inserts the next event for a workflow. The idx is derived
-- from the current MAX(idx) for that workflow; the (workflow_id, idx)
-- primary key guarantees monotonicity. Concurrent appends to the same
-- workflow may collide on idx; callers must serialize per workflow or
-- retry on unique-violation.
INSERT INTO workflow_events (workflow_id, idx, event_type, timestamp, details)
VALUES (
    $1,
    (SELECT COALESCE(MAX(idx), 0) + 1
       FROM workflow_events
       WHERE workflow_id = $1),
    $2,
    NOW(),
    $3
)
RETURNING idx, timestamp;

-- name: GetHistory :many
SELECT workflow_id, idx, event_type, timestamp, details
FROM workflow_events
WHERE workflow_id = $1
ORDER BY idx ASC;
