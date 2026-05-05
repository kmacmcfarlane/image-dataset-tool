.PHONY: claude claude-resume ralph ralph-resume ralph-auto ralph-auto-resume ralph-auto-debug ralph-debug capture-runtime-context up down logs up-dev down-dev logs-dev test-frontend test-frontend-watch test-backend test-backend-watch logs-snapshot test-e2e test-e2e-serial test-e2e-live test-e2e-live-run test-e2e-live-down

COMPOSE_DEV = docker compose -f docker-compose.yml -f docker-compose.dev.yml

claude:
	claude-sandbox --docker-socket

claude-resume:
	claude-sandbox --docker-socket --resume

ralph:
	claude-sandbox --docker-socket --ralph --interactive ${ARGS}

ralph-resume:
	claude-sandbox --docker-socket --ralph --interactive --resume ${ARGS}

ralph-auto:
	claude-sandbox --docker-socket --ralph --dangerously-skip-permissions ${ARGS}

ralph-auto-once:
	claude-sandbox --docker-socket --ralph --dangerously-skip-permissions --limit 1 ${ARGS}

# make ralph-auto-resume ARGS="<resume id>"
ralph-auto-resume:
	claude-sandbox --docker-socket --ralph --dangerously-skip-permissions --resume ${ARGS}

# make ralph-auto-resume-once ARGS="<resume id>"
ralph-auto-resume-once:
	claude-sandbox --docker-socket --ralph --dangerously-skip-permissions --limit 1 --resume ${ARGS}

# Debug: run the normal story pipeline with full decision logging (autonomous, single pass).
# After the run, review .ralph-debug/ for the full decision trail of every agent.
ralph-auto-debug:
	claude-sandbox --docker-socket --ralph --dangerously-skip-permissions --log-context --limit 1 ${ARGS}

# Debug: interactive version with full decision logging
ralph-debug:
	claude-sandbox --docker-socket --ralph --interactive --log-context ${ARGS}

# Capture runtime context snapshot (container logs, errors) to .debug-context
capture-runtime-context:
	./scripts/capture-runtime-context.sh

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

up-dev:
	$(COMPOSE_DEV) up -d --build

down-dev:
	$(COMPOSE_DEV) down -v

logs-dev:
	$(COMPOSE_DEV) logs -f

test-frontend:
	$(COMPOSE_DEV) exec frontend npm run test

test-frontend-watch:
	$(COMPOSE_DEV) exec frontend npm run test:watch

test-backend:
	$(COMPOSE_DEV) exec backend ginkgo -r --cover --race ./internal/... ./cmd/...

test-backend-watch:
	$(COMPOSE_DEV) exec backend ginkgo watch -r --cover --race ./internal/... ./cmd/...

logs-snapshot:
	$(COMPOSE_DEV) up -d --build && sleep 5 && $(COMPOSE_DEV) logs --no-color 2>&1 | head -200 && $(COMPOSE_DEV) down -v

COMPOSE_E2E = docker compose -f docker-compose.e2e.yml

# E2E tests via Docker (full isolated stack; Playwright runs in a container with all system deps).
# Run all: make test-e2e
# Run specific spec(s): make test-e2e-serial SPEC=settings.spec.ts
test-e2e:
	$(COMPOSE_E2E) up --build --abort-on-container-exit --exit-code-from e2e
	$(COMPOSE_E2E) down -v

test-e2e-serial:
	SPEC=$(SPEC) $(COMPOSE_E2E) up --build --abort-on-container-exit --exit-code-from e2e
	$(COMPOSE_E2E) down -v

# Live E2E development: hot-reload stack + interactive Playwright runner.
# Start: make test-e2e-live
# Run spec: make test-e2e-live-run SPEC=settings.spec.ts
# Stop: make test-e2e-live-down
test-e2e-live:
	$(COMPOSE_DEV) up -d --build

test-e2e-live-run:
	$(COMPOSE_DEV) exec frontend npx playwright test --workers=1 $(if $(SPEC),e2e/$(SPEC),)

test-e2e-live-down:
	$(COMPOSE_DEV) down -v
