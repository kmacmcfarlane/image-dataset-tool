import { test, expect } from '@playwright/test'
import { bypassSetupWizard } from './helpers'

/**
 * E2E tests for subject-account linking (S-012).
 *
 * Prerequisites: dev stack must be running (make up-dev).
 * Run: make test-e2e SPEC=subject-account-links.spec.ts
 */

test.beforeEach(async ({ page }) => {
  await bypassSetupWizard(page)
})

/**
 * Helper: create a project and navigate to its detail page.
 */
async function createProjectAndNavigate(
  page: import('@playwright/test').Page,
  projectName: string,
): Promise<void> {
  await page.goto('/')
  await expect(page.locator('[data-testid="view-project-list"]')).toBeVisible()

  await page.locator('[data-testid="create-project-btn"]').click()
  const dialog = page.locator('[data-testid="project-dialog"]')
  await expect(dialog).toBeVisible()
  await page.locator('[data-testid="project-name-input"]').fill(projectName)
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).not.toBeVisible()

  const projectCard = page.locator('[data-testid^="project-card-"]', { hasText: projectName })
  await expect(projectCard).toBeVisible({ timeout: 5000 })
  await projectCard.click()
  await expect(page.locator('[data-testid="view-project-detail"]')).toBeVisible({ timeout: 5000 })
}

/**
 * Helper: create a subject in the current project detail view.
 */
async function createSubject(
  page: import('@playwright/test').Page,
  subjectName: string,
): Promise<void> {
  await page.locator('[data-testid="create-subject-btn"]').click()
  const dialog = page.locator('[data-testid="subject-dialog"]')
  await expect(dialog).toBeVisible()
  await page.locator('[data-testid="subject-name-input"]').fill(subjectName)
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).not.toBeVisible()
  await expect(page.locator('[data-testid^="subject-card-"]', { hasText: subjectName })).toBeVisible({ timeout: 5000 })
}

/**
 * Helper: create an account via the accounts page.
 */
async function createAccount(
  page: import('@playwright/test').Page,
  platform: string,
  handle: string,
): Promise<void> {
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()

  await page.locator('[data-testid="create-account-btn"]').click()
  const dialog = page.locator('[data-testid="account-dialog"]')
  await expect(dialog).toBeVisible()
  // Platform is now a dropdown select — click to open, then select option.
  const platformLabel = platform.charAt(0).toUpperCase() + platform.slice(1)
  await page.locator('[data-testid="account-platform-input"]').click()
  await page.getByText(platformLabel, { exact: true }).click()
  await page.locator('[data-testid="account-handle-input"]').fill(handle)
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).not.toBeVisible()
}

// AC: E2E: Link, list, unlink round trip
test('link, list, and unlink account from subject — round trip', async ({ page }) => {
  const ts = Date.now()
  const projectName = `E2E LinkAcc Project ${ts}`
  const subjectName = `E2E LinkAcc Subject ${ts}`
  const handle = `e2euser${ts}`

  // Create account first
  await createAccount(page, 'instagram', handle)

  // Navigate back to project and create subject
  await createProjectAndNavigate(page, projectName)
  await createSubject(page, subjectName)

  // AC: FE: Subject detail has 'Link account' picker
  const subjectCard = page.locator('[data-testid^="subject-card-"]', { hasText: subjectName })
  const menuBtn = subjectCard.locator('[data-testid^="subject-menu-"]')
  await menuBtn.click()
  await page.getByText('Link account', { exact: true }).click()

  const linkDialog = page.locator('[data-testid="link-account-dialog"]')
  await expect(linkDialog).toBeVisible()

  // AC: FE: option to navigate to /accounts
  await expect(linkDialog.locator('[data-testid="go-to-accounts-link"]')).toBeVisible()

  // AC: BE: POST /v1/subjects/{sid}/accounts links existing account
  const picker = linkDialog.locator('[data-testid="account-picker"]')
  await expect(picker).toBeVisible()

  // Wait for picker to have options loaded, then select our account by label.
  // `<option>` elements are not visible in Playwright's sense, so we check count.
  await expect(picker).toBeVisible({ timeout: 5000 })
  await expect(picker.locator(`option`, { hasText: `instagram/${handle}` })).toHaveCount(1, { timeout: 5000 })
  await picker.selectOption({ label: `instagram/${handle}` })

  await linkDialog.getByRole('button', { name: 'Link' }).click()

  // AC: BE: GET /v1/subjects/{sid}/accounts lists linked accounts
  // Verify the linked account appears in the dialog
  await expect(
    linkDialog.locator('[data-testid="linked-accounts-list"]'),
  ).toBeVisible({ timeout: 5000 })

  const linkedRow = linkDialog.locator(`[data-testid^="linked-account-row-"]`)
  await expect(linkedRow).toBeVisible()
  await expect(linkedRow).toContainText(handle)

  // AC: BE: DELETE /v1/subjects/{sid}/accounts/{aid} unlinks (join row only)
  const unlinkBtn = linkedRow.locator('[data-testid^="unlink-account-btn-"]')
  await unlinkBtn.click()

  // Account should disappear from linked list (but not be deleted)
  await expect(linkedRow).not.toBeVisible({ timeout: 5000 })
  await expect(linkDialog.locator('[data-testid="linked-accounts-list"]')).not.toBeVisible()

  // Close dialog
  await linkDialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(linkDialog).not.toBeVisible()

  // Verify account still exists (unlink doesn't delete account)
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()
  await expect(page.locator(`[data-testid="account-card-instagram-${handle}"]`)).toBeVisible()
})

