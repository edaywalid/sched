# Changelog

## [Unreleased] Phase 3.1 through 3.3 and Phase 5.1, 5.2: Retries, Signals, Heartbeats, Observability

Activities retry with exponential backoff on failure. Signals are
deliverable end to end via the new `WaitForSignal` RPC. Long running
activities can heartbeat to extend their visibility timeout.
Structured logging via `slog` replaces the standard `log` package
across the engine. Prometheus metrics are exposed on `/metrics`.

### Added

- `proto.EngineService.WaitForSignal`. Blocking pull side of the
  signal API. Workers call `WorkflowContext.WaitForSignal(timeout)`
  to suspend until `SignalWorkflow` delivers a named signal.
- `proto.EngineService.RecordActivityHeartbeat`. Long running
  activities call `ActivityContext.Heartbeat(details)` periodically.
  The engine resets the pending task's `DequeuedAt` timestamp so the
  reclaim loop does not steal a still running task.
- `Engine.DeliverSignal` and `Engine.WaitForSignal`. Per workflow
  signal queues with sleeping waiters, replacing the half wired
  `engine.signals` map.
- `RetryPolicy.BackoffFor(attempt)`. Geometric backoff helper with
  defensive defaults and a `MaximumInterval` clamp.
- `EventActivityRetryScheduled` event type, surfaced in workflow
  history with the upcoming attempt number and delay.
- Worker demo workflows. `RetryDemo` schedules an always failing
  activity that exhausts the default 3 attempt retry policy.
  `SignalDemo` blocks on `WaitForSignal` until signalled. `LongDemo`
  runs a 35 second activity that heartbeats every five seconds.
- `internal/observability` package. `NewLogger` installs a `slog`
  logger as the package default, switching between text and JSON via
  `SCHED_LOG_FORMAT`. `NewMetrics` registers Prometheus counters and
  histograms under the `sched_` namespace. `StartMetricsServer`
  serves `/metrics` and `/healthz` on a configurable port (default
  9090).

### Changed

- `ScheduleActivity` stamps `attempt=1` plus the default
  `RetryPolicy` onto every activity envelope. `CompleteActivity` on
  failure writes `ActivityFailed`, then schedules a durable timer
  via `TimerManager` whose callback re enqueues the activity at
  `attempt+1`.
- `ActivityContext` is no longer the empty interface. The concrete
  SDK context exposes `TaskToken()` and `Heartbeat(details)`.
- `log.Printf` and `log.Fatalf` are gone from the engine binary and
  the engine package. `slog` carries structured attributes
  (`workflow_id`, `run_id`, `task_token`, `timer_id`) on every hot
  path line.
- `cmd/engine` boots an observability logger, registers metrics
  with promauto, and starts `/metrics` on port 9090 alongside the
  gRPC server.

### Added (Phase 5.8b)

- `EngineService.StreamActivityTasks` RPC. Mirrors
  `StreamWorkflowTasks` for activity task dispatch. SDK
  `Client.StartStreamingWorker` now opens both streams in
  parallel, so a worker started with `SCHED_WORKER_STREAMING=true`
  uses bidi streaming for every task type. `CompleteActivity`
  stays unary so retry counters and visibility-timeout
  bookkeeping continue to work exactly as before.
- Engine re-enqueue path on Send failure preserves the original
  envelope's Attempt, MaxAttempts, and RetryPolicy so retries
  resume from the right attempt number even after a stream
  drop.

### Added (Phase 5.7 and Phase 5.8)

- Phase 5.7: graceful SIGTERM / SIGINT handling in the engine.
  `engine.NewGRPCServer` exposes Serve and Stop so cmd/engine
  can drive a bounded shutdown. The drain cancels long-polling
  Dequeue calls via a server-lifetime context, runs
  GracefulStop, then force-stops after an 8 second deadline.
  `SCHED_SHUTDOWN_GRACE_SECONDS` overrides the deadline. Docker
  Compose declares a 15 second stop_grace_period so docker stop
  works without the operator setting it.
- Phase 5.8: bidi streaming for workflow task dispatch. New
  `StreamWorkflowTasks` RPC carries a oneof Subscribe / Ready
  client message and a stream of PollWorkflowTaskResponse from
  the server. SDK `Client.StartStreamingWorker` opens the stream
  and replaces the 60s long-poll round trip with a single
  long-lived stream plus per-task Ready credits. Workers opt in
  via `SCHED_WORKER_STREAMING=true`; activities still use the
  polling path until a matching streaming RPC lands.

### Added (Phase 3.4d)

- `EngineService.RegisterWorkflowTimer` RPC. The SDK calls it
  when a workflow function reaches Sleep with no matching
  `TimerScheduled` + `TimerFired` in history. The engine
  schedules a durable timer via TimerManager and arranges for
  the fire callback to re-dispatch the yielded workflow task.
- `replayState.findTimerScheduled` and
  `replayState.findTimerFired` plus the matching SDK Sleep path.
  Sleep now yields instead of blocking on `time.Sleep`, and the
  re-dispatched run short-circuits via replay.
