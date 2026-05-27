# PRD — Sched v2: Production-Grade, Sharded Workflow Engine

## Context

`sched` today is a single-process workflow engine with a clean gRPC contract and a templ-based dashboard, but the README oversells what the code actually does:

- **Persistence is in-memory** (`engine/presistence.go` — `map[string]*WorkflowExecution`). A process restart wipes every workflow.
- **The Redis queue is dead code.** `Engine.queue` is constructed but the gRPC server (`engine/api.go`) routes all tasks through in-process Go channels (`workflowTasksCh`, `activityTasksCh`). Workers in separate processes cannot share work.
- **Timers run on the worker** via `time.Sleep` inside `SDKWorkflowContext.Sleep` (`sdk/workflow.go:48`). A worker crash mid-sleep loses the workflow.
- **Retries don't exist.** `RetryPolicy` is defined in `engine/workflow.go` but no code references it.
- **Signals are half-wired.** `SignalWorkflow` pushes onto `engine.signals[wfID]`, but the map is never populated and there is no `WaitForSignal` API.
- **Metrics are silently wrong.** `GetWorkflowMetrics` switches on `"Running"/"Completed"/"Failed"` but persistence stores `"RUNNING"/"COMPLETED"/"FAILED"` — dashboard counters read zero.
- **No tests, no CI, no structured logging, no metrics, no tracing.**

We're committing to turning `sched` into a **production-grade, horizontally scalable, Temporal-style sharded workflow engine**. The proto and SDK are free to evolve (no external consumers yet). This PRD defines the target architecture and a phased delivery plan so the work is shippable in increments rather than a single big-bang rewrite.

## Goals

1. **Durability.** Every workflow state transition is persisted to Postgres before the RPC returns success. Restarting any process loses no work.
2. **Determinism & replay.** Workflows are event-sourced; on worker crash, history is replayed to reconstruct state. `Sleep`, `QueueActivity`, and signals are commands that produce history events, not side effects.
3. **True distribution.** Multiple engine replicas, multiple workers, a real task queue (Redis Streams or Postgres-backed) — not in-process channels.
4. **Sharding.** Workflows are partitioned across engine replicas by `hash(workflowID) % numShards`. Each shard is owned by exactly one engine instance at a time, coordinated via a lease (Postgres advisory lock or Redis lease).
5. **Failure handling.** Retry policies, activity heartbeats, workflow timeouts, signal delivery guarantees.
6. **Observability.** Structured logs (slog), Prometheus metrics, OpenTelemetry tracing across engine/worker/dashboard with trace propagation through gRPC.
7. **Quality gates.** Unit + integration tests, `golangci-lint`, GitHub Actions CI.

## Non-Goals (v2)

- Multi-region replication.
- Cross-cluster workflows.
- Long-term history archival (S3/GCS) — out of scope, can revisit in v3.
- Authn/authz on the gRPC API and dashboard. Listed as a follow-up; not blocking v2.
- Custom workflow query language. Basic queries only.
- SDKs in languages other than Go.

## Target Architecture

```
                  ┌──────────────┐
                  │  Dashboard   │  templ + htmx, gRPC client
                  └──────┬───────┘
                         │ gRPC (frontend service)
                         ▼
┌──────────────┐   ┌───────────────────────────────────┐   ┌──────────────┐
│   Workers    │──►│ Frontend  ─►  Matching  ─►  History │◄─│   Workers    │
│  (sdk.Client)│   │  (stateless)  (task routing) (shard-owned)│ (sdk.Client) │
└──────────────┘   └─────────────────┬─────────────────┘   └──────────────┘
                                     │
                       ┌─────────────┼─────────────┐
                       ▼             ▼             ▼
                  ┌────────┐   ┌──────────┐   ┌──────────┐
                  │Postgres│   │  Redis   │   │OTEL/Prom │
                  │(events,│   │ (streams │   │  (logs,  │
                  │ state, │   │  + lease │   │ metrics, │
                  │ shards)│   │  coord)  │   │  traces) │
                  └────────┘   └──────────┘   └──────────┘
```

Three logical engine services, deployable as one binary in v2.0 and splittable in later milestones:

- **Frontend** — public gRPC API (`StartWorkflow`, `Signal`, `Query`, dashboard reads). Stateless. Routes writes to the owning History shard.
- **Matching** — owns task queues (workflow task queue, activity task queue, per shard). Backed by Redis Streams with consumer groups so polling workers get exclusive delivery + acks.
- **History** — owns workflow state. Each shard is leased by exactly one history instance. Applies commands → appends events → persists in a single Postgres transaction → enqueues resulting tasks to Matching.

This mirrors Temporal's frontend/matching/history split but stays as one binary until phase 3.

## Phased Roadmap

Each phase produces a working, deployable system. Don't start phase N+1 until phase N's verification passes.

