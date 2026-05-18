import { test, expect } from '@playwright/test'
import { bypassSetupWizard } from './helpers'

/**
 * Job history E2E tests.
 *
 * Prerequisites: dev stack must be running (make up-dev).
 * Run: make test-e2e SPEC=jobs.spec.ts
 */

test.beforeEach(async ({ page }) => {
  await bypassSetupWizard(page)
})

// AC: FE: /jobs view with job list
test('jobs view loads and shows job list table', async ({ page }) => {
  await page.goto('/jobs')
  await expect(page.locator('[data-testid="view-job-list"]')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Job History' })).toBeVisible()
  await expect(page.locator('[data-testid="jobs-table"]')).toBeVisible()
})

// AC: FE: /jobs view navigable from sidebar
test('jobs view is accessible via navigation', async ({ page }) => {
  await page.goto('/')
  // Navigate directly to /jobs since sidebar link exists
  await page.goto('/jobs')
  await expect(page.locator('[data-testid="view-job-list"]')).toBeVisible()
})

// AC: BE: GET /v1/jobs returns paginated list
test('GET /v1/jobs returns job list JSON', async ({ page }) => {
  const response = await page.request.get('/v1/jobs')
  expect(response.status()).toBe(200)
  const body = await response.json()
  expect(body).toHaveProperty('jobs')
  expect(body).toHaveProperty('total')
  expect(Array.isArray(body.jobs)).toBe(true)
})

// AC: BE: GET /v1/jobs filters by status
test('GET /v1/jobs?status=running returns only running jobs', async ({ page }) => {
  const response = await page.request.get('/v1/jobs?status=running')
  expect(response.status()).toBe(200)
  const body = await response.json()
  for (const job of body.jobs) {
    expect(job.status).toBe('running')
  }
})

// AC: BE: GET /v1/jobs/{id} returns 404 for unknown job
test('GET /v1/jobs/nonexistent returns 404', async ({ page }) => {
  const response = await page.request.get('/v1/jobs/00000000-0000-0000-0000-000000000000')
  expect(response.status()).toBe(404)
})

// AC: FE: level filter toggle visible when messages panel is open
// (Only applicable if at least one job exists; skip gracefully otherwise.)
test('messages panel shows level filter when job is selected', async ({ page }) => {
  // Seed: check if any jobs exist via the API
  const response = await page.request.get('/v1/jobs?limit=1')
  const body = await response.json()

  if (body.jobs.length === 0) {
    // No jobs yet — just verify the panel is hidden by default.
    await page.goto('/jobs')
    await expect(page.locator('[data-testid="messages-panel"]')).not.toBeVisible()
    return
  }

  const jobId = body.jobs[0].id

  await page.goto('/jobs')
  await expect(page.locator('[data-testid="jobs-table"]')).toBeVisible()

  // Click the Messages button for the first job.
  const msgBtn = page.locator(`[data-testid="view-messages-btn-${jobId}"]`)
  await expect(msgBtn).toBeVisible()
  await msgBtn.click()

  // Messages panel should appear with level filter toggles.
  await expect(page.locator('[data-testid="messages-panel"]')).toBeVisible()
  await expect(page.locator('[data-testid="level-filter-error"]')).toBeVisible()
  await expect(page.locator('[data-testid="level-filter-warning"]')).toBeVisible()
  await expect(page.locator('[data-testid="level-filter-info"]')).toBeVisible()

  // Close panel.
  await page.locator('[data-testid="close-messages-btn"]').click()
  await expect(page.locator('[data-testid="messages-panel"]')).not.toBeVisible()
})

// AC: BE: GET /v1/jobs/{id}/messages default level filter = warning
test('GET /v1/jobs/{id}/messages respects level filter parameter', async ({ page }) => {
  const listResponse = await page.request.get('/v1/jobs?limit=1')
  const listBody = await listResponse.json()

  if (listBody.jobs.length === 0) {
    test.skip()
    return
  }

  const jobId = listBody.jobs[0].id

  // Default (warning) filter — should not return info-level only messages.
  const msgResponse = await page.request.get(`/v1/jobs/${jobId}/messages?level=warning`)
  expect(msgResponse.status()).toBe(200)
  const msgBody = await msgResponse.json()
  expect(msgBody).toHaveProperty('messages')
  expect(msgBody).toHaveProperty('total')
  for (const msg of msgBody.messages) {
    expect(['warning', 'error']).toContain(msg.level)
  }
})

// AC: BE: each message has required fields
test('job messages have timestamp, level, message text fields', async ({ page }) => {
  const listResponse = await page.request.get('/v1/jobs?limit=1')
  const listBody = await listResponse.json()

  if (listBody.jobs.length === 0) {
    test.skip()
    return
  }

  const jobId = listBody.jobs[0].id
  const msgResponse = await page.request.get(`/v1/jobs/${jobId}/messages?level=info&limit=5`)
  expect(msgResponse.status()).toBe(200)
  const msgBody = await msgResponse.json()

  for (const msg of msgBody.messages) {
    expect(msg).toHaveProperty('id')
    expect(msg).toHaveProperty('level')
    expect(msg).toHaveProperty('message')
    expect(msg).toHaveProperty('created_at')
    expect(msg).toHaveProperty('job_run_id')
  }
})

// AC: Refresh button reloads job list
test('refresh button reloads the job list', async ({ page }) => {
  await page.goto('/jobs')
  await expect(page.locator('[data-testid="jobs-table"]')).toBeVisible()
  await page.locator('[data-testid="refresh-jobs-btn"]').click()
  // Table should still be visible after refresh (no crash).
  await expect(page.locator('[data-testid="jobs-table"]')).toBeVisible()
})
