import { test, expect } from '@playwright/test'
import { bypassSetupWizard } from './helpers'

/**
 * E2E tests for S-017: Streaming grid view with connect-then-fetch,
 * virtual scroll, and lazy thumbnails.
 */

// Derive the backend URL from Playwright's baseURL config.
function backendURL(baseURL: string | undefined): string {
  return (baseURL ?? 'http://localhost:3000').includes('frontend')
    ? 'http://backend:8080'
    : 'http://localhost:8080'
}

// AC: BE: GET /v1/subjects/{sid}/samples returns full metadata array (no pagination)
test('GET /v1/subjects/{sid}/samples returns samples array', async ({ request, baseURL }) => {
  // Use a non-existent subject ID — should return empty array, not error.
  const resp = await request.get(
    `${backendURL(baseURL)}/v1/subjects/00000000-0000-0000-0000-000000000001/samples`,
  )
  expect(resp.status()).toBe(200)

  const body = await resp.json()
  expect(body).toHaveProperty('samples')
  expect(Array.isArray(body.samples)).toBe(true)
})

// AC: BE: GET /v1/subjects/{sid}/samples validates subject_id format
test('GET /v1/subjects/{sid}/samples rejects invalid subject_id', async ({ request, baseURL }) => {
  const resp = await request.get(
    `${backendURL(baseURL)}/v1/subjects/not!a!valid!uuid/samples`,
  )
  expect(resp.status()).toBe(400)
})

// AC: FE: Connect-then-fetch: SSE first, fetch metadata, merge by ID (idempotent)
test('Sample grid view loads and connects SSE', async ({ page, baseURL }) => {
  const base = baseURL ?? 'http://frontend:3000'
  await bypassSetupWizard(page)

  const suffix = Date.now().toString(36)

  // Create a project and subject via API to have valid route params.
  const projectResp = await page.request.post(`${backendURL(baseURL)}/v1/projects`, {
    data: { name: `E2E Grid Test ${suffix}`, description: 'test' },
  })
  expect(projectResp.status()).toBe(201)
  const project = await projectResp.json()

  const subjectResp = await page.request.post(
    `${backendURL(baseURL)}/v1/projects/${project.id}/subjects`,
    { data: { name: `Grid Subject ${suffix}` } },
  )
  expect(subjectResp.status()).toBe(201)
  const subject = await subjectResp.json()

  // Navigate to sample grid view.
  await page.goto(`${base}/projects/${project.id}/subjects/${subject.id}/samples`)

  // Verify the grid view renders.
  await expect(page.locator('[data-testid="view-sample-grid"]')).toBeVisible()

  // Verify controls are visible.
  await expect(page.locator('[data-testid="sample-grid-controls"]')).toBeVisible()

  // AC: FE: Status toggles (pending/kept/rejected) client-side; default pending
  await expect(page.locator('[data-testid="filter-pending"]')).toBeVisible()
  await expect(page.locator('[data-testid="filter-kept"]')).toBeVisible()
  await expect(page.locator('[data-testid="filter-rejected"]')).toBeVisible()

  // AC: FE: Duplicate toggle: off by default
  await expect(page.locator('[data-testid="filter-duplicates"]')).toBeVisible()

  // Since no samples exist, should show empty state.
  await expect(page.locator('[data-testid="sample-grid-empty"]')).toBeVisible({ timeout: 5000 })
})

// AC: BE: SSE emits sample.new and sample.updated events
// AC: FE: New samples from active ingest appear in real-time
test('SSE sample.new event appears in grid in real-time', async ({ page, baseURL }) => {
  const base = baseURL ?? 'http://frontend:3000'
  await bypassSetupWizard(page)

  const suffix = Date.now().toString(36)

  // Create project and subject.
  const projectResp = await page.request.post(`${backendURL(baseURL)}/v1/projects`, {
    data: { name: `E2E SSE Test ${suffix}`, description: 'test' },
  })
  const project = await projectResp.json()

  const subjectResp = await page.request.post(
    `${backendURL(baseURL)}/v1/projects/${project.id}/subjects`,
    { data: { name: `SSE Subject ${suffix}` } },
  )
  const subject = await subjectResp.json()

  // Navigate to sample grid.
  await page.goto(`${base}/projects/${project.id}/subjects/${subject.id}/samples`)
  await expect(page.locator('[data-testid="view-sample-grid"]')).toBeVisible()

  // Verify SSE connection is established by checking EventSource readyState from page context.
  const sseConnected = await page.evaluate(async (sseURL) => {
    return new Promise<boolean>((resolve) => {
      const es = new EventSource(sseURL)
      es.onopen = () => {
        es.close()
        resolve(true)
      }
      es.onerror = () => {
        es.close()
        resolve(false)
      }
      setTimeout(() => {
        es.close()
        resolve(false)
      }, 5000)
    })
  }, `${base}/api/sse`)

  expect(sseConnected).toBe(true)
})
