# Changelog

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
