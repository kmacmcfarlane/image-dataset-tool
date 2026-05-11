import { test, expect } from '@playwright/test'

/**
 * Setup wizard E2E tests.
 *
 * These tests exercise the first-run setup wizard. Each test clears
 * localStorage before navigating so the wizard guard fires.
 *
 * Prerequisites: dev stack must be running (make up-dev).
 * Run: make test-e2e SPEC=setup-wizard.spec.ts
 */

const SETUP_COMPLETE_KEY = 'setup_wizard_complete'

// Helper: navigate to root with setup_wizard_complete cleared so the guard
// redirects to /setup.
async function openWizard(page: import('@playwright/test').Page): Promise<void> {
  // Clear the flag before loading the page.
  await page.goto('/setup')
  await page.evaluate((key) => localStorage.removeItem(key), SETUP_COMPLETE_KEY)
  await page.goto('/')
  // Guard should redirect to /setup
  await expect(page).toHaveURL(/\/setup/)
}

// AC: On first launch (empty DB / setup_wizard_complete not set), redirects to /setup
test('root redirects to /setup when setup not complete', async ({ page }) => {
  await page.goto('/setup')
  await page.evaluate((key) => localStorage.removeItem(key), SETUP_COMPLETE_KEY)
  await page.goto('/')
  await expect(page).toHaveURL(/\/setup/)
  await expect(page.locator('[data-testid="setup-wizard"]')).toBeVisible()
})

// AC: Setup wizard not shown on subsequent launches (localStorage flag set)
test('root does NOT redirect to /setup when setup is complete', async ({ page }) => {
  await page.goto('/setup')
  await page.evaluate((key) => localStorage.setItem(key, 'true'), SETUP_COMPLETE_KEY)
  await page.goto('/')
  await expect(page).toHaveURL('/')
  await expect(page.locator('[data-testid="view-project-list"]')).toBeVisible()
})

// AC: Step 1 — data directory: shows $DATA_DIR, validates writable
test('step 1 shows data directory path and writable status', async ({ page }) => {
  await openWizard(page)
  await expect(page.locator('[data-testid="step-1"]')).toBeVisible()

  // Data dir path should be non-empty
  const dataDirEl = page.locator('[data-testid="data-dir-path"]')
  await expect(dataDirEl).toBeVisible({ timeout: 10000 })
  const text = await dataDirEl.textContent()
  expect(text?.trim().length).toBeGreaterThan(0)

  // Writable indicator should be visible
  await expect(page.locator('[data-testid="data-dir-writable"]')).toBeVisible()
})

// AC: Step 1 Next advances to step 2
test('step 1 Next button advances to step 2', async ({ page }) => {
  await openWizard(page)
  await expect(page.locator('[data-testid="step-1"]')).toBeVisible()
  // Wait for status to load (Next becomes enabled when data_dir_writable)
  await expect(page.locator('[data-testid="data-dir-writable"]')).toBeVisible({ timeout: 10000 })
  const nextBtn = page.locator('[data-testid="step-1-next"]')
  await expect(nextBtn).not.toBeDisabled({ timeout: 10000 })
  await nextBtn.click()
  await expect(page.locator('[data-testid="step-2"]')).toBeVisible()
})

// AC: Step 2 — encryption key: shows 'Key found' if exists; Generate button if missing
test('step 2 shows key status', async ({ page }) => {
  await openWizard(page)
  // Advance to step 2
  const nextBtn = page.locator('[data-testid="step-1-next"]')
  await expect(nextBtn).not.toBeDisabled({ timeout: 10000 })
  await nextBtn.click()

  await expect(page.locator('[data-testid="step-2"]')).toBeVisible()
  await expect(page.locator('[data-testid="key-status-badge"]')).toBeVisible()

  const statusText = await page.locator('[data-testid="key-status-badge"]').textContent()
  // Should show either "Key found" or "Missing" message
  expect(statusText).toBeTruthy()
})

// AC: Step 2 — key found case: Next is enabled and no Generate button
test('step 2 Next is enabled when key is found', async ({ page }) => {
  await openWizard(page)
  const nextBtn = page.locator('[data-testid="step-1-next"]')
  await expect(nextBtn).not.toBeDisabled({ timeout: 10000 })
  await nextBtn.click()

  await expect(page.locator('[data-testid="step-2"]')).toBeVisible()
  const statusBadge = page.locator('[data-testid="key-status-badge"]')
  await expect(statusBadge).toBeVisible({ timeout: 5000 })
  const statusText = await statusBadge.textContent()

  if (statusText?.includes('Key found')) {
    // Next should be enabled and Generate button should not be visible
    await expect(page.locator('[data-testid="step-2-next"]')).not.toBeDisabled()
    await expect(page.locator('[data-testid="generate-key-btn"]')).not.toBeVisible()
  } else {
    // Generate button should be visible
    await expect(page.locator('[data-testid="generate-key-btn"]')).toBeVisible()
  }
})

