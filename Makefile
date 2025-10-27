# Backend Assessment Makefile

.PHONY: setup build test test-all coverage clean migrate seed run lint docs serve-docs

# Build configuration
BUILD_DIR := bin
MAIN_PACKAGE := ./cmd/server
CLI_PACKAGE := ./cmd/cli

# Database configuration
DB_HOST := 127.0.0.1
DB_PORT := 5432
DB_USER := postgres
DB_PASSWORD := postgres
DB_NAME := backend_assessment_test
DATABASE_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

# Default target
all: build

# Setup development environment
setup:
	@echo "Setting up development environment..."
	go mod download
	go mod tidy
	docker-compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if docker exec backend-assessment-postgres pg_isready -U postgres > /dev/null 2>&1; then \
			echo "PostgreSQL is ready!"; \
			break; \
		fi; \
		echo "Waiting for PostgreSQL ($$i/10)..."; \
		sleep 2; \
	done
	make migrate
	@echo "Setup complete!"

# Build all binaries
build:
	@echo "Building binaries..."
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/server $(MAIN_PACKAGE)
	go build -o $(BUILD_DIR)/cli $(CLI_PACKAGE)
	@echo "Build complete!"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race -v ./...

# Run all tests including integration tests
test-all: test-race
	@echo "Running integration tests..."
	go test -tags=integration -v ./...

# Generate test coverage report
coverage:
	@echo "Generating coverage report..."
	mkdir -p coverage
	go test -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated at coverage/coverage.html"

# Run database migrations
migrate:
	@echo "Running database migrations..."
	go run ./cmd/migrate up
	@echo "Migrations complete!"

# Seed test data
seed:
	@echo "Seeding test data..."
	go run ./cmd/seed
	@echo "Test data seeded!"

# Run the server locally
run: build
	@echo "Starting server..."
	DATABASE_URL=$(DATABASE_URL) ./$(BUILD_DIR)/server

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf coverage
	@echo "Clean complete!"

# Docker commands
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-clean:
	docker-compose down -v

# Help
help:
	@echo "Available targets:"
	@echo "  setup     - Set up development environment"
	@echo "  build     - Build all binaries"
	@echo "  test      - Run tests"
	@echo "  test-race - Run tests with race detection"
	@echo "  test-all  - Run all tests including integration"
	@echo "  coverage  - Generate test coverage report"
	@echo "  migrate   - Run database migrations"
	@echo "  seed      - Seed test data"
	@echo "  run       - Run server locally"
	@echo "  lint      - Run linter"
	@echo "  clean     - Clean build artifacts"
	@echo "  docs      - Validate/generate API docs"
	@echo "  serve-docs- Serve Swagger UI at http://localhost:8081"

# Docs
docs:
	@echo "Validating OpenAPI spec..."
	@test -f docs/openapi.yaml || (echo "docs/openapi.yaml not found" && exit 1)
	@echo "OpenAPI spec present at docs/openapi.yaml"

serve-docs:
	@echo "Serving docs at http://localhost:8081"
	@which python3 >/dev/null 2>&1 || (echo "python3 not found" && exit 1)
	cd docs && python3 -m http.server 8081