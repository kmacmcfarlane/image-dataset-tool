# local-web-app

A template for local-first web applications. Runs on Linux via Docker and provides a browser-based UI backed by a Go API server.

## Tech stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25, Goa v3 |
| Frontend | Vue 3 (Composition API), Vite, TypeScript |
| Testing | Ginkgo/Gomega (backend), Vitest (frontend) |
| Infrastructure | Docker Compose, multi-stage builds |

## Quick start

**Prerequisites:** Docker and Docker Compose.

```bash
make up
```

Open [http://localhost:3002](http://localhost:3002) in your browser (or `http://image-dataset-tool.mcfacehead.com` on the McFacehead LAN).

To stop:

```bash
make down
```

## Development

Start the dev stack with hot reload (backend via air, frontend via Vite HMR):

```bash
make up-dev
```

Watch tests continuously:

```bash
make test-backend-watch    # Ginkgo watch in docker
make test-frontend-watch   # Vitest watch in docker
```

Other targets:

```bash
make logs          # Tail operational logs
make logs-dev      # Tail dev logs
make down-dev      # Stop dev stack
```

### Backend commands (from repo root)

```bash
cd backend && make gen      # Goa codegen
cd backend && make build    # Build binary
cd backend && make lint     # Go vet
cd backend && make test     # Run tests
cd backend && make run      # Build and run
```

### Frontend commands (from repo root)

```bash
cd frontend && npm ci              # Install dependencies
cd frontend && npm run dev         # Vite dev server
cd frontend && npm run build       # Production build
cd frontend && npm run lint        # ESLint
cd frontend && npm run test:watch  # Vitest watch
```

## Project structure

```
your-project/
├── backend/
│   ├── cmd/server/           # Entrypoint (wiring only)
│   ├── internal/
│   │   ├── model/            # Domain structs
│   │   ├── service/          # Business logic
│   │   ├── store/            # Persistence + external resources
│   │   └── api/
│   │       ├── design/       # Goa DSL definitions
│   │       └── gen/          # Generated code (DO NOT EDIT)
│   ├── Dockerfile            # Production image
│   └── Dockerfile.dev        # Dev image with air hot reload
├── frontend/
│   ├── src/
│   │   ├── api/              # Backend API client modules
│   │   ├── components/       # UI components
│   │   └── views/            # Route-level pages
│   ├── Dockerfile            # nginx production image
│   └── Dockerfile.dev        # Vite dev server
├── docs/                     # Architecture, database, and API docs
├── scripts/                  # Tooling scripts (MCP servers)
├── docker-compose.yml        # Production compose
├── docker-compose.dev.yml    # Dev overlay
├── Makefile                  # Root orchestration targets
└── CHANGELOG.md
```

## Architecture

The backend enforces strict separation of concerns:

- **model** — Domain structs shared across layers (no serialization tags)
- **service** — Business logic depending on interfaces
- **store** — Persistence and external resource access
- **api** — Goa design-first transport glue and implementation

The frontend isolates all backend communication through `src/api/` modules. UI components never construct fetch requests directly.

Data flows: **Browser &rarr; Frontend (Vue) &rarr; Backend API (Goa) &rarr; Service &rarr; Store**

For full details see [docs/architecture.md](docs/architecture.md), [docs/database.md](docs/database.md), and [docs/api.md](docs/api.md).

## API documentation

The backend serves interactive Swagger UI at [http://localhost:8082/docs](http://localhost:8082/docs) with an OpenAPI 3.0 spec.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Backend server port |

## Testing

Backend tests use Ginkgo/Gomega and run inside the dev container (Go is not required on the host):

```bash
make test-backend-watch
```

Frontend tests use Vitest and can run directly if Node.js 22 is available:

```bash
cd frontend && npx vitest run
```

## Development tooling

I develop this project with the Claude Code tooling at [github.com/kmacmcfarlane](https://github.com/kmacmcfarlane); contributors are welcome to use the same setup, but nothing here requires it.

## License

This project is licensed under the [GPL-3.0](../LICENSE).
