# API Design

## 1) Overview

The backend API is built with **Goa v3**, a design-first framework for Go. The API design DSL in 
`/backend/internal/api/design/` is the source of truth for all endpoints, payloads, and responses. This document covers 
general approach and patterns rather than enumerating specific endpoints.

## 2) Design-first workflow

### 2.1 Source of truth

The Goa DSL files under `/backend/internal/api/design/` define:
- Service groupings and HTTP paths
- Method signatures (request/response types)
- Error types and HTTP status mappings
- Security requirements per endpoint
- CORS configuration

### 2.2 Code generation

```
backend/internal/api/design/   ← DSL definitions (hand-edited)
        │
        │  `make gen` (goa gen)
        ▼
backend/internal/api/gen/      ← Generated code (DO NOT EDIT)
```

- Generated code includes HTTP transport, encoding/decoding, and OpenAPI specs.
- Regenerate after any design change: `cd backend && make gen`.
- Mock generation (mockery) runs after Goa codegen when interfaces change.

### 2.3 Swagger / OpenAPI

- Swagger UI is hosted at `/docs` (served by the `docs` Goa service).
- The generated `openapi.json` is served alongside the Swagger UI assets.
- The Swagger UI provides interactive API documentation and testing.

### 2.4 Validation

- Use Goa's built in `Format()` directive to require correct formatting for request/response types.

## 3) API structure

### 3.1 Service grouping

The API is organized into Goa services, each mapping to a resource domain:

| Service     | Base Path    | Purpose                             |
|-------------|--------------|-------------------------------------|
| health      | /health      | Health check with subsystem status   |
| queues      | /v1/queues   | NATS JetStream queue admin           |
| settings    | /v1/settings | Secrets, config, provider testing    |
| exports     | /v1/exports  | Dataset export (server-side + zip)   |
| samples*    | /v1/samples  | Sample status updates (single + bulk)|
| edits*      | /v1/samples  | Edit operations persistence (CRUD)   |
| captions*   | /v1/samples  | Caption CRUD with study grouping      |
| media*      | /v1/media    | Raw image and thumbnail serving      |
| sse*        | /api/sse     | Server-sent events stream            |
| docs*       | /docs        | Swagger UI and OpenAPI spec          |

*Custom HTTP handlers (not Goa-generated) for non-JSON content types.

Each service corresponds to a file in the design package (e.g., `items.go`, `users.go`).

### 3.2 URL conventions

- Base path: `/v1/` (versioned API prefix)
- Resource collections: plural nouns (e.g., `/v1/foos`)
- Individual resources: collection + ID (e.g., `/v1/foos/{id}`)
- Actions: sub-paths where RESTful verbs don't suffice (e.g., `/v1/foos/import`)
- Standard HTTP methods: GET (read), POST (create), PUT (update), DELETE (remove)

## 4) Authentication and authorization
TBD...

## 5) Error handling

### 5.1 Error response type

All API errors use the `ErrorWithCode` type:

```
{
  "Code": "STABLE_ERROR_CODE",
  "Message": "Human-readable description"
}
```

- `Code` is a stable string for programmatic consumption by the frontend.
- `Message` is a sanitized, user-facing description.
- No secrets, stack traces, or internal details are exposed in error responses.

### 5.2 HTTP status mapping

Goa maps service errors to HTTP status codes in the design DSL. General conventions:

| Scenario              | HTTP Status | Error Code pattern     |
|-----------------------|-------------|------------------------|
| Validation failure    | 400         | `INVALID_*`            |
| Authentication needed | 401         | `UNAUTHORIZED`         |
| Resource not found    | 404         | `NOT_FOUND`            |
| Conflict (e.g., dupe) | 409         | `CONFLICT`             |
| Server error          | 500         | `INTERNAL_ERROR`       |
| Not implemented       | 501         | `NOT_IMPLEMENTED`      |

## 6) Request/response patterns

### 6.1 List endpoints

- Support filtering via query parameters (e.g., `?name=...` for foobars).
- Return arrays of resources.
- Pagination is optional for MVP but the design should accommodate it (offset/limit or cursor) if needed later.

### 6.2 Create/update endpoints

- Accept JSON request bodies.
- Return the created/updated resource.
- Validation errors return 400 with specific error codes.

### 6.3 Bulk operations

- JSON import accepts a JSON array of foobars matching the foobar API schema.
- Returns per-entry results (created/updated/skipped/error).

### 6.4 Send job lifecycle

- POST to create a send job (includes message version ID, recipient selection, medium overrides, dry-run flag).
- GET to check job status and retrieve stats.
- Job logs are retrieved via the history service.

## 7) CORS

- CORS is configured via `CORSMiddleware` in `internal/api/middleware.go`.
- Default origin: `http://localhost:3000` (override via `CORS_ORIGIN` env var).
- Supported methods: GET, POST, PUT, DELETE, OPTIONS.
- Credentials are allowed.
- Preflight (OPTIONS) returns 204 with appropriate headers.

## 7.1) Middleware stack

Applied in order (outermost first):
1. **CORSMiddleware** — CORS headers and OPTIONS preflight handling
2. **ErrorMiddleware** — Panic recovery, returns `ErrorWithCode` JSON
3. **RequestLoggingMiddleware** — Structured request logging; full detail at `LOG_LEVEL=debug`

## 7.2) SSE (Server-Sent Events)

- Endpoint: `GET /api/sse`
- Content-Type: `text/event-stream`
- Each event has a monotonically increasing `id` field
- Reconnection: send `Last-Event-ID` header to replay missed events from the in-memory ring buffer (capacity: 1024 events)
- Events are sourced from the pipeline `ChannelEventSink` (wired in `main.go`)
- Event types match PRD section 5.5 (job.state, job.progress, consumer.stats, etc.)

## 8) Content types

- **JSON** is the primary content type for all API requests and responses.
- **Multipart form data** is used for file uploads (attachments).
- **Binary** responses are used for attachment retrieval (with appropriate Content-Type headers).

## 9) Implementation pattern

The Goa-generated transport layer calls into hand-written service implementations:

```
HTTP Request
    │
    ▼
Goa Generated Handler (decode, validate)
    │
    ▼
API Implementation (internal/api/)
    │
    ▼
Service Layer (internal/service/)
    │
    ▼
Store / Provider (internal/store/)
    │
    ▼
HTTP Response ◀── Goa Generated Encoder
```

The API implementation files in `internal/api/` adapt between Goa-generated types and the service layer's domain model types.
