import { test, expect } from '@playwright/test'

/**
 * Subject CRUD E2E tests.
 *
 * Prerequisites: dev stack must be running (make up-dev).
 * Run: make test-e2e SPEC=subjects.spec.ts
 */

// Helper: create a project and navigate to its detail page.
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

  // Wait for project card and click to navigate
  const projectCard = page.locator('[data-testid^="project-card-"]', { hasText: projectName })
  await expect(projectCard).toBeVisible({ timeout: 5000 })
  await projectCard.click()

  // Wait for project detail view
  await expect(page.locator('[data-testid="view-project-detail"]')).toBeVisible({ timeout: 5000 })
}

// AC: FE: Subject list at /projects/:id
test('subject list page loads with empty state', async ({ page }) => {
  const projectName = `E2E Sub List ${Date.now()}`
  await createProjectAndNavigate(page, projectName)

  await expect(page.locator('[data-testid="subjects-empty"]')).toBeVisible()
  await expect(page.locator('[data-testid="create-subject-btn"]')).toBeVisible()
})

// AC: Full CRUD with rename round trip
test('create, read, update (rename), delete subject round trip', async ({ page }) => {
  const projectName = `E2E Sub CRUD ${Date.now()}`
  const subjectName = `Test Subject ${Date.now()}`
  await createProjectAndNavigate(page, projectName)

  // --- CREATE ---
  // AC: BE: POST /v1/projects/{pid}/subjects creates subject
  await page.locator('[data-testid="create-subject-btn"]').click()
  const dialog = page.locator('[data-testid="subject-dialog"]')
  await expect(dialog).toBeVisible()

  await page.locator('[data-testid="subject-name-input"]').fill(subjectName)
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).not.toBeVisible()

  // --- READ ---
  // AC: BE: GET /v1/projects/{pid}/subjects lists subjects with sample counts
  // AC: FE: cards with sample count badges
  const subjectCard = page.locator('[data-testid^="subject-card-"]', { hasText: subjectName })
  await expect(subjectCard).toBeVisible({ timeout: 5000 })

  // Verify sample count badge shows 0
  const sampleBadge = subjectCard.locator('[data-testid^="subject-sample-count-"]')
  await expect(sampleBadge).toHaveText('0 samples')

  // --- UPDATE (rename) ---
  // AC: BE: PUT /v1/projects/{pid}/subjects/{sid} updates name; renames dir if slug changes
  const menuBtn = subjectCard.locator('[data-testid^="subject-menu-"]')
  await menuBtn.click()
  await page.getByText('Edit', { exact: true }).click()

  const editDialog = page.locator('[data-testid="subject-dialog"]')
  await expect(editDialog).toBeVisible()

  const nameInput = page.locator('[data-testid="subject-name-input"]')
  await expect(nameInput).toHaveValue(subjectName)

  const updatedName = `${subjectName} Renamed`
  await nameInput.fill(updatedName)
  await editDialog.getByRole('button', { name: 'Save' }).click()
  await expect(editDialog).not.toBeVisible()

  // Verify renamed card appears
  const updatedCard = page.locator('[data-testid^="subject-card-"]', { hasText: updatedName })
  await expect(updatedCard).toBeVisible({ timeout: 5000 })

  // --- DELETE ---
  // AC: BE: DELETE same soft/hard pattern as projects
  const updatedMenuBtn = updatedCard.locator('[data-testid^="subject-menu-"]')
  await updatedMenuBtn.click()
  await page.getByText('Delete', { exact: true }).click()

  const deleteDialog = page.locator('[data-testid="delete-subject-dialog"]')
  await expect(deleteDialog).toBeVisible()

  await deleteDialog.getByRole('button', { name: 'Delete' }).click()
  await expect(deleteDialog).not.toBeVisible()
  await expect(updatedCard).not.toBeVisible({ timeout: 5000 })
})

// AC: Empty name shows warning
test('empty subject name shows warning', async ({ page }) => {
  const projectName = `E2E Sub Empty ${Date.now()}`
  await createProjectAndNavigate(page, projectName)

  await page.locator('[data-testid="create-subject-btn"]').click()
  const dialog = page.locator('[data-testid="subject-dialog"]')
  await expect(dialog).toBeVisible()

  // Try to save with empty name — dialog stays open
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeVisible()
})

// AC: BE: DELETE with hard_delete checkbox
test('delete subject with hard delete checkbox', async ({ page }) => {
  const projectName = `E2E Sub Hard Del ${Date.now()}`
  const subjectName = `Hard Del Subject ${Date.now()}`
  await createProjectAndNavigate(page, projectName)

  // Create a subject
  await page.locator('[data-testid="create-subject-btn"]').click()
  const dialog = page.locator('[data-testid="subject-dialog"]')
  await expect(dialog).toBeVisible()
  await page.locator('[data-testid="subject-name-input"]').fill(subjectName)
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).not.toBeVisible()

  const subjectCard = page.locator('[data-testid^="subject-card-"]', { hasText: subjectName })
  await expect(subjectCard).toBeVisible({ timeout: 5000 })

  // Open context menu and delete
  const menuBtn = subjectCard.locator('[data-testid^="subject-menu-"]')
  await menuBtn.click()
  await page.getByText('Delete', { exact: true }).click()

  const deleteDialog = page.locator('[data-testid="delete-subject-dialog"]')
  await expect(deleteDialog).toBeVisible()

  // Check hard delete checkbox
  await page.locator('[data-testid="hard-delete-checkbox"]').check()
  await deleteDialog.getByRole('button', { name: 'Delete' }).click()

  await expect(deleteDialog).not.toBeVisible()
  await expect(subjectCard).not.toBeVisible({ timeout: 5000 })
})
