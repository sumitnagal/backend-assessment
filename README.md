# IoT Platform Backend Assessment

## Overview
This assessment evaluates your ability to work with a complex, production-grade backend system for IoT device management. You'll be debugging, extending, and optimizing a Go-based REST API server that manages gateways, applications, users, and organizations across thousands of devices.

The assessment is based on a simplified version of our production IoT platform that handles device provisioning, application deployment, health monitoring, and real-time communication with edge devices.

## Assessment Goals
- Demonstrate proficiency with Go backend development
- Show ability to work with REST APIs, databases, and message queues
- Debug complex distributed system issues
- Implement scalable backend features
- Write production-quality code with proper testing
- Demonstrate understanding of authentication/authorization
- Show competency with database design and optimization

## Time Expectation
Take 4-6 hours to complete the core requirements. Additional time can be spent on bonus features and code quality improvements.

## System Architecture Overview

The platform consists of several key components:

### Core Services
- **REST API Server** - Main HTTP server handling client requests
- **Gateway Communication** - Real-time messaging with IoT devices
- **Message Processors** - Background workers processing device events
- **Authentication Service** - JWT-based auth with role-based access control
- **Health Monitoring** - Device and site health tracking system

### Database Schema
- **Organizations** - Customer tenants with isolated data
- **Users** - With role-based permissions per organization
- **Sites** - Physical locations containing gateways
- **Gateways** - IoT devices that run applications
- **Applications** - Software packages deployed to gateways
- **Bundles** - Groups of applications assigned to gateways via tags

## Setup Instructions

### Prerequisites
- Go 1.21 or later
- PostgreSQL 13+ (Docker recommended)
- Docker and Docker Compose
- Git configured with your name and email
- Basic familiarity with REST APIs and SQL

### Installation
```bash
cd backend-assessment
go mod tidy
make setup     # Sets up test database
make build     # Builds all binaries
make test      # Runs test suite
```

### Optional: Distributed Tracing (Jaeger)
```bash
# Start Jaeger
docker-compose up -d jaeger

# Run server with tracing env vars
export TRACING_ENABLED=true
export JAEGER_ENDPOINT=http://127.0.0.1:14268/api/traces
export SERVICE_NAME=backend-assessment
export ENVIRONMENT=dev
make run

# Open Jaeger UI
open http://localhost:16686
```
The server instruments HTTP endpoints via OpenTelemetry and emits spans for background worker processing.

### Reliability: Rate Limiting and Circuit Breaker

Per-user rate limiting (token bucket) is applied to sensitive endpoints like reboot.

Configure via env:
```bash
export RATE_LIMIT_RPM=600     # tokens per minute per user
export RATE_LIMIT_BURST=50    # burst tokens per user
export CB_FAILURE_THRESHOLD=5 # opens after 5 consecutive failures
export CB_RESET_TIMEOUT_SEC=30
export TRACING_ENABLED=true
export JAEGER_ENDPOINT=http://127.0.0.1:14268/api/traces
export SERVICE_NAME=backend-assessment
export ENVIRONMENT=dev

make run
```

Metrics:
```bash
curl http://localhost:8080/debug/vars | jq '.rate_limit_dropped'
```

Circuit breaker + retry usage example:
```go
cb := reliability.DefaultBreaker()
cfg := reliability.DefaultRetry()
err := cb.DoWithRetry(r.Context(), cfg, func(e error) bool {
    // decide retriable errors
    return true
}, func(ctx context.Context) error {
    // external call
    return nil
})
if err != nil {
    // handle failure
}
```

### Database Setup
```bash
# Start PostgreSQL with docker-compose
docker-compose up -d postgres

# Run migrations
make migrate

# Seed test data
make seed
```

### Git Workflow Requirements
We evaluate your git commit history as part of the assessment.

#### Initial Setup
```bash
git init
git add .
git commit -m "Initial commit: Add assessment scaffold"
```

#### Working Process
- Make atomic commits for each bug fix or feature
- Write clear, descriptive commit messages explaining the "why"
- Commit frequently (after each logical change)
- Use conventional commit format when appropriate

## Tasks

