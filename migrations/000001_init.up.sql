-- Phase 1: durable single-node workflow engine.
-- Schema reflects the in-memory PersistenceLayer contract: one row per
-- workflow execution, append-only event history, and lightweight queues
-- for tasks and timers. Sharding columns (shard_id) are added in Phase 4.

CREATE TABLE workflow_executions (
    workflow_id   TEXT        PRIMARY KEY,
    run_id        TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    status        TEXT        NOT NULL,
    input         JSONB,
    result        JSONB,
    error         TEXT        NOT NULL DEFAULT '',
    start_time    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_time      TIMESTAMPTZ,
    version       INTEGER     NOT NULL DEFAULT 1
);

CREATE INDEX workflow_executions_status_start_idx
    ON workflow_executions (status, start_time DESC);

CREATE INDEX workflow_executions_name_idx
    ON workflow_executions (name);

-- Append-only history. idx is monotonic per workflow starting at 1.
CREATE TABLE workflow_events (
    workflow_id   TEXT        NOT NULL REFERENCES workflow_executions(workflow_id) ON DELETE CASCADE,
    idx           BIGINT      NOT NULL,
    event_type    TEXT        NOT NULL,
    timestamp     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    details       JSONB,
    PRIMARY KEY (workflow_id, idx)
);

CREATE INDEX workflow_events_timestamp_idx
    ON workflow_events (timestamp DESC);

-- Tasks queued for workers. kind is 'workflow' or 'activity'.
-- Phase 1 stores tasks for visibility; Phase 2 promotes the real queue
-- to Redis Streams and may drop this table or use it as the durable
-- fallback.
CREATE TABLE tasks (
    task_token         TEXT        PRIMARY KEY,
    workflow_id        TEXT        NOT NULL REFERENCES workflow_executions(workflow_id) ON DELETE CASCADE,
    kind               TEXT        NOT NULL CHECK (kind IN ('workflow', 'activity')),
    activity_name      TEXT,
    input              JSONB,
    status             TEXT        NOT NULL DEFAULT 'pending',
    claim_token        TEXT,
    visibility_timeout TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tasks_status_kind_idx
    ON tasks (status, kind, created_at);

-- Durable timers. fire_at is when the timer should fire; fired marks it
-- consumed. The partial index keeps the timer-loop scan small.
CREATE TABLE timers (
    timer_id     TEXT        PRIMARY KEY,
    workflow_id  TEXT        NOT NULL REFERENCES workflow_executions(workflow_id) ON DELETE CASCADE,
    fire_at      TIMESTAMPTZ NOT NULL,
    fired        BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX timers_pending_idx
    ON timers (fire_at)
    WHERE NOT fired;
