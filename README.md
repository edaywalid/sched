# Sched

A workflow orchestration engine in Go, inspired by Temporal and Cadence.
Currently shipping toward production-grade per the
[`Sched v2` PRD](./docs/PRD.md).

## Status

| Capability                          | Today (v2 / Phase 1)                                  |
| ----------------------------------- | ----------------------------------------------------- |
| Workflow + activity execution       | ✅ Single engine, gRPC SDK, in-process task dispatch  |
| Workflow state persistence          | ✅ PostgreSQL (pgx/v5 + sqlc, event-sourced history)  |
| Web dashboard                       | ✅ templ + htmx, served by the dashboard service      |
| Distributed task queue              | ⏳ Phase 2 — Redis is provisioned but not on the path |
| Durable timers (survive crash)      | ⏳ Phase 2                                            |
| Replay, retries, signals, queries   | ⏳ Phase 3                                            |
| Sharded engine for HA               | ⏳ Phase 4                                            |
| Structured logs / metrics / tracing | ⏳ Phase 5                                            |

## Architecture

![Architecture Diagram](/assets/architecture.png)

The system consists of three main components:

- **Engine**: Central orchestrator managing workflows, activities, and timers via gRPC
- **SDK/Worker**: Executes workflow and activity logic, communicates with engine
- **Dashboard**: Web UI for monitoring and managing workflows

## How It Works

1. **Workflow Definition**: Define workflows as Go functions that orchestrate activities and timers
2. **Workflow Execution**: The SDK sends workflow start requests to the Engine via gRPC
3. **Task Distribution**: The Engine queues activities in Redis and workers subscribe to receive tasks
4. **Activity Execution**: Workers pull activities from the queue, execute them, and report results back
5. **State Persistence**: Workflow state and history are stored in PostgreSQL for durability
6. **Failure Handling**: Failed activities are automatically retried based on configured retry policies
7. **Monitoring**: The web dashboard provides real-time visibility into workflow executions

## Dashboard

The web dashboard provides a visual interface for monitoring your workflows:

![Dashboard Overview](/assets/dashboard-overview.png)
*Main dashboard showing active workflows and system status*

![Workflow Details](/assets/workflow-details.png)
*Detailed view of individual workflow execution and activity history*

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.24+ (for local development)

### Using Docker

```bash
# Start everything (also runs DB migrations as a one-shot service)
make up

# View logs
make logs

# Stop services
make down
```

The dashboard is at `http://localhost:8080`. Workflow state lives in
Postgres at `localhost:5432` (db `sched`, user `sched`).

### Database migrations

Migrations live in `migrations/` as `golang-migrate` files. The compose
stack runs them automatically via the `migrate` one-shot. To run them by
hand:

```bash
make migrate-up                   # apply pending migrations
make migrate-down                 # roll back the last migration
make migrate-new NAME=add_shards  # scaffold a new pair
```

To run the engine without Docker leave `SCHED_POSTGRES_DSN` unset — the
engine falls back to an in-memory store (state lost on restart).

### Local Development

```bash
# Install dependencies
make deps

# Generate protobuf files
make proto

# Run engine
make run-engine

# Run worker (in another terminal)
make run-worker
```

## Project Structure

```
cmd/
  ├── engine/      # Workflow engine server
  ├── dashboard/   # Web UI
  ├── sdk/         # Worker implementation
  └── test/        # Test workflows
engine/            # Core engine logic
sdk/               # Client SDK for workflows/activities
proto/             # gRPC protocol definitions
queue/             # Message queue implementation
```

## Usage Example

Define a workflow:

```go
func MyWorkflow(ctx sdk.WorkflowContext, input any) (any, error) {
    ctx.QueueActivity("processData", input)
    ctx.Sleep(10 * time.Second)
    ctx.QueueActivity("sendNotification", input)
    return "completed", nil
}
```

Register and start:

```go
client := sdk.NewClient("localhost:50051")
client.RegisterWorkflow("myWorkflow", MyWorkflow)
client.StartWorkflow("myWorkflow", myInput)
```

## Available Commands

```bash
make build          # Build Docker images
make up             # Start all services
make down           # Stop all services
make logs           # View logs
make scale N=3      # Scale workers
make test           # Run tests
make proto          # Generate protobuf files
make clean          # Clean up everything
```

## Configuration

Environment variables:

| Variable              | Component | Default                                                                | Purpose                                                       |
| --------------------- | --------- | ---------------------------------------------------------------------- | ------------------------------------------------------------- |
| `ENGINE_PORT`         | engine    | `50051`                                                                | gRPC listen port                                              |
| `SCHED_POSTGRES_DSN`  | engine    | _(unset → in-memory store)_                                            | Postgres connection string                                    |
| `ENGINE_ADDRESS`      | worker, dashboard | `localhost:50051`                                              | gRPC address of the engine                                    |
| `TASK_QUEUE`          | worker    | `default`                                                              | Task queue name to poll                                       |
| `DASHBOARD_PORT`      | dashboard | `8080`                                                                 | Web port                                                      |
| `AUTO_START_TEST`     | worker    | `false`                                                                | Demo mode: auto-register and start test workflows on startup  |

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.


