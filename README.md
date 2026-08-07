# AeroXe ComplianceHub

> Phase 1 | AeroXe Ecosystem

---

## Table of Contents

- [What Is This Project?](#what-is-this-project)
- [Why Was It Built?](#why-was-it-built)
- [When Should You Use It?](#when-should-you-use-it)
- [Where Does It Run?](#where-does-it-run)
- [How Does It Work?](#how-does-it-work)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Service Modules](#service-modules)
- [Saga Orchestrator](#saga-orchestrator)
- [WebSocket Events](#websocket-events)
- [Database Schema](#database-schema)
- [Setup & Installation](#setup--installation)
- [Environment Variables](#environment-variables)
- [Development](#development)
- [Testing](#testing)
- [Deployment](#deployment)

---

## What Is This Project?

AeroXe ComplianceHub is a compliance tracking and regulatory management platform that monitors compliance requirements, schedules audits, tracks violations, and manages corrective actions.

---

## Why Was It Built?

Regulatory compliance is complex and ever-changing. Non-compliance results in fines and reputational damage. ComplianceHub automates compliance monitoring, audit scheduling, and violation tracking to ensure continuous compliance.

---

## When Should You Use It?

Use ComplianceHub for regulated industries (finance, healthcare, manufacturing), government agencies, or any organization with complex compliance requirements.

---

## Where Does It Run?

Backend runs Go/Hertz with PostgreSQL for compliance data, Redis for compliance status caching, NATS for compliance alert events. React web dashboard for compliance officers. WebSocket pushes compliance alerts and audit reminders.

---

## How Does It Work?

ComplianceHub implements ComplianceService, AuditService, RegulationService, ChecklistService, AlertService, ReportingService, AnalyticsService, and NotificationService. ComplianceCheck saga monitors compliance status against regulations. AuditExecution saga manages audit scheduling and execution. ViolationProcessing saga handles violation detection and corrective action tracking.

---

## Architecture

### High-Level Architecture

```
+---------------------------------------------------------------------+
|                          CLIENT LAYER                               |
|  +----------+  +------------------+  +-----------------------+     |
|  | React    |  | Android          |  | iOS                   |     |
|  | (Web)    |  | (Kotlin+Compose) |  | (SwiftUI)             |     |
|  +----+-----+  +--------+---------+  +----------+------------+     |
|       |                 |                        |                  |
+-------+-----------------+------------------------+------------------+
        |                 |                        |
        v                 v                        v
+---------------------------------------------------------------------+
|                       API GATEWAY (Hertz)                           |
|  +-------------+  +--------------+  +------------------------+     |
|  | HTTP REST   |  | gRPC Proxy   |  | WebSocket Hub          |     |
|  | Routes      |  | (grpc-gw)    |  | (coder/websocket)      |     |
|  +------+------+  +------+-------+  +----------+------------+     |
|         |                |                       |                  |
|  +------v----------------v-----------------------v----------+      |
|  |  Auth | Rate Limit | Circuit Breaker | Logging           |      |
|  +----------------------------------------------------------+      |
+-------------+-------------------+------------------+---------------+
              |                   |                  |
     +--------v--------+  +------v------+  +--------v--------+
     |  gRPC (sync)    |  |  NATS (async)|  |  WebSocket      |
     |  point-to-point |  |  pub/sub     |  |  real-time      |
     +--------+--------+  +------+-------+  +--------+--------+
              |                   |                  |
+-------------v-------------------v------------------v---------------+
|                  MODULAR MONOLITH BACKEND                          |
|  +------------+ +------------+ +------------+ +------------+     |
|  | Module A   | | Module B   | | Module C   | | Module D   |     |
|  +-----+------+ +-----+------+ +-----+------+ +-----+------+     |
|        |              |              |              |              |
|  +-----v--------------v--------------v--------------v------+      |
|  |  +----------+ +-------+ +--------+ +--------------+    |      |
|  |  | Postgres | | Redis | |  NATS  | | Saga Engine  |    |      |
|  |  +----------+ +-------+ +--------+ +--------------+    |      |
|  +---------------------------------------------------------+      |
+--------------------------------------------------------------------+
```

### Tech Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **HTTP Framework** | [Hertz](https://github.com/cloudwego/hertz) | High-performance HTTP server |
| **RPC** | [gRPC](https://grpc.io/) | Synchronous service-to-service |
| **Messaging** | [NATS](https://nats.io/) JetStream | Async event-driven messaging |
| **Database** | [PostgreSQL](https://www.postgresql.org/) 15+ | Primary data store |
| **Cache** | [Redis](https://redis.io/) 7+ | Caching, sessions, saga state |
| **WebSocket** | [coder/websocket](https://github.com/coder/websocket) | Real-time communication |
| **Frontend** | React 18 + TypeScript + Tailwind | Web application |
| **Android** | Kotlin + Jetpack Compose + Hilt | Android application |
| **iOS** | Swift 5.9+ + SwiftUI | iOS application |

---

## Service Modules

| Module | Description | Protocol |
|--------|-------------|----------|
| ComplianceService | Core compliance operations | gRPC + NATS |
| AuditService | Core audit operations | gRPC + NATS |
| RegulationService | Core regulation operations | gRPC + NATS |
| ChecklistService | Core checklist operations | gRPC + NATS |
| AlertService | Core alert operations | gRPC + NATS |
| ReportingService | Core reporting operations | gRPC + NATS |
| AnalyticsService | Core analytics operations | gRPC + NATS |
| NotificationService | Core notification operations | gRPC + NATS |

---

## Saga Orchestrator

| Saga | Pattern |
|------|---------|
| ComplianceCheck | Orchestrated via NATS + Redis state |
| AuditExecution | Orchestrated via NATS + Redis state |
| ViolationProcessing | Orchestrated via NATS + Redis state |
| CorrectiveActionFlow | Orchestrated via NATS + Redis state |

---

## WebSocket Events

| Event | Description |
|-------|-------------|
| `compliance_alert` | Real-time updates for compliance alert |
| `audit_scheduled` | Real-time updates for audit scheduled |
| `violation_detected` | Real-time updates for violation detected |
| `deadline_approaching` | Real-time updates for deadline approaching |

---

## Database Schema

| Table | Description | Key Fields |
|-------|-------------|------------|
| Compliance | `compliances` table | UUID, timestamps, soft delete |
| Audit | `audits` table | UUID, timestamps, soft delete |
| Regulation | `regulations` table | UUID, timestamps, soft delete |
| Checklist | `checklists` table | UUID, timestamps, soft delete |
| Alert | `alerts` table | UUID, timestamps, soft delete |
| Report | `reports` table | UUID, timestamps, soft delete |
| Violation | `violations` table | UUID, timestamps, soft delete |
| CorrectiveAction | `correctiveactions` table | UUID, timestamps, soft delete |
| Deadline | `deadlines` table | UUID, timestamps, soft delete |
| AuditLog | `auditlogs` table | UUID, timestamps, soft delete |

### Redis Usage

| Key Pattern | Purpose | TTL |
|------------|---------|-----|
| `session:<user_id>` | User session | 24h |
| `cache:<slug>:<id>` | Entity cache | 15m |
| `saga:<saga_id>` | Saga state | Until completion |
| `ratelimit:<ip>` | Rate limiting | 1m |

---

## Setup & Installation

### Prerequisites

- Go 1.22+
- PostgreSQL 15+
- Redis 7+
- NATS Server 2.10+ (with JetStream)
- Node.js 18+ (for React)
- Docker & Docker Compose (optional)

### Quick Start

```bash
# Clone the repository
git clone https://github.com/aeroxe/compliance-hub.git
cd compliance-hub

# Start infrastructure services
docker-compose up -d postgres redis nats

# Run database migrations
make migrate-up

# Seed initial data
make seed

# Start the Go server
make run

# In another terminal - start React frontend
cd clients/web
npm install
npm run dev
```

### Backend (Phase 1)

The backend lives in [`backend/`](backend/) — a Go + Hertz monolith backed by **GORM** for all persistence (no raw SQL is used anywhere in the codebase).

```bash
cd backend
cp .env.example .env          # adjust credentials if needed
docker compose up -d postgres redis nats   # start infra
make run                      # boots API + WebSocket + event bus
make migrate-up               # apply schema (also runs on server start)
make seed                     # insert sample records
make test                     # unit + handler tests
```

Layout:

| Path | Purpose |
|------|---------|
| `cmd/server` | HTTP API entrypoint (Hertz) |
| `cmd/migrate` | GORM AutoMigrate CLI |
| `cmd/seed` | Sample data seeder |
| `internal/models` | All 10 schema tables (UUID, timestamps, soft delete) |
| `internal/repository` | Generic cache-aware GORM CRUD |
| `internal/modules/*` | The 8 README services + violation / corrective-action / deadline |
| `internal/bus` | NATS event bus (in-process fallback) |
| `internal/cache` | Redis cache (in-memory fallback) |
| `internal/ws` | WebSocket hub (coder/websocket) |

Redis and NATS are optional at startup: when unreachable, the server transparently falls back to in-memory cache and in-process pub/sub so local development works without infrastructure.

### Docker Compose

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: compliance-hub
      POSTGRES_USER: aeroxe
      POSTGRES_PASSWORD: secret
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  nats:
    image: nats:2.10-alpine
    ports:
      - "4222:4222"
      - "8222:8222"
    command: ["-js"]

  app:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
      - nats
```

---

## Environment Variables

```bash
# Server
PORT=8080
ENV=development
LOG_LEVEL=debug

# PostgreSQL
DATABASE_URL=postgres://aeroxe:secret@localhost:5432/compliance-hub?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379

# NATS
NATS_URL=nats://localhost:4222

# JWT
JWT_SECRET=your-secret-key
JWT_EXPIRY=24h

# gRPC
GRPC_PORT=9090

# WebSocket
WS_MAX_CONNECTIONS=1000
WS_PING_INTERVAL=30s
```

---

## Development

### Makefile Targets

```makefile
run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

test:
	go test ./... -v -cover

migrate-up:
	go run cmd/migrate/main.go up

proto-gen:
	protoc --go_out=. --go-grpc_out=. proto/*.proto

lint:
	golangci-lint run
```

### React Development

```bash
cd clients/web
npm install
npm run dev
npm run build
npm test
```

---

## Testing

```bash
# Unit tests
go test ./internal/modules/... -v

# Integration tests
go test ./internal/modules/... -tags=integration -v

# Coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Deployment

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: compliance-hub
spec:
  replicas: 3
  selector:
    matchLabels:
      app: compliance-hub
  template:
    metadata:
      labels:
        app: compliance-hub
    spec:
      containers:
      - name: app
        image: aeroxe/compliance-hub:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

---

## License

Copyright (c) 2026 AeroXe Enterprises Private Limited. All rights reserved.

---

*Built with love by the AeroXe Team*