- Engine bufconn test `TestDurableSleepRedispatch` covering the
  RegisterWorkflowTimer plus timer-fire re-dispatch round trip.

End-to-end with MonthlyReport: three `ActivityScheduled`,
`TimerScheduled`, `WorkflowTaskYielded`, `ActivityCompleted`,
`TimerFired` cycles followed by `WorkflowCompleted`. Each
re-dispatch replays the workflow function from scratch, and the
SDK skips every prior `QueueActivity` and `Sleep` via the cursor.

### Added (Phase 3.4b and Phase 3.4c)

- Phase 3.4b: SDK replay-state cursor over the workflow history.
  QueueActivity scans past the cursor for a matching
  ActivityScheduled and skips the RPC when found, so workflow
  re-runs do not duplicate side-effects on the engine.
  WaitForSignal applies the same pattern against SignalReceived
  and returns the recorded payload without re-blocking.
- Phase 3.4c: yield-based replay. CompleteWorkflowTaskRequest
  grows a `yielded` bool. The SDK panics with a yieldErr
  sentinel from WaitForSignal when no history match exists; the
  worker recovers, sets yielded=true on the completion, and frees
  the slot. The engine writes a WorkflowTaskYielded event, keeps
  the workflow RUNNING, and remembers the workflow as awaiting
  dispatch. When SignalWorkflow then lands a SignalReceived
  event, the engine re-marshals the workflow envelope and
  enqueues a fresh workflow task. The replayed function picks up
  the recorded signal via Phase 3.4b's cursor and continues.
- New event type: `WorkflowTaskYielded`.
- Engine bufconn test `TestYieldAndRedispatch` covering the full
  round trip.

### Added (Phase 5.6 and Phase 3.4a)

- Dashboard: Cancel button on the workflow detail page (only
  rendered when the workflow status is RUNNING), htmx-posted to a
  new `/api/cancel-workflow` proxy handler that calls
  `CancelWorkflow` on the engine.
- Dashboard: status filter chips above the workflow list for
  Running, Completed, Failed, Timed out, Canceled, and an All
  reset. Each chip swaps the inner content of the workflow list
  container without a full reload.
- Dashboard: an `execution_timeout_seconds` number input on the
  start-workflow form that propagates to
  `WorkflowExecutionTimeoutSeconds` on the StartWorkflow request.
- Dashboard: lower-case status CSS classes plus dedicated
  `status-timed_out` and `status-canceled` rules so the new
  Phase 3.5 states render with the right palette.
- Phase 3.4a: `PollWorkflowTaskResponse` gains a `history` field.
  The engine ships the workflow's full durable event log with
  every workflow task. The SDK logs `history=N` for now; real
  consumers land in Phase 3.4b.

### Added (Phase 3.5)

- `StartWorkflowRequest.workflow_execution_timeout_seconds`. When set,
  the engine arms a durable timer; if the workflow has not reached a
  terminal state by then, it is marked `TIMED_OUT` and a
  `WorkflowTimedOut` event lands in history.
- `EngineService.CancelWorkflow` RPC. Marks an in-flight workflow for
  cancellation and writes a `WorkflowCancelRequested` event. The
  in-process `cancelRequested` set drives the existing
  `cancel_requested` field on subsequent `RecordActivityHeartbeat`
  responses, so cooperative activities can return early via
  `ActivityContext.Heartbeat(...)`.
- `store.StatusCanceled` and the matching `EventWorkflowCanceled` /
  `EventWorkflowCancelRequested` / `EventWorkflowTimedOut` event types.
- Engine bufconn tests covering both flows
  (`TestWorkflowExecutionTimeout`, `TestCancelWorkflow`).

### Added (Phase 5.4)

- `internal/observability/tracing.go`. `InitTracing` installs the
  global OpenTelemetry tracer provider with the OTLP/gRPC exporter
  when `SCHED_OTLP_ENDPOINT` is set, or the noop tracer when it is
  not. The W3C `TraceContext` and `Baggage` propagators are set
  globally so trace context flows across processes.
- `otelgrpc` stats handler on the engine gRPC server, the SDK gRPC
  client, and the dashboard gRPC client. RPC spans link the three
  services automatically once a span is in flight.
- Explicit `workflow.<name>` and `activity.<name>` spans in the SDK
  worker loop, tagged with `workflow.id`, `workflow.run_id`,
  `activity.name`, and `activity.task_token`. Errors get
  `SetStatus(Error, msg)` and `RecordError`.
- `jaeger` service in `docker-compose.yml` behind a `tracing`
  profile. Run with `SCHED_OTLP_ENDPOINT=jaeger:4317 docker compose
  --profile tracing up -d` and open `http://localhost:16686`.

### Added (Phase 5.3)

- `.golangci.yml` enabling errcheck, govet, ineffassign,
  staticcheck, unused, gocritic, revive, and misspell. Pragmatic
  exclusions for sqlc generated code and the templ generated
  dashboard renderer.
- `make lint` target that auto installs `golangci-lint v2.7.2` on
  first use.
