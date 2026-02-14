# Changelog

## [Unreleased] — Phase 2: Real Queue + Durable Timers

**Outcome:** workers in separate processes share work via a real
distributed queue, and timer state survives engine restarts. Workflow
crash-resume during `Sleep` still waits on Phase 3 (replay).

### Added
- `queue.Queue.Ack` and `queue.Queue.Reclaim`, plus a `Message`
  wrapper carrying an opaque ack token. `InMemoryQueue` keeps the
  same interface with no-op ack/reclaim semantics.
- `queue.RedisQueue`: Redis Streams backend with one stream per task
  queue and a shared consumer group ("sched"). Includes
  XPENDING+XCLAIM-based reclaim and a sensible default consumer name
  derived from `<hostname>-<pid>`.
- `store.Store.InsertTimer`, `FetchDueTimers` (FOR UPDATE SKIP LOCKED),
  and `ListPendingTimers`, with implementations in both
  `MemoryStore` and `PostgresStore`.
- Engine startup recovers pending timer rows via
  `Engine.TimerManager().RecoverPendingTimers`.
- Engine reclaim loop scans in-process pending task maps every 15s
  and re-enqueues entries past their 30s visibility timeout.

### Changed
- `EngineServer` now dispatches workflow tasks via
  `tasks:wf:<queue>` and activity tasks via `tasks:act:<queue>` on
  the configured queue backend. Completion handlers persist the
  WorkflowCompleted / ActivityCompleted events and XACK the stream
  entry inline; the old result-channel goroutine spawned by
  `StartWorkflow` is gone.
- `TimerManager.ScheduleTimer` now persists a `timers` row before
  returning, and the firing loop polls Postgres instead of an
  in-memory map. Callbacks are still local-only; recovered timers
  emit `TimerFired` events but no in-process side-effect (Phase 3).
- `cmd/engine` constructs the queue (`REDIS_ADDR` → RedisQueue,
  otherwise InMemoryQueue) and recovers timers on boot.
- `docker-compose.yml` drops the `worker` `container_name` so the
  service can be scaled with `--scale worker=N`.

### Known limitations (planned for Phase 3)
- `SDKWorkflowContext.Sleep` still calls `time.Sleep` on the worker,
  so a worker crash mid-sleep abandons the in-flight workflow. The
  engine-side durable Sleep RPC requires the replay machinery.
- Reclaim runs against the engine's in-process pending map; once
  Phase 4 ships shard ownership, reclaim will use XPENDING across
  the consumer group.

## [Unreleased] — Phase 1: Durable Single-Node

**Outcome:** workflow state survives engine restarts. Foundation laid for
the multi-phase rewrite described in
[`docs/PRD.md`](./docs/PRD.md).

### Added
- `internal/store` package with a `Store` interface, an in-memory
  implementation for tests, and a Postgres-backed implementation built
  on `pgx/v5` and `sqlc`-generated queries.
- Initial migration `000001_init.up.sql` defining
  `workflow_executions`, `workflow_events`, `tasks`, and `timers`
  tables, plus their indexes.
- `migrate` one-shot service in `docker-compose.yml` that applies
  migrations before the engine starts.
- Makefile targets: `migrate-up`, `migrate-down`, `migrate-new`,
  `sqlc-gen`.
- Engine configuration via `SCHED_POSTGRES_DSN` (in-memory fallback when
  unset).

### Changed
- `engine.Engine` no longer owns an in-process `map`-based persistence
  layer; it now accepts a `store.Store` via constructor injection.
- `EngineServer` handlers (`StartWorkflow`, `CompleteWorkflowTask`,
  `ScheduleActivity`, `SignalWorkflow`, `GetWorkflowStatus`,
  `ListWorkflows`, `GetWorkflowDetails`, `GetWorkflowMetrics`) persist
  state via `Store`.
- `TimerManager` records `TimerScheduled`/`TimerFired` events via
  `Store` instead of the old in-memory persistence layer.
- Engine Go base bumped to `golang:1.25-alpine` (also worker and
  dashboard images).

### Fixed
- `GetWorkflowMetrics` was comparing workflow status with mixed casing
  (`"Running"` vs `"RUNNING"`), so dashboard counters always read zero.
- `RunId` was generated in `StartWorkflow` but never persisted and never
  returned from `ListWorkflows` / `GetWorkflowDetails`; it now round-trips.
- Removed the misnamed `engine/presistence.go` (now `engine/persistence.go`).
