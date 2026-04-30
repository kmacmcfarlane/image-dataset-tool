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
| health      | /health      | Health check (e.g., for monitoring) |
| docs        | /docs        | Swagger UI and OpenAPI spec         |
| ...         | /...         | ETC...                              |

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

- CORS is configured in the API design DSL.
- Allows requests from the frontend origin.
- Supported methods: GET, POST, PUT, DELETE, OPTIONS.
- Credentials are allowed (for cookie-based session tokens if used).

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