- `.github/workflows/ci.yml` running build, vet, race tests, and
  golangci-lint on push and PR. Postgres 16 and Redis 7 service
  containers let the integration tests in `internal/store` and
  `queue` run for real instead of skipping.
- `.github/dependabot.yml` for weekly Go module, Docker image,
  and GitHub Actions updates. Grouped PRs for OTel, Prometheus,
  and gRPC ecosystems.

### Deferred to Phase 3.4, Phase 4, Phase 5.4

- Event sourced workflow replay (workflow function as a pure
  function of input plus history). Without it, `Sleep` is still a
  local `time.Sleep` on the worker, so worker crashes during a
  sleep still abandon in flight workflows. The replay machinery is
  one large coordinated change and lands in its own focused effort.
- Workflow queries (`QueryWorkflow` RPC) and cooperative activity
  cancellation. The `cancel_requested` field on the heartbeat
  response is wire ready, but the engine never sets it yet.
- Sharded engine (Phase 4). Requires replay first.
- OpenTelemetry tracing across engine, worker, and dashboard
  (Phase 5.4).

## [Unreleased] Phase 2: Real Queue and Durable Timers

Workers in separate processes share work via a real distributed
queue. Timer state survives engine restarts. Workflow crash resume
during `Sleep` still waits on Phase 3.4 (replay).

### Added

- `queue.Queue.Ack` and `queue.Queue.Reclaim`, plus a `Message`
  wrapper carrying an opaque ack token. `InMemoryQueue` keeps the
  same interface with no op ack and reclaim semantics.
- `queue.RedisQueue`. Redis Streams backend with one stream per task
  queue and a shared consumer group (`sched`). Includes
  `XPENDING` and `XCLAIM` based reclaim, with a default consumer
  name derived from `<hostname>-<pid>`.
- `store.Store.InsertTimer`, `FetchDueTimers` (`FOR UPDATE SKIP LOCKED`),
  and `ListPendingTimers`, with implementations in both
  `MemoryStore` and `PostgresStore`.
- Engine startup recovers pending timer rows via
  `Engine.TimerManager().RecoverPendingTimers`.
- Engine reclaim loop scans in process pending task maps every 15
  seconds and re enqueues entries past their 30 second visibility
  timeout.

### Changed

- `EngineServer` now dispatches workflow tasks via
  `tasks:wf:<queue>` and activity tasks via `tasks:act:<queue>` on
  the configured queue backend. Completion handlers persist the
  `WorkflowCompleted` and `ActivityCompleted` events and `XACK` the
  stream entry inline. The old result channel goroutine spawned by
  `StartWorkflow` is gone.
- `TimerManager.ScheduleTimer` now persists a `timers` row before
  returning, and the firing loop polls Postgres instead of an in
  memory map. Callbacks are still local only; recovered timers
  emit `TimerFired` events but no in process side effect runs yet
  (closed in Phase 3.4).
- `cmd/engine` constructs the queue. `REDIS_ADDR` selects
  `RedisQueue`, otherwise the engine uses `InMemoryQueue`, and
  recovers timers on boot.
- `docker-compose.yml` drops the `worker` `container_name` so the
  service can be scaled with `--scale worker=N`.

## [Unreleased] Phase 1: Durable Single Node

Workflow state survives engine restarts. Foundation laid for the
multi phase rewrite described in [`docs/PRD.md`](./docs/PRD.md).

### Added

- `internal/store` package with a `Store` interface, an in memory
  implementation for tests, and a Postgres backed implementation
  built on `pgx/v5` and `sqlc` generated queries.
- Initial migration `000001_init.up.sql` defining
  `workflow_executions`, `workflow_events`, `tasks`, and `timers`
  tables, plus their indexes.
- `migrate` one shot service in `docker-compose.yml` that applies
  migrations before the engine starts.
- Makefile targets: `migrate-up`, `migrate-down`, `migrate-new`,
  `sqlc-gen`.
- Engine configuration via `SCHED_POSTGRES_DSN`. In memory fallback
  when unset.

### Changed

- `engine.Engine` no longer owns an in process map based persistence
  layer. It accepts a `store.Store` via constructor injection.
- `EngineServer` handlers (`StartWorkflow`, `CompleteWorkflowTask`,
  `ScheduleActivity`, `SignalWorkflow`, `GetWorkflowStatus`,
  `ListWorkflows`, `GetWorkflowDetails`, `GetWorkflowMetrics`)
  persist state via `Store`.
- `TimerManager` records `TimerScheduled` and `TimerFired` events
  via `Store` instead of the old in memory persistence layer.
- Engine Go base bumped to `golang:1.25-alpine`, along with the
  worker and dashboard images.

### Fixed

- `GetWorkflowMetrics` was comparing workflow status with mixed
  casing (`"Running"` vs `"RUNNING"`), so dashboard counters always
  read zero.
- `RunId` was generated in `StartWorkflow` but never persisted and
  never returned from `ListWorkflows` and `GetWorkflowDetails`. It
  now round trips.
- Removed the misnamed `engine/presistence.go` (now
  `engine/persistence.go`).
