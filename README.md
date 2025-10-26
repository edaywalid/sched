# Sched

A lightweight distributed workflow orchestration engine built with Go, inspired by Temporal and Cadence.

## Overview

Sched is a workflow engine that provides:
- **Durable workflow execution** with state persistence
- **Activity scheduling** with retry policies
- **Timer management** for delayed execution
- **gRPC-based communication** between components
- **Redis-backed message queue** for reliable task distribution
- **PostgreSQL persistence** for workflow state
- **Web dashboard** for monitoring workflows

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
# Start all services
make up

# View logs
make logs

# Stop services
make down
```

The dashboard will be available at `http://localhost:8080`

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

- `ENGINE_PORT`: Engine gRPC port (default: 50051)
- `REDIS_ADDR`: Redis address (default: localhost:6379)
- `POSTGRES_HOST`: PostgreSQL host
- `DASHBOARD_PORT`: Dashboard web port (default: 8080)

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.


