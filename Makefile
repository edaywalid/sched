.PHONY: help build up down logs restart clean test proto migrate-up migrate-down migrate-new sqlc-gen lint

# Default Postgres DSN for local migrations (mirrors docker-compose.yml).
POSTGRES_DSN ?= postgres://sched:sched_password@localhost:5432/sched?sslmode=disable

# Default target
help:
	@echo "Sched Workflow Engine - Make Commands"
	@echo "======================================"
	@echo "  make build          - Build Docker images"
	@echo "  make build-frontend - Build React dashboard frontend"
	@echo "  make build-dashboard- Rebuild dashboard with new frontend"
	@echo "  make up             - Start all services"
	@echo "  make down           - Stop all services"
	@echo "  make logs           - View logs"
	@echo "  make restart        - Restart all services"
	@echo "  make clean          - Remove all containers and volumes"
	@echo "  make scale N=3      - Scale workers to N instances"
	@echo "  make proto          - Generate protobuf files"
	@echo "  make migrate-up     - Apply database migrations"
	@echo "  make migrate-down   - Roll back the last migration"
	@echo "  make migrate-new NAME=x - Create a new migration pair"
	@echo "  make sqlc-gen       - Regenerate sqlc query bindings"
	@echo "  make lint           - Run golangci-lint across the module"
	@echo "  make test           - Run tests"
	@echo "  make test-workflow  - Start test workflows"
	@echo "  make dev            - Run in development mode"

# Build Docker images
build:
	@echo "Building Docker images..."
	docker-compose build

# Build frontend
build-frontend:
	@echo "Building React frontend..."
	cd cmd/dashboard/frontend && pnpm install && pnpm run build

# Build and rebuild dashboard
build-dashboard: build-frontend
	@echo "Rebuilding dashboard service..."
	docker-compose build dashboard

# Start all services
up:
	@echo "Starting all services..."
	docker-compose up -d
	@echo "Services started!"
	@echo "Dashboard: http://localhost:8080"
	@echo "gRPC: localhost:50051"

# Start with logs
dev:
	@echo "Starting in development mode..."
	docker-compose up --build

# Stop all services
down:
	@echo "Stopping all services..."
	docker-compose down

# View logs
logs:
	docker-compose logs -f

# View engine logs
logs-engine:
	docker-compose logs -f engine

# View worker logs
logs-worker:
	docker-compose logs -f worker

# Restart services
restart:
	@echo "Restarting services..."
	docker-compose restart

# Scale workers
scale:
	@echo "Scaling workers to $(N) instances..."
	docker-compose up -d --scale worker=$(N)

# Clean up everything
clean:
	@echo "Cleaning up..."
	docker-compose down -v --rmi all
	@echo "Cleaned up all containers, volumes, and images"

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/engine.proto
	@echo "Protobuf files generated"

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Test workflows
test-workflow:
	@echo "Starting test workflows..."
	go run cmd/test/main.go

# Build Go binaries locally
build-local:
	@echo "Building engine..."
	go build -o bin/engine cmd/engine/main.go
	@echo "Building worker..."
	go build -o bin/worker cmd/sdk/main.go
	@echo "Binaries built in bin/"

# Run locally (no Docker)
run-engine:
	@echo "Starting engine locally..."
	go run cmd/engine/main.go

run-worker:
	@echo "Starting worker locally..."
	go run cmd/sdk/main.go

# Docker operations
ps:
	docker-compose ps

stats:
	docker stats

# Health checks
health:
	@echo "Checking service health..."
	@echo "Redis:"
	@docker-compose exec redis redis-cli ping || echo "Redis not responding"
	@echo "PostgreSQL:"
	@docker-compose exec postgres pg_isready -U sched || echo "PostgreSQL not responding"
	@echo "Engine:"
	@curl -s http://localhost:8080 > /dev/null && echo "Engine responding" || echo "Engine not responding"


# Install dependencies
deps:
	go mod download
	go mod tidy

# Apply all pending database migrations.
migrate-up:
	@echo "Applying migrations to $(POSTGRES_DSN)..."
	migrate -path migrations -database "$(POSTGRES_DSN)" up

# Roll back the most recent migration.
migrate-down:
	@echo "Rolling back one migration on $(POSTGRES_DSN)..."
	migrate -path migrations -database "$(POSTGRES_DSN)" down 1

# Scaffold a new migration pair. Usage: make migrate-new NAME=add_shards
migrate-new:
	@test -n "$(NAME)" || (echo "NAME is required, e.g. make migrate-new NAME=add_shards" && exit 1)
	migrate create -ext sql -dir migrations -seq $(NAME)

# Regenerate sqlc query bindings under internal/store/db/.
sqlc-gen:
	cd internal/store && sqlc generate

# Run golangci-lint against the whole module. Installs the linter on
# first use if it is not already on PATH.
lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint v2.7.2"; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2; \
	fi
	golangci-lint run ./...
