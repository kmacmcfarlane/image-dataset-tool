import { test, expect } from '@playwright/test'

/**
 * Settings page E2E tests.
 *
 * Prerequisites: dev stack must be running (make up-dev).
 * Run: make test-e2e-serial SPEC=settings.spec.ts
 */

// AC: /settings view loads and displays the page
test('settings page loads', async ({ page }) => {
  await page.goto('/settings')
  await expect(page.locator('[data-testid="view-settings"]')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible()
})

// AC: Encryption key status is displayed (found/missing/wrong permissions)
test('encryption key status is shown', async ({ page }) => {
  await page.goto('/settings')
  // Wait for system info to load
  await expect(page.locator('[data-testid="system-info"]')).toBeVisible({ timeout: 10000 })
  // One of the three status indicators should be visible
  const keyStatusFound = page.locator('[data-testid="key-status-found"]')
  const keyStatusMissing = page.locator('[data-testid="key-status-missing"]')
  const keyStatusWrong = page.locator('[data-testid="key-status-wrong_permissions"]')

  const anyVisible = await Promise.any([
    keyStatusFound.isVisible(),
    keyStatusMissing.isVisible(),
    keyStatusWrong.isVisible(),
  ])
  expect(anyVisible).toBe(true)
})

// AC: Data dir path is displayed read-only
test('data directory path is displayed', async ({ page }) => {
  await page.goto('/settings')
  await expect(page.locator('[data-testid="system-info"]')).toBeVisible({ timeout: 10000 })
  const dataDir = page.locator('[data-testid="data-dir"]')
  await expect(dataDir).toBeVisible()
  const text = await dataDir.textContent()
  expect(text).toBeTruthy()
  expect(text!.trim().length).toBeGreaterThan(0)
})

// AC: Toast notification level threshold setting (persisted in localStorage)
test('toast threshold buttons are displayed and update localStorage', async ({ page }) => {
  await page.goto('/settings')
  await expect(page.locator('[data-testid="threshold-buttons"]')).toBeVisible()

  // Click 'error' threshold
  await page.locator('[data-testid="threshold-btn-error"]').click()
  const stored = await page.evaluate(() => localStorage.getItem('toast-level-threshold'))
  expect(stored).toBe('error')

  // Click 'info' threshold
  await page.locator('[data-testid="threshold-btn-info"]').click()
  const stored2 = await page.evaluate(() => localStorage.getItem('toast-level-threshold'))
  expect(stored2).toBe('info')
})

// AC: Secrets management — list keys
test('secrets section shows the secrets table', async ({ page }) => {
  await page.goto('/settings')
  await expect(page.locator('[data-testid="secrets-table"]')).toBeVisible({ timeout: 10000 })
})

// AC: Secrets management — add secret
test('can add a new secret', async ({ page }) => {
  await page.goto('/settings')
  // Wait for secrets table
  await expect(page.locator('[data-testid="secrets-table"]')).toBeVisible({ timeout: 10000 })

  // Open add secret modal
  await page.locator('[data-testid="add-secret-btn"]').click()
  const addModal = page.locator('[data-testid="secret-modal"]')
  await expect(addModal).toBeVisible()

  // Fill in the form
  await page.locator('[data-testid="secret-key-input"]').fill('e2e_test_key')
  await page.locator('[data-testid="secret-value-input"]').fill('e2e-test-value')

  // Save
  await addModal.getByRole('button', { name: 'Save' }).click()

  // The modal should close and the secret should appear in the table
  await expect(addModal).not.toBeVisible()
  await expect(page.locator('td', { hasText: 'e2e_test_key' })).toBeVisible({ timeout: 5000 })
})

// AC: Secrets management — update secret
test('can update an existing secret', async ({ page }) => {
  await page.goto('/settings')
  await expect(page.locator('[data-testid="secrets-table"]')).toBeVisible({ timeout: 10000 })

  // Ensure the test key exists first
  await page.locator('[data-testid="add-secret-btn"]').click()
  const createModal = page.locator('[data-testid="secret-modal"]')
  await expect(createModal).toBeVisible()
  await page.locator('[data-testid="secret-key-input"]').fill('e2e_update_key')
  await page.locator('[data-testid="secret-value-input"]').fill('original-value')
  await createModal.getByRole('button', { name: 'Save' }).click()
  await expect(createModal).not.toBeVisible()

  // Now click Update on the row
  await expect(page.locator('[data-testid="edit-secret-btn-e2e_update_key"]')).toBeVisible({ timeout: 5000 })
  await page.locator('[data-testid="edit-secret-btn-e2e_update_key"]').click()

  // Modal should open with key pre-filled and disabled
  const editModal = page.locator('[data-testid="secret-modal"]')
  await expect(editModal).toBeVisible()
  const keyInput = page.locator('[data-testid="secret-key-input"]')
  await expect(keyInput).toHaveValue('e2e_update_key')
  await expect(keyInput).toBeDisabled()

  // Update value
  await page.locator('[data-testid="secret-value-input"]').fill('updated-value')
  await editModal.getByRole('button', { name: 'Save' }).click()
  await expect(editModal).not.toBeVisible()
})

// AC: Secrets management — delete secret
test('can delete a secret', async ({ page }) => {
  await page.goto('/settings')
  await expect(page.locator('[data-testid="secrets-table"]')).toBeVisible({ timeout: 10000 })

  // Create a test secret to delete
  await page.locator('[data-testid="add-secret-btn"]').click()
  const createModal = page.locator('[data-testid="secret-modal"]')
  await expect(createModal).toBeVisible()
  await page.locator('[data-testid="secret-key-input"]').fill('e2e_delete_key')
  await page.locator('[data-testid="secret-value-input"]').fill('delete-me')
  await createModal.getByRole('button', { name: 'Save' }).click()
  await expect(createModal).not.toBeVisible()

  // Wait for the row to appear
  await expect(page.locator('[data-testid="delete-secret-btn-e2e_delete_key"]')).toBeVisible({ timeout: 5000 })

  // Click delete
  await page.locator('[data-testid="delete-secret-btn-e2e_delete_key"]').click()

  // Confirm delete modal
  const deleteModal = page.locator('[data-testid="delete-secret-modal"]')
  await expect(deleteModal).toBeVisible()
  await deleteModal.getByRole('button', { name: 'Delete' }).click()

  // Secret should be gone from the table
  await expect(page.locator('td', { hasText: 'e2e_delete_key' })).not.toBeVisible({ timeout: 5000 })
})