### Phase 1 — Durable Single-Node (foundation)
**Outcome:** Same external behavior as today, but state survives restarts and the code is structured for the future split.

- Replace `engine.PersistenceLayer` with a Postgres-backed implementation. Schema: `workflow_executions`, `workflow_events` (event-sourced history), `tasks` (workflow + activity), `timers`, `signals`.
- Introduce a `persistence.Store` interface so tests can use a fake; production uses pgx + sqlc-generated queries (per `golang-database` skill conventions).
- Migrations via `golang-migrate` checked into `migrations/`.
- Wire the engine to read/write history through the store for every state transition (`StartWorkflow`, `CompleteWorkflowTask`, `CompleteActivity`, `ScheduleActivity`, `SignalWorkflow`, timer fire).
- Fix the status-casing mismatch in `engine/api.go:386` (`GetWorkflowMetrics`).
- Fix `RunId` so it's persisted and returned (`engine/api.go:301,336`).
- Rename `engine/presistence.go` → `engine/persistence.go`.

**Critical files:** `engine/persistence.go`, `engine/engine.go`, `engine/api.go`, new `engine/store/` package, new `migrations/`.

**Reuse:** keep the `WorkflowEvent` shape; keep gRPC handler signatures (only their bodies change).

### Phase 2 — Real Distributed Queue + Durable Timers
**Outcome:** Multiple worker processes truly share work. Workflows survive worker crashes during `Sleep` and pending activities.

- Implement matching on Redis Streams with consumer groups (`XADD`, `XREADGROUP`, `XACK`). One stream per task queue: `tasks:wf:<queue>`, `tasks:act:<queue>`.
- Replace the in-memory channels in `EngineServer` (`workflowTasksCh`, `activityTasksCh`, `pendingWfTasks`, `pendingActTasks`) with stream-backed equivalents.
- `PollWorkflowTask` / `PollActivityTask` become `XREADGROUP BLOCK` calls.
- `CompleteWorkflowTask` / `CompleteActivity` do `XACK` + persist result in the same Postgres tx.
- Move `Sleep` to engine-side: `WorkflowContext.Sleep` now sends a `RegisterTimer` command. The engine writes a `TimerScheduled` event, sleeps server-side via `TimerManager`, then on fire writes `TimerFired` and re-enqueues the workflow task. `SDKWorkflowContext.Sleep` becomes a blocking RPC, not a local `time.Sleep`.
- Visibility timeout on stream entries: if a worker doesn't ack within N seconds, the message is reclaimed (`XPENDING`/`XCLAIM`).

