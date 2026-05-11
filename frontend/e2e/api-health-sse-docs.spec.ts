import { test, expect } from '@playwright/test'
import { bypassSetupWizard } from './helpers'

/**
 * E2E tests for S-006: API server skeleton — health, SSE, docs endpoints.
 *
 * These tests verify the backend API directly.
 * In Docker Compose E2E, the backend service is named 'backend' on port 8080.
 * /health and /docs are NOT proxied through the frontend Vite dev server,
 * so those tests use Playwright's `request` fixture (Node.js HTTP, not browser).
 *
 * /api/sse is proxied at baseURL/api/sse → backend:8080/api/sse.
 * SSE tests use page.evaluate with same-origin EventSource.
 */

// Derive the backend URL from Playwright's baseURL config.
// In Docker Compose E2E, baseURL is http://frontend:3000 → backend is http://backend:8080.
// Locally, baseURL is http://localhost:3000 → backend is http://localhost:8080.
function backendURL(baseURL: string | undefined): string {
  return (baseURL ?? 'http://localhost:3000').includes('frontend')
    ? 'http://backend:8080'
    : 'http://localhost:8080'
}

// ---------------------------------------------------------------------------
// GET /health — accepts 200 (all healthy) or 503 (subsystem failing).
// The key requirement is: the response includes a 'status' field and a
// 'subsystems' map with entries for all four subsystems.
// ---------------------------------------------------------------------------

test('GET /health responds with status code 200 or 503', async ({ request, baseURL }) => {
  const resp = await request.get(`${backendURL(baseURL)}/health`)
  // 200 = healthy, 503 = unhealthy but structured response required either way.
  expect([200, 503]).toContain(resp.status())
})

test('GET /health includes all four required subsystems', async ({ request, baseURL }) => {
  const resp = await request.get(`${backendURL(baseURL)}/health`)
  // Accept both 200 (healthy) and 503 (one or more subsystems failing).
  expect([200, 503]).toContain(resp.status())

  const body = await resp.json()
  // Must have a 'status' field.
  expect(typeof body.status).toBe('string')
  expect(['healthy', 'unhealthy']).toContain(body.status)

  // Must have a 'subsystems' object with all four keys.
  expect(typeof body.subsystems).toBe('object')
  const keys = Object.keys(body.subsystems)
  expect(keys).toEqual(expect.arrayContaining(['sqlite', 'nats', 'data_dir', 'key']))

  // Each subsystem must have an 'ok' boolean.
  for (const key of ['sqlite', 'nats', 'data_dir', 'key']) {
    expect(typeof body.subsystems[key].ok).toBe('boolean')
  }
})

test('GET /health sqlite and data_dir subsystems are healthy', async ({ request, baseURL }) => {
  // SQLite and data_dir should always be healthy when the server is up.
  const resp = await request.get(`${backendURL(baseURL)}/health`)
  expect([200, 503]).toContain(resp.status())

  const body = await resp.json()
  // sqlite and data_dir should be healthy regardless of NATS/key state.
  expect(body.subsystems['sqlite'].ok).toBe(true)
  expect(body.subsystems['data_dir'].ok).toBe(true)
})

// ---------------------------------------------------------------------------
// GET /docs — Swagger UI
// ---------------------------------------------------------------------------

test('GET /docs serves Swagger UI HTML', async ({ request, baseURL }) => {
  const resp = await request.get(`${backendURL(baseURL)}/docs`)
  expect(resp.status()).toBe(200)

  const contentType = resp.headers()['content-type'] ?? ''
  expect(contentType).toContain('text/html')

  const text = await resp.text()
  // Swagger UI HTML includes "swagger" in the page content.
  expect(text.toLowerCase()).toContain('swagger')
})

test('GET /docs/openapi3.json serves embedded OpenAPI spec', async ({ request, baseURL }) => {
  const resp = await request.get(`${backendURL(baseURL)}/docs/openapi3.json`)
  expect(resp.status()).toBe(200)

  const contentType = resp.headers()['content-type'] ?? ''
  expect(contentType).toContain('application/json')

  const spec = await resp.json()
  // OpenAPI 3.x spec must have these top-level fields.
  expect(spec.openapi).toBeTruthy()
  expect(spec.info).toBeTruthy()
})

// ---------------------------------------------------------------------------
// GET /api/sse — Server-Sent Events
// The SSE endpoint is a long-lived streaming connection.
// We verify: EventSource transitions to OPEN state via the proxy.
// ---------------------------------------------------------------------------

test('GET /api/sse EventSource connects successfully via proxy', async ({ page, baseURL }) => {
  const base = baseURL ?? 'http://frontend:3000'

  // Navigate to the app so we have a same-origin page context.
  // Bypass setup wizard guard first so navigation is not intercepted.
  await bypassSetupWizard(page)
  await page.goto(base)

  // Use EventSource (the native SSE API) which the browser supports natively.
  // EventSource connects via same-origin proxy (/api/sse → backend).
  // readyState: 0=CONNECTING, 1=OPEN, 2=CLOSED
  // We check that it transitions from CONNECTING to OPEN within a timeout.
  const readyState = await page.evaluate(async (sseURL) => {
    return new Promise<number>((resolve) => {
      const es = new EventSource(sseURL)

      es.onopen = () => {
        resolve(es.readyState) // Should be 1 (OPEN)
      }

      es.onerror = () => {
        resolve(es.readyState) // Returns current state on error
        es.close()
      }

      // Timeout fallback — return current state after 8s.
      setTimeout(() => {
        resolve(es.readyState)
        es.close()
      }, 8000)
    })
  }, `${base}/api/sse`)

  // EventSource.OPEN = 1 means the connection was established successfully.
  expect(readyState).toBe(1)
})

// ---------------------------------------------------------------------------
// CORS headers
// ---------------------------------------------------------------------------

test('backend sets CORS header for frontend origin', async ({ request, baseURL }) => {
  const resp = await request.get(`${backendURL(baseURL)}/health`, {
    headers: { Origin: 'http://localhost:3000' },
  })
  const acao = resp.headers()['access-control-allow-origin'] ?? ''
  expect(acao).toBe('http://localhost:3000')
})

test('OPTIONS preflight returns 204', async ({ request, baseURL }) => {
  const resp = await request.fetch(`${backendURL(baseURL)}/v1/settings/secrets`, {
    method: 'OPTIONS',
    headers: {
      Origin: 'http://localhost:3000',
      'Access-Control-Request-Method': 'GET',
    },
  })
  expect(resp.status()).toBe(204)
})