// AC: Step 3 — provider setup: optional API key input + provider dropdown; Skip button
test('step 3 has provider dropdown, API key input, and Skip button', async ({ page }) => {
  await openWizard(page)

  // Navigate to step 3 (advance through steps 1 and 2)
  const step1Next = page.locator('[data-testid="step-1-next"]')
  await expect(step1Next).not.toBeDisabled({ timeout: 10000 })
  await step1Next.click()
  await expect(page.locator('[data-testid="step-2"]')).toBeVisible()

  // If key is found, next is enabled; otherwise we may need to generate
  const step2Next = page.locator('[data-testid="step-2-next"]')
  await expect(step2Next).not.toBeDisabled({ timeout: 5000 })
  await step2Next.click()

  await expect(page.locator('[data-testid="step-3"]')).toBeVisible()
  await expect(page.locator('[data-testid="provider-select"]')).toBeVisible()
  await expect(page.locator('[data-testid="provider-api-key-input"]')).toBeVisible()
  await expect(page.locator('[data-testid="skip-provider-btn"]')).toBeVisible()
})

// AC: Step 3 — Skip navigates to step 4
test('step 3 Skip button goes to step 4', async ({ page }) => {
  await openWizard(page)

  const step1Next = page.locator('[data-testid="step-1-next"]')
  await expect(step1Next).not.toBeDisabled({ timeout: 10000 })
  await step1Next.click()

  const step2Next = page.locator('[data-testid="step-2-next"]')
  await expect(step2Next).not.toBeDisabled({ timeout: 5000 })
  await step2Next.click()

  await page.locator('[data-testid="skip-provider-btn"]').click()
  await expect(page.locator('[data-testid="step-4"]')).toBeVisible()
})

// AC: Step 4 — complete: redirects to / and sets localStorage flag
test('step 4 completes setup and redirects to /', async ({ page }) => {
  await openWizard(page)

  const step1Next = page.locator('[data-testid="step-1-next"]')
  await expect(step1Next).not.toBeDisabled({ timeout: 10000 })
  await step1Next.click()

  const step2Next = page.locator('[data-testid="step-2-next"]')
  await expect(step2Next).not.toBeDisabled({ timeout: 5000 })
  await step2Next.click()

  await page.locator('[data-testid="skip-provider-btn"]').click()
  await expect(page.locator('[data-testid="step-4"]')).toBeVisible()

  await page.locator('[data-testid="complete-setup-btn"]').click()

  // Should redirect to project list
  await expect(page).toHaveURL('/')
  await expect(page.locator('[data-testid="view-project-list"]')).toBeVisible()

  // localStorage flag should be set
  const flag = await page.evaluate((key) => localStorage.getItem(key), SETUP_COMPLETE_KEY)
  expect(flag).toBe('true')
})

// AC: Walk through setup wizard on fresh $DATA_DIR (full happy path)
test('full setup wizard walk-through', async ({ page }) => {
  await openWizard(page)

  // Step 1: data directory
  await expect(page.locator('[data-testid="step-1"]')).toBeVisible()
  await expect(page.locator('[data-testid="step-indicators"]')).toBeVisible()
  const step1Next = page.locator('[data-testid="step-1-next"]')
  await expect(step1Next).not.toBeDisabled({ timeout: 10000 })
  await step1Next.click()

  // Step 2: encryption key
  await expect(page.locator('[data-testid="step-2"]')).toBeVisible()
  const step2Next = page.locator('[data-testid="step-2-next"]')
  await expect(step2Next).not.toBeDisabled({ timeout: 5000 })
  await step2Next.click()

  // Step 3: provider (skip)
  await expect(page.locator('[data-testid="step-3"]')).toBeVisible()
  await page.locator('[data-testid="skip-provider-btn"]').click()

  // Step 4: complete
  await expect(page.locator('[data-testid="step-4"]')).toBeVisible()
  await page.locator('[data-testid="complete-setup-btn"]').click()

  // Redirected to projects
  await expect(page).toHaveURL('/')
  await expect(page.locator('[data-testid="view-project-list"]')).toBeVisible()
})