**Critical files:** new `queue/streams.go` (replace `queue/queue.go`'s Redis impl), `engine/api.go`, `engine/timer.go` (now persists timers, recovers on startup), `sdk/workflow.go`.

**Reuse:** keep `queue.Queue` interface but extend with `Ack`, `Reclaim`. The in-memory impl stays for unit tests only.

### Phase 3 — Determinism, Replay, Retries, Signals, Queries
**Outcome:** Workflow re-execution from history; activity retries with backoff; complete signal API; basic queries.

- **Event sourcing for the workflow execution:** the engine, not the worker, is the source of truth for what the workflow has done. Worker dispatches a `WorkflowTask` carrying the *full history*; the SDK replays history while re-running the workflow function and only issues new commands (activity scheduling, timer registration, etc.) when execution moves past the last replayed event. This is the standard Temporal/Cadence pattern.
- Update `SDKWorkflowContext` to be a deterministic replay context. Commands (`QueueActivity`, `Sleep`, `WaitForSignal`) buffer their commands; the SDK returns the command buffer in `CompleteWorkflowTask` for the engine to apply atomically.
- Implement retries: when an activity fails, the engine consults the activity's `RetryPolicy` (use the struct already in `engine/workflow.go:7-21`), computes the next attempt's delay, and schedules a timer that re-enqueues the activity task.
- Complete the signal API: `SignalWorkflow` writes `SignalReceived` to history. On the next workflow task, the SDK exposes pending signals via `ctx.WaitForSignal(name)`. The engine ensures the workflow is woken if it's blocked on a signal.
- Workflow queries: read-only RPC `QueryWorkflow` that runs the workflow function in replay mode and invokes a registered query handler — no new history events.
- Activity heartbeats: `RecordActivityHeartbeat` RPC; engine resets the activity's heartbeat-timeout timer.

**Critical files:** `sdk/workflow.go` (largest delta), `sdk/communicator.go`, `engine/api.go`, new `engine/replay.go`, `engine/workflow.go`, proto additions in `proto/engine.proto`.

**Reuse:** existing `RetryPolicy` struct, existing event type constants (`EventTypeActivityScheduled`, etc.).

### Phase 4 — Sharding & Horizontal Scaling
**Outcome:** Multiple engine replicas. Each shard owned by exactly one replica. Workflows balanced across shards by hash.

- Introduce a `shards` table in Postgres: `(shard_id, owner_instance_id, lease_expires_at)`.
- Each engine instance acquires a configurable number of shards via Postgres advisory locks (`pg_try_advisory_lock`) with periodic lease renewal.
- All workflow state mutations route to the owner of `hash(workflowID) % numShards`. The Frontend service looks up the owner and forwards the RPC (in v2.0 the lookup is a static map; later, service discovery).
- Default `numShards = 1024`. Default replicas: configurable, deployment-time.
- Add a `shard_id` column to `workflow_executions`, `workflow_events`, `timers` for shard-scoped queries.
- TimerManager becomes shard-scoped — only fires timers for owned shards.

**Critical files:** new `engine/sharding/` package, `engine/api.go`, `engine/timer.go`, schema migration.

### Phase 5 — Observability, Quality, CI
**Outcome:** Engine, workers, and dashboard are instrumented. CI enforces tests and lint on every PR.

- **Logging:** replace `log.*` with `slog`. JSON handler in prod, text in dev. Per `golang-observability` skill conventions.
- **Metrics:** Prometheus client lib. Counters: `sched_workflows_started_total`, `sched_workflows_completed_total{status}`, `sched_activities_executed_total{status}`, `sched_task_poll_latency_seconds`, `sched_shard_owned_count`. Histograms for activity duration and task latency.
- **Tracing:** OTel SDK with OTLP exporter. Propagate trace context through gRPC (`otelgrpc` interceptor). Workflow/activity spans nested under the start RPC.
- **Tests:** table-driven unit tests for `persistence`, `timer`, `queue`. Integration tests using `testcontainers-go` for Postgres + Redis. SDK end-to-end test that starts an in-process engine and runs a workflow.
- **Lint:** `golangci-lint` with the config from the `golang-lint` skill. CI fails on lint errors.
- **CI:** GitHub Actions workflows for `lint`, `test`, `build`, `docker-publish`, plus Dependabot/Renovate per the `golang-continuous-integration` skill.

**Critical files:** `.github/workflows/*.yml`, `.golangci.yml`, new `*_test.go` siblings throughout, new `internal/observability/` package.

## Cross-Cutting Concerns

- **Proto evolution:** since we have permission to break the proto, regenerate cleanly. Add `version = 2` reserved fields where useful; bump `EngineService` to `EngineServiceV2` if signatures change drastically.
- **Configuration:** standardize on `spf13/viper` + env vars with `SCHED_` prefix. One config struct per binary.
- **Dependency injection:** use constructor injection (no global state) so tests can swap stores and queues. Optional: adopt `samber/do` for wiring if hand-rolled DI gets noisy.
- **Filename fix:** `engine/presistence.go` → `engine/persistence.go` early (phase 1).

## Deliverables Per Phase

Each phase ships:
1. Code + migrations on `main`.
2. Updated `docker-compose.yml` and `Makefile` targets.
3. Updated `README.md` reflecting what's actually true after the phase.
4. CHANGELOG entry.
5. Verification evidence (test runs, screenshots, traces).

## Verification

End-to-end checks per phase:

- **Phase 1:** `make up`; start a workflow via dashboard; `docker compose restart engine`; confirm the workflow row + history events are still queryable in Postgres and rendered in the dashboard.
- **Phase 2:** scale workers to 3 (`make scale N=3`); start 100 workflows; verify tasks are distributed across worker logs (no single worker takes all). Kill one worker mid-`Sleep`; confirm the workflow resumes on another worker after visibility timeout.
- **Phase 3:** start a workflow that runs an activity which fails 2 of 3 times; confirm history shows two `ActivityFailed` events followed by `ActivityCompleted`, with backoff intervals between them. Send a signal to a workflow waiting on `WaitForSignal`; confirm it unblocks. Run a query against a running workflow and confirm it returns without writing new history events.
- **Phase 4:** run two engine replicas; print which engine owns which shard ranges; restart one replica; confirm shards migrate to the survivor and back when the original returns.
- **Phase 5:** `make test` passes; `make lint` passes; CI green; open Grafana and see the dashboards populated; open the OTEL collector / Jaeger and see a full trace from `StartWorkflow` through activity execution.

## Resolved Decisions

- **Postgres driver + query layer:** `pgx/v5` + `sqlc`. Queries live in `internal/store/queries/*.sql`; generated code lands in `internal/store/db/`.
- **Migration tool:** `golang-migrate`. Migrations in `migrations/` as `NNNNNN_description.up.sql` / `.down.sql` pairs.
- **Dashboard reads:** through the Frontend gRPC service only. No direct DB access from the dashboard process.

## Open Questions (defer to the phase that needs them)

- Visibility timeout default for stream messages — 30s? Configurable per task queue? **Defer to Phase 2.**
- Should v2 keep `cmd/sdk` as a demo worker, or split demo workflows into `examples/`? **Defer to Phase 3.**
