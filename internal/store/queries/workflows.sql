-- name: CreateWorkflow :exec
INSERT INTO workflow_executions (
    workflow_id, run_id, name, status, input, start_time, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
);

-- name: GetWorkflow :one
SELECT workflow_id, run_id, name, status, input, result, error,
       start_time, end_time, version
FROM workflow_executions
WHERE workflow_id = $1;

-- name: ListWorkflows :many
SELECT workflow_id, run_id, name, status, input, result, error,
       start_time, end_time, version
FROM workflow_executions
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY start_time DESC
LIMIT sqlc.arg('limit_n');

-- name: CompleteWorkflow :exec
UPDATE workflow_executions
SET status   = $2,
    result   = $3,
    error    = $4,
    end_time = NOW(),
    version  = version + 1
WHERE workflow_id = $1;

-- name: BumpVersion :exec
UPDATE workflow_executions
SET version = version + 1
WHERE workflow_id = $1;