### Task 1: Fix Critical Production Bugs (Required)

The system has several production issues that need immediate attention:

#### Bug 1: Memory Leak in Message Processor  
**Location**: `applications/messageprocessor/worker.go`  
**Issue**: Database connections not properly closed, causing memory leaks  
**How to test**: Run with memory profiling and monitor connection pool usage

#### Bug 2: Deadlock in Health Monitoring
**Location**: `applications/edgehealth/processor.go`  
**Issue**: Concurrent access to shared state causes deadlocks under load  
**How to test**: Run concurrent health processing operations and use `go test -race`

#### Bug 3: Race Condition in Gateway Cache
**Location**: `internal/endpoints/gateways.go`  
**Issue**: Concurrent map read/write operations causing runtime panics  
**How to test**: Make concurrent requests to gateway endpoints and use `go test -race`

**Deliverable**: Fix all bugs with individual commits and descriptive messages.

***command***
``` 
seq 1 50 | xargs -P 20 -I{} curl -sS -H 'X-User-ID: test' 'http://localhost:8080/v1/gateways?search=a'
```

***Load test ***
```
./bin/cli worker-load --rate 50 --duration 20 --queue 1000
```

### Task 2: Production Readiness Features (Choose 1 of 3)

#### Option A: Distributed Tracing

**Requirement**: Implement OpenTelemetry distributed tracing
- Add trace context propagation across service boundaries
- Instrument critical code paths (API handlers, database queries)
- Include trace IDs in structured logging
- Add Jaeger exporter configuration

*** setup ***
```
 export TRACING_ENABLED=true
export JAEGER_ENDPOINT=http://127.0.0.1:14268/api/traces
export SERVICE_NAME=backend-assessment
export ENVIRONMENT=dev
go mod tidy
make run
```


#### Option B: Rate Limiting & Circuit Breaker

**Requirement**: Implement production-grade reliability patterns
- Add per-user rate limiting for API endpoints
- Implement circuit breaker for external service calls
- Add retry logic with exponential backoff
- Include proper metrics and alerting hooks

*** Setup ***
```
export RATE_LIMIT_RPM=10
export RATE_LIMIT_BURST=2
make run
```

```
seq 1 20 | xargs -P 10 -I{} curl -s -o /dev/null -w '%{http_code}\n' -H 'X-User-ID: u1' -X POST http://localhost:8080/v1/gateways/30000000-0000-0000-0000-000000000001/reboot
```

```bash
export RATE_LIMIT_RPM=600     # tokens per minute per user
export RATE_LIMIT_BURST=50    # burst tokens per user
export CB_FAILURE_THRESHOLD=5 # opens after 5 consecutive failures
export CB_RESET_TIMEOUT_SEC=30
export TRACING_ENABLED=true
export JAEGER_ENDPOINT=http://127.0.0.1:14268/api/traces
export SERVICE_NAME=backend-assessment
export ENVIRONMENT=dev

make run
```

Metrics:
```bash
curl http://localhost:8080/debug/vars | jq '.rate_limit_dropped'
```


#### Option C: Multi-tenant Data Isolation

**Requirement**: Enhance tenant isolation and compliance
- Implement row-level security for sensitive data
- Add data encryption at rest for PII fields
- Implement audit logging for all data access
- Add tenant data export functionality for GDPR compliance

**Deliverable**: Production-ready implementation with appropriate testing.

### Task 3: Technical Writing Sample (Required)

#### Professional Technical Documentation

**Requirement**: Write a professional technical document (800-1200 words) describing ONE of the following:

**Option A: Architectural Decision Record (ADR)**
- Document a significant technical decision from this assessment OR a previous project
- Include context, alternatives considered, decision rationale, and consequences
- Follow ADR format: Problem → Decision → Rationale → Trade-offs → Outcomes

**Option B: Technical System Overview**
- Provide a comprehensive technical overview of a system you've designed or implemented
- Can be from this assessment or any previous project you've worked on
- Include architecture diagrams, data flow, key design decisions
- Cover performance characteristics, reliability, security considerations

