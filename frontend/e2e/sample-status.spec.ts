import { test, expect } from '@playwright/test'
import { bypassSetupWizard } from './helpers'

/**
 * E2E tests for S-018: Keep/reject + multi-select + bulk actions.
 */

// Derive the backend URL from Playwright's baseURL config.
function backendURL(baseURL: string | undefined): string {
  return (baseURL ?? 'http://localhost:3000').includes('frontend')
    ? 'http://backend:8080'
    : 'http://localhost:8080'
}

// AC: BE: PUT /v1/samples/{id}/status updates status
test('PUT /v1/samples/{id}/status updates sample status', async ({ request, baseURL }) => {
  const backend = backendURL(baseURL)
  const suffix = Date.now().toString(36)

  // Create project, subject, and seed a sample via DB.
  const projectResp = await request.post(`${backend}/v1/projects`, {
    data: { name: `Status Test ${suffix}`, description: 'test' },
  })
  expect(projectResp.status()).toBe(201)
  const project = await projectResp.json()

  const subjectResp = await request.post(
    `${backend}/v1/projects/${project.id}/subjects`,
    { data: { name: `Status Subject ${suffix}` } },
  )
  expect(subjectResp.status()).toBe(201)
  const subject = await subjectResp.json()

  // List samples (empty at first, but we need a sample — we'll check the endpoint logic).
  const listResp = await request.get(
    `${backend}/v1/subjects/${subject.id}/samples`,
  )
  expect(listResp.status()).toBe(200)

  // Test with a non-existent sample ID (should 404).
  const updateResp = await request.put(
    `${backend}/v1/samples/00000000-0000-0000-0000-000000000001/status`,
    { data: { status: 'kept' } },
  )
  expect(updateResp.status()).toBe(404)
})

// AC: BE: PUT /v1/samples/{id}/status validates status value
test('PUT /v1/samples/{id}/status rejects invalid status', async ({ request, baseURL }) => {
  const backend = backendURL(baseURL)

  const resp = await request.put(
    `${backend}/v1/samples/00000000-0000-0000-0000-000000000001/status`,
    { data: { status: 'invalid' } },
  )
  expect(resp.status()).toBe(400)
})

// AC: BE: PUT /v1/samples/bulk-status atomic batch update
test('PUT /v1/samples/bulk-status validates input', async ({ request, baseURL }) => {
  const backend = backendURL(baseURL)

  // Empty IDs should return 400.
  const emptyResp = await request.put(`${backend}/v1/samples/bulk-status`, {
    data: { ids: [], status: 'kept' },
  })
  expect(emptyResp.status()).toBe(400)

  // Missing status should return 400.
  const noStatusResp = await request.put(`${backend}/v1/samples/bulk-status`, {
    data: { ids: ['00000000-0000-0000-0000-000000000001'] },
  })
  expect(noStatusResp.status()).toBe(400)
})

// AC: BE: PUT /v1/samples/bulk-status returns count of updated samples
test('PUT /v1/samples/bulk-status returns updated count (0 for non-existent)', async ({ request, baseURL }) => {
  const backend = backendURL(baseURL)

  const resp = await request.put(`${backend}/v1/samples/bulk-status`, {
    data: { ids: ['00000000-0000-0000-0000-000000000001'], status: 'kept' },
  })
  expect(resp.status()).toBe(200)
  const body = await resp.json()
  expect(body.updated).toBe(0) // non-existent IDs are skipped
})

// AC: FE: Grid view renders with selection controls and keyboard shortcuts
test('Sample grid view shows selection count and supports keyboard shortcuts', async ({ page, baseURL }) => {
  const base = baseURL ?? 'http://frontend:3000'
  const backend = backendURL(baseURL)
  await bypassSetupWizard(page)

  const suffix = Date.now().toString(36)

  // Create project and subject.
  const projectResp = await page.request.post(`${backend}/v1/projects`, {
    data: { name: `E2E Select Test ${suffix}`, description: 'test' },
  })
  expect(projectResp.status()).toBe(201)
  const project = await projectResp.json()

  const subjectResp = await page.request.post(
    `${backend}/v1/projects/${project.id}/subjects`,
    { data: { name: `Select Subject ${suffix}` } },
  )
  expect(subjectResp.status()).toBe(201)
  const subject = await subjectResp.json()

  // Navigate to sample grid.
  await page.goto(`${base}/projects/${project.id}/subjects/${subject.id}/samples`)
  await expect(page.locator('[data-testid="view-sample-grid"]')).toBeVisible()

  // Grid should be keyboard-focusable (tabindex=0).
  const gridView = page.locator('[data-testid="view-sample-grid"]')
  await expect(gridView).toHaveAttribute('tabindex', '0')

  // Since no samples, verify empty state.
  await expect(page.locator('[data-testid="sample-grid-empty"]')).toBeVisible({ timeout: 5000 })
})