// AC: Test linking same account to multiple subjects
test('same account can be linked to multiple subjects', async ({ page }) => {
  const ts = Date.now()
  const projectName = `E2E MultiSubj ${ts}`
  const subject1Name = `Subj1 ${ts}`
  const subject2Name = `Subj2 ${ts}`
  const handle = `multiuser${ts}`

  await createAccount(page, 'instagram', handle)
  await createProjectAndNavigate(page, projectName)
  await createSubject(page, subject1Name)
  await createSubject(page, subject2Name)

  // Link account to subject 1
  const card1 = page.locator('[data-testid^="subject-card-"]', { hasText: subject1Name })
  await card1.locator('[data-testid^="subject-menu-"]').click()
  await page.getByText('Link account', { exact: true }).click()
  const dialog = page.locator('[data-testid="link-account-dialog"]')
  await expect(dialog).toBeVisible()
  const picker = dialog.locator('[data-testid="account-picker"]')
  await picker.selectOption({ label: `instagram/${handle}` })
  await dialog.getByRole('button', { name: 'Link' }).click()
  await expect(dialog.locator('[data-testid^="linked-account-row-"]')).toBeVisible({ timeout: 5000 })
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(dialog).not.toBeVisible()

  // Link same account to subject 2
  const card2 = page.locator('[data-testid^="subject-card-"]', { hasText: subject2Name })
  await card2.locator('[data-testid^="subject-menu-"]').click()
  await page.getByText('Link account', { exact: true }).click()
  await expect(dialog).toBeVisible()
  await picker.selectOption({ label: `instagram/${handle}` })
  await dialog.getByRole('button', { name: 'Link' }).click()
  await expect(dialog.locator('[data-testid^="linked-account-row-"]')).toBeVisible({ timeout: 5000 })
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(dialog).not.toBeVisible()
})

// AC: Test unlinking doesn't delete the account itself
test('unlinking does not delete the account', async ({ page }) => {
  const ts = Date.now()
  const projectName = `E2E UnlinkKeep ${ts}`
  const subjectName = `UnlinkKeep Subj ${ts}`
  const handle = `keepuser${ts}`

  await createAccount(page, 'instagram', handle)
  await createProjectAndNavigate(page, projectName)
  await createSubject(page, subjectName)

  // Link
  const card = page.locator('[data-testid^="subject-card-"]', { hasText: subjectName })
  await card.locator('[data-testid^="subject-menu-"]').click()
  await page.getByText('Link account', { exact: true }).click()
  const dialog = page.locator('[data-testid="link-account-dialog"]')
  await expect(dialog).toBeVisible()
  await dialog.locator('[data-testid="account-picker"]').selectOption({ label: `instagram/${handle}` })
  await dialog.getByRole('button', { name: 'Link' }).click()
  await expect(dialog.locator('[data-testid^="linked-account-row-"]')).toBeVisible({ timeout: 5000 })

  // Unlink
  await dialog.locator('[data-testid^="unlink-account-btn-"]').click()
  await expect(dialog.locator('[data-testid^="linked-account-row-"]')).not.toBeVisible({ timeout: 5000 })
  await dialog.getByRole('button', { name: 'Cancel' }).click()

  // Account still exists in accounts list
  await page.goto('/accounts')
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()
  await expect(page.locator(`[data-testid="account-card-instagram-${handle}"]`)).toBeVisible()
})

// AC: FE: option to navigate to /accounts
test('navigate to accounts from link account dialog', async ({ page }) => {
  const ts = Date.now()
  const projectName = `E2E NavToAccounts ${ts}`
  const subjectName = `NavSubj ${ts}`

  await createProjectAndNavigate(page, projectName)
  await createSubject(page, subjectName)

  const card = page.locator('[data-testid^="subject-card-"]', { hasText: subjectName })
  await card.locator('[data-testid^="subject-menu-"]').click()
  await page.getByText('Link account', { exact: true }).click()

  const dialog = page.locator('[data-testid="link-account-dialog"]')
  await expect(dialog).toBeVisible()

  await dialog.locator('[data-testid="go-to-accounts-link"]').click()

  // Should navigate to /accounts
  await expect(page).toHaveURL('/accounts', { timeout: 5000 })
  await expect(page.locator('[data-testid="view-accounts"]')).toBeVisible()
})