**Option C: Technical Deep Dive**
- Deep technical analysis of a complex problem you've solved
- Can be from this assessment (e.g., the deadlock fix) or any challenging technical problem from your experience
- Explain root cause analysis, solution approach, implementation details
- Include code examples, testing strategy, and lessons learned

**Requirements**:
- Professional technical writing appropriate for engineering team documentation
- Clear explanations suitable for both senior engineers and technical management
- Include relevant diagrams, code snippets, or data where appropriate
- Demonstrate systems thinking and production engineering mindset

**Evaluation Focus**:
- Technical depth and accuracy
- Clear communication of complex concepts
- Professional documentation standards
- Strategic thinking and trade-off analysis

**Deliverable**: Standalone technical document in markdown format.

## Git Commit Best Practices

### Commit Message Format
```
<type>(<scope>): <short description>

<detailed explanation of what and why>

<footer with issue references or breaking changes>
```

### Commit Types
- `fix:` Bug fixes
- `feat:` New features  
- `perf:` Performance improvements
- `security:` Security fixes
- `refactor:` Code refactoring
- `test:` Adding or updating tests
- `docs:` Documentation changes

### Examples
```
fix(auth): prevent gateway authentication bypass

Added missing organization ID validation in gateway endpoints.
Gateways can now only access resources within their own organization.

Fixes critical security vulnerability in Task 1.
```

```
feat(api): implement bulk gateway operations endpoint

- Add POST /v1/gateways/bulk/update for batch updates
- Include async operation tracking with UUIDs
- Add proper authorization and input validation
- Support up to 1000 gateways per operation

Implements bulk operations requirement.
```

## Submission Guidelines

### What to Submit
1. Complete source code with all implementations
2. Clean git history with meaningful commit messages  
3. **Technical Writing Sample** (800-1200 words): Choose one of:
   - Architectural Decision Record for a major technical decision
   - Technical System Overview of a component you implemented  
   - Technical Deep Dive on a complex problem you solved
4. **Assessment Summary** (500-750 words) covering:
   - Bugs found and fix approaches
   - Key architecture decisions and trade-offs
   - Performance optimizations implemented
   - Security considerations and mitigations
   - Areas for future improvement

### How to Submit
```bash
# Ensure all changes are committed
git status

# Run full test suite
make test-all

# Generate coverage report
make coverage

# Create submission archive with git history
tar -czf backend-assessment-submission.tar.gz \
    --exclude='bin/' \
    --exclude='coverage/' \
    --exclude='vendor/' \
    .
```

**IMPORTANT**: Include the `.git` directory for commit history review.

## API Documentation

The system uses OpenAPI/Swagger for API documentation:
```bash
make docs    # Generate API documentation
make serve-docs    # Serve docs at http://localhost:8080
```

Review existing endpoints at `/docs` for patterns and conventions.

## Database Schema

Key database tables and relationships:
```sql
-- Organizations are the top-level tenant boundary
organizations (id, name, settings)

-- Users belong to organizations with roles
users (id, email, organization_id)
user_org_roles (user_id, organization_id, role)

-- Sites group gateways by location
sites (id, name, organization_id, location)

-- Gateways are the IoT devices
gateways (id, serial, site_id, last_seen, health_status)

-- Applications deployed to gateways
apps (id, name, organization_id)
app_revisions (id, app_id, version, package_url)

-- Bundles group apps for deployment
bundles (id, name, organization_id)
bundle_apps (bundle_id, app_id, app_revision_id)
```

## Getting Help
- Review Go documentation: https://golang.org/doc/
- PostgreSQL docs: https://www.postgresql.org/docs/
- OpenAPI specification: https://swagger.io/specification/
- For clarification questions: Include in your writeup

## Bonus Points
- Implement comprehensive monitoring with Prometheus metrics
- Add Docker containerization with multi-stage builds
- Create Kubernetes deployment manifests
- Implement graceful shutdown and health check endpoints
- Add performance benchmarks with pprof profiling
- Exceptional git workflow with detailed commit messages
- Find and report additional bugs or security issues
- Implement additional security hardening measures

Good luck! This assessment reflects real-world backend engineering challenges you'd encounter in our production environment.
