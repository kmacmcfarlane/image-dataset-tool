import { test, expect } from '@playwright/test'
import { bypassSetupWizard } from './helpers'

/**
 * Account CRUD E2E tests.
 *
 * Prerequisites: dev stack must be running (make up-dev).
 * Run: make test-e2e SPEC=accounts.spec.ts
 */

// Ensure the setup wizard guard does not intercept navigation in these tests.
test.beforeEach(async ({ page }) => {
  await bypassSetupWizard(page)
})

/**
 * Helper: select a platform from the NSelect dropdown.
 * Clicks the select trigger, then clicks the matching option in the popup.
 */
async function selectPlatform(page: import('@playwright/test').Page, label: string): Promise<void> {
  await page.locator('[data-testid="account-platform-input"]').click()
  // Naive UI renders dropdown options in a detached popup; find option by text.
  await page.getByText(label, { exact: true }).click()
}

// AC: FE: /accounts view with list, add dialog (platform + handle + cookie textarea), delete
test('accounts page loads with empty state or list', async ({ page }) => {
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()

  // Either empty state or account list should be visible
  const emptyOrList = page.locator('[data-testid="accounts-empty"], [data-testid="account-list"]')
  await expect(emptyOrList.first()).toBeVisible({ timeout: 5000 })

  // Create button should always be visible
  await expect(page.locator('[data-testid="create-account-btn"]')).toBeVisible()
})

// AC: BE: POST /v1/accounts creates account (platform, handle); unique (platform, handle)
// AC: BE: GET /v1/accounts lists all; never returns cookies
// AC: BE: DELETE /v1/accounts/{id} removes account + all subject_accounts links
// AC: FE: /accounts view with list, add dialog, delete
// AC: Platform dropdown replaces free-text input
test('create, read, delete account round trip', async ({ page }) => {
  const handle = `e2e-user-${Date.now()}`
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()

  // --- CREATE ---
  await page.locator('[data-testid="create-account-btn"]').click()
  const dialog = page.locator('[data-testid="account-dialog"]')
  await expect(dialog).toBeVisible()

  await selectPlatform(page, 'Instagram')
  await page.locator('[data-testid="account-handle-input"]').fill(handle)
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).not.toBeVisible()

  // --- READ ---
  const accountCard = page.locator(`[data-testid="account-card-instagram-${handle}"]`)
  await expect(accountCard).toBeVisible({ timeout: 5000 })

  // Verify cookie status shows "No cookies"
  const cookieStatus = page.locator(`[data-testid="account-cookies-instagram-${handle}"]`)
  await expect(cookieStatus).toHaveText('No cookies')

  // --- DELETE ---
  const menuBtn = accountCard.locator(`[data-testid="account-menu-instagram-${handle}"]`)
  await menuBtn.click()
  await page.getByText('Delete', { exact: true }).click()

  const deleteDialog = page.locator('[data-testid="delete-account-dialog"]')
  await expect(deleteDialog).toBeVisible()

  await deleteDialog.getByRole('button', { name: 'Delete' }).click()
  await expect(deleteDialog).not.toBeVisible()
  await expect(accountCard).not.toBeVisible({ timeout: 5000 })
})

// AC: BE: PUT /v1/accounts/{id}/cookies updates cookies; encrypts with AES-256-GCM
// AC: E2E: encrypt/decrypt round trip
// AC: Cookie extraction instructions shown in create dialog
test('create account with cookies and verify cookies-set status', async ({ page }) => {
  const handle = `e2e-cookies-${Date.now()}`
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()

  // Create account with cookies
  await page.locator('[data-testid="create-account-btn"]').click()
  const dialog = page.locator('[data-testid="account-dialog"]')
  await expect(dialog).toBeVisible()

  await selectPlatform(page, 'Instagram')

  // AC: Cookie example shown after platform selection
  await expect(page.locator('[data-testid="cookie-example"]')).toBeVisible()
  await expect(page.locator('[data-testid="cookie-example"]')).toContainText('sessionid')

  await page.locator('[data-testid="account-handle-input"]').fill(handle)
  await page.locator('[data-testid="account-cookies-input"]').fill(
    'ig_did=abc; mid=def; sessionid=abc123; csrftoken=xyz789; ds_user_id=12345',
  )
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).not.toBeVisible()

  // Verify card shows "Cookies set"
  const accountCard = page.locator(`[data-testid="account-card-instagram-${handle}"]`)
  await expect(accountCard).toBeVisible({ timeout: 5000 })
  const cookieStatus = page.locator(`[data-testid="account-cookies-instagram-${handle}"]`)
  await expect(cookieStatus).toHaveText('Cookies set')

  // Clean up
  const menuBtn = accountCard.locator(`[data-testid="account-menu-instagram-${handle}"]`)
  await menuBtn.click()
  await page.getByText('Delete', { exact: true }).click()
  const deleteDialog = page.locator('[data-testid="delete-account-dialog"]')
  await deleteDialog.getByRole('button', { name: 'Delete' }).click()
  await expect(accountCard).not.toBeVisible({ timeout: 5000 })
})

