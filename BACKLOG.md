# Backlog

The complete state of sched, what is shipped, what is being worked on, and what is on deck. This is the canonical status document; the README links here.

Last reviewed: 2026-05-29.

## Shipped

### Engine

- Workflow + activity execution over a gRPC SDK
- Postgres-backed durable state (event-sourced history via `pgx/v5` + `sqlc`)
- Distributed task queue on Redis Streams with consumer groups and reclaim
- Durable timers (`ctx.Sleep` survives worker crash and engine restart)
- Activity retries with exponential backoff via `RetryPolicy.BackoffFor`
- Signals end-to-end: `SignalWorkflow` push and `ctx.WaitForSignal` pull
- `WaitForSignal` survives worker crash through yield-based replay
- Activity heartbeats with cancel-flag propagation
- Workflow cancellation (`CancelWorkflow` RPC)
- Workflow execution timeout (`workflow_execution_timeout_seconds` on `StartWorkflow`)
- Bidi-streamed workflow + activity task dispatch (`StreamWorkflowTasks`, `StreamActivityTasks`)
- Active-passive HA via Postgres advisory lock
- Graceful SIGTERM with configurable grace period

### Observability

- Structured logging via `slog` (`SCHED_LOG_FORMAT=json|text`, `SCHED_LOG_LEVEL=...`)
- Prometheus metrics on `:${SCHED_METRICS_PORT}/metrics` with counters and histograms
- OpenTelemetry tracing (OTLP/gRPC) with `otelgrpc` interceptors across engine, worker, dashboard
- `docker compose --profile tracing up` brings up Jaeger on `:16686`

### Web

- React 19 + Vite 6 + Tailwind v4 dashboard, embedded in the Go binary
- Sidebar shell, sectioned content, animated workflow rows, history timeline
- TanStack Start landing site (`web/apps/site`) with workflow demo, SDK tabs, FAQ
- MDX docs runtime with Shiki syntax highlighting and a custom code viewer
- 11 docs pages live: install, quickstart, workflows, activities, signals, timers, architecture overview, replay model, persistence, observability, HA, configuration reference
- Static prerender script for Netlify deploy
- Shared `@sched/design` package: brand tokens, fonts, `Logo`, Motion helpers

### Quality

- `golangci-lint` configuration committed
- GitHub Actions CI: build, vet, race-enabled tests against Postgres 16 + Redis 7
- Schema migrations via `golang-migrate` under `migrations/`

## In progress

These are the gating items for tagging `v0.1.0`. Each is bounded scope, ~half a day to a day of work.

- [ ] **Workflow queries** (`QueryWorkflow` RPC).
      Read-only replay-mode execution. The engine re-runs the workflow function against frozen history and invokes a registered query handler instead of writing new events. PRD section "Phase 3: Queries".
- [ ] **Activity start-to-close timeouts.**
      Per-activity timeout that, when exceeded, fails the attempt and triggers the retry policy. Today the only durable timeout is the workflow-level execution timeout.
- [ ] **README polish + repo housekeeping** for v0.1.0 release: changelog entry, release notes draft, license headers audit.

## Planned

Targeted for `v0.x` or beyond. Order is rough priority, not commitment.

- **Multi-active sharded engine** (PRD Phase 4.b).
  Sharded ownership: `hash(workflow_id) % N` decides which engine instance owns a workflow. Frontend forwards writes to the owner. Replaces active-passive with true horizontal scaling.
- **Per-activity retry policy.**
  `sdk.WithRetry(...)` registration option. Default policy stays as today. Required for production-grade activity tuning.
- **Workflow signals with named filters.**
  Today `WaitForSignal` returns the next signal regardless of name. A `WaitForSignalNamed(name, timeout)` would skip non-matching signals (buffering them for later) and remove the loop boilerplate.
- **History pagination.**
  Workflows that run for weeks accumulate large histories. Dispatching the full history every replay does not scale. Plan: continuation tokens, snapshotting at idempotent checkpoints, or both.
- **Postgres connection pool tuning + RDS-friendly defaults.**
  Document `pool_max_conns`, validate `pgbouncer` compatibility, ship reasonable defaults for prepared statements.
- **Sched CLI** (`schedctl`).
  Read workflow state, start workflows, send signals, cancel, all without the dashboard or `curl`. Useful for ops automation.

## Considered

Ideas worth thinking about but not committed.

- **Python and TypeScript SDKs.**
  The replay model is language-agnostic. A Python SDK would substantially expand the audience. Would require committing to a stable proto contract first.
- **Historical archival to object storage.**
  Move terminal workflows older than N days to S3-compatible storage. Today's recommendation is a manual `DELETE` cron.
- **Authn / authz on the gRPC API and dashboard.**
  Today the dashboard is unauthenticated; the recommendation is to put it behind oauth2-proxy or equivalent. Native auth would simplify deployment.
- **Workflow versioning.**
  Long-running workflows whose code changes mid-flight. Temporal's `GetVersion` model is the obvious reference; the tradeoff is significant SDK surface.
- **Cross-workflow signaling helpers.**
  `ctx.SignalChildWorkflow(...)` and similar. Today external code does this through the client.
- **Dashboard search + filtering.**
  Full-text search over workflow inputs and history details. Probably wants Postgres `tsvector`.
- **Pagefind search for the docs site.**
  Once the docs grow beyond what fits in the sidebar.
- **Blog with RSS** under `web/apps/site/blog/`.
  Posts on the replay model, the active-passive HA story, and similar. Foundation is already in `apps/site`.

## Won't do (for v2)

Out of scope, explicitly. Revisit only if the project's purpose changes.

- **Multi-region replication.**
  Single-region only. A multi-region story needs a different persistence model than a single Postgres.
- **Cross-cluster workflow continuation.**
  A workflow stays in the cluster it was started in.
- **A workflow query language beyond a single typed handler.**
  No SQL-over-history, no GraphQL.

## How this file changes

Move items between sections as work lands. Add items to "Considered" liberally; promoting to "Planned" should require a small write-up of why and when. The README does not duplicate this list; it links here.