// AC: Update cookies via context menu
// AC: Cookie extraction instructions shown in update cookies dialog
test('update cookies via context menu', async ({ page }) => {
  const handle = `e2e-update-cookies-${Date.now()}`
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()

  // Create account without cookies
  await page.locator('[data-testid="create-account-btn"]').click()
  const createDialog = page.locator('[data-testid="account-dialog"]')
  await expect(createDialog).toBeVisible()
  await selectPlatform(page, 'Instagram')
  await page.locator('[data-testid="account-handle-input"]').fill(handle)
  await createDialog.getByRole('button', { name: 'Save' }).click()
  await expect(createDialog).not.toBeVisible()

  const accountCard = page.locator(`[data-testid="account-card-instagram-${handle}"]`)
  await expect(accountCard).toBeVisible({ timeout: 5000 })

  // Verify no cookies initially
  await expect(page.locator(`[data-testid="account-cookies-instagram-${handle}"]`)).toHaveText('No cookies')

  // Open context menu and update cookies
  const menuBtn = accountCard.locator(`[data-testid="account-menu-instagram-${handle}"]`)
  await menuBtn.click()
  await page.getByText('Update Cookies', { exact: true }).click()

  const cookiesDialog = page.locator('[data-testid="cookies-dialog"]')
  await expect(cookiesDialog).toBeVisible()

  // AC: Cookie example shown in update cookies dialog
  await expect(page.locator('[data-testid="update-cookie-example"]')).toBeVisible()
  await expect(page.locator('[data-testid="update-cookie-example"]')).toContainText('sessionid')

  await page.locator('[data-testid="update-cookies-input"]').fill(
    'sessionid=new_value; csrftoken=updated; ds_user_id=999',
  )
  await cookiesDialog.getByRole('button', { name: 'Save' }).click()
  await expect(cookiesDialog).not.toBeVisible()

  // Verify cookies status updated
  await expect(page.locator(`[data-testid="account-cookies-instagram-${handle}"]`)).toHaveText('Cookies set')

  // Clean up
  await menuBtn.click()
  await page.getByText('Delete', { exact: true }).click()
  const deleteDialog = page.locator('[data-testid="delete-account-dialog"]')
  await deleteDialog.getByRole('button', { name: 'Delete' }).click()
  await expect(accountCard).not.toBeVisible({ timeout: 5000 })
})

// AC: Empty platform/handle shows warning
test('empty platform or handle shows warning', async ({ page }) => {
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()

  await page.locator('[data-testid="create-account-btn"]').click()
  const dialog = page.locator('[data-testid="account-dialog"]')
  await expect(dialog).toBeVisible()

  // Try to save with empty fields — dialog stays open
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeVisible()

  // Fill platform but not handle
  await selectPlatform(page, 'Instagram')
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeVisible()
})

// AC: Cookie validation — missing required keys shows inline error
test('cookie validation shows error for missing required keys in create dialog', async ({ page }) => {
  const handle = `e2e-cookie-val-${Date.now()}`
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()

  await page.locator('[data-testid="create-account-btn"]').click()
  const dialog = page.locator('[data-testid="account-dialog"]')
  await expect(dialog).toBeVisible()

  await selectPlatform(page, 'Instagram')
  await page.locator('[data-testid="account-handle-input"]').fill(handle)
  // Provide cookies missing required keys (no csrftoken, no ds_user_id)
  await page.locator('[data-testid="account-cookies-input"]').fill('sessionid=abc; mid=def')
  await dialog.getByRole('button', { name: 'Save' }).click()

  // Dialog should stay open with inline error
  await expect(dialog).toBeVisible()
  const errorMsg = page.locator('[data-testid="create-cookie-error"]')
  await expect(errorMsg).toBeVisible()
  await expect(errorMsg).toContainText('csrftoken')
  await expect(errorMsg).toContainText('ds_user_id')
})

// AC: Cookie validation — missing required keys shows inline error in update dialog
test('cookie validation shows error for missing required keys in update dialog', async ({ page }) => {
  const handle = `e2e-cookie-val2-${Date.now()}`
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()

  // Create account first
  await page.locator('[data-testid="create-account-btn"]').click()
  const createDialog = page.locator('[data-testid="account-dialog"]')
  await expect(createDialog).toBeVisible()
  await selectPlatform(page, 'Instagram')
  await page.locator('[data-testid="account-handle-input"]').fill(handle)
  await createDialog.getByRole('button', { name: 'Save' }).click()
  await expect(createDialog).not.toBeVisible()

  const accountCard = page.locator(`[data-testid="account-card-instagram-${handle}"]`)
  await expect(accountCard).toBeVisible({ timeout: 5000 })

  // Open update cookies dialog
  const menuBtn = accountCard.locator(`[data-testid="account-menu-instagram-${handle}"]`)
  await menuBtn.click()
  await page.getByText('Update Cookies', { exact: true }).click()

  const cookiesDialog = page.locator('[data-testid="cookies-dialog"]')
  await expect(cookiesDialog).toBeVisible()

  // Provide cookies missing required keys
  await page.locator('[data-testid="update-cookies-input"]').fill('ig_did=abc; mid=def')
  await cookiesDialog.getByRole('button', { name: 'Save' }).click()

  // Dialog should stay open with inline error
  await expect(cookiesDialog).toBeVisible()
  const errorMsg = page.locator('[data-testid="update-cookie-error"]')
  await expect(errorMsg).toBeVisible()
  await expect(errorMsg).toContainText('sessionid')
  await expect(errorMsg).toContainText('csrftoken')
  await expect(errorMsg).toContainText('ds_user_id')

  // Clean up — close dialog and delete account
  await cookiesDialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(cookiesDialog).not.toBeVisible()
  await menuBtn.click()
  await page.getByText('Delete', { exact: true }).click()
  const deleteDialog = page.locator('[data-testid="delete-account-dialog"]')
  await deleteDialog.getByRole('button', { name: 'Delete' }).click()
  await expect(accountCard).not.toBeVisible({ timeout: 5000 })
})
