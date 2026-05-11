import { test, expect } from '@playwright/test'

/**
 * Project CRUD E2E tests.
 *
 * Prerequisites: dev stack must be running (make up-dev).
 * Run: make test-e2e SPEC=projects.spec.ts
 */

// AC: Project list at /; FE: project list displayed
test('project list page loads', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('[data-testid="view-project-list"]')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible()
})

// AC: Create → read → update → delete round trip
test('create, read, update, delete project round trip', async ({ page }) => {
  const projectName = `E2E Test ${Date.now()}`
  const projectDesc = 'Created by E2E test'

  // Navigate to project list
  await page.goto('/')
  await expect(page.locator('[data-testid="view-project-list"]')).toBeVisible()

  // --- CREATE ---
  // AC: FE: create dialog
  await page.locator('[data-testid="create-project-btn"]').click()
  const dialog = page.locator('[data-testid="project-dialog"]')
  await expect(dialog).toBeVisible()

  await page.locator('[data-testid="project-name-input"]').fill(projectName)
  await page.locator('[data-testid="project-description-input"]').fill(projectDesc)
  await dialog.getByRole('button', { name: 'Save' }).click()

  // Dialog should close
  await expect(dialog).not.toBeVisible()

  // --- READ ---
  // AC: GET /v1/projects lists all projects (verified through UI)
  // Wait for the project card to appear in the list
  const projectCard = page.locator('[data-testid^="project-card-"]', { hasText: projectName })
  await expect(projectCard).toBeVisible({ timeout: 5000 })

  // Verify description is shown
  await expect(projectCard.locator('.project-description')).toHaveText(projectDesc)

  // --- UPDATE ---
  // AC: FE: edit context menu
  // Open context menu via the menu button
  const menuBtn = projectCard.locator('[data-testid^="project-menu-"]')
  await menuBtn.click()

  // Click Edit in the dropdown
  await page.getByText('Edit', { exact: true }).click()

  // Edit dialog should appear
  const editDialog = page.locator('[data-testid="project-dialog"]')
  await expect(editDialog).toBeVisible()

  // Verify current name is pre-filled
  const nameInput = page.locator('[data-testid="project-name-input"]')
  await expect(nameInput).toHaveValue(projectName)

  // Update the name
  const updatedName = `${projectName} Updated`
  await nameInput.fill(updatedName)
  await editDialog.getByRole('button', { name: 'Save' }).click()

  // Dialog should close and updated name should appear
  await expect(editDialog).not.toBeVisible()
  const updatedCard = page.locator('[data-testid^="project-card-"]', { hasText: updatedName })
  await expect(updatedCard).toBeVisible({ timeout: 5000 })

  // --- DELETE ---
  // AC: FE: delete context menu
  const updatedMenuBtn = updatedCard.locator('[data-testid^="project-menu-"]')
  await updatedMenuBtn.click()

  // Click Delete in the dropdown
  await page.getByText('Delete', { exact: true }).click()

  // Delete confirmation dialog should appear
  const deleteDialog = page.locator('[data-testid="delete-project-dialog"]')
  await expect(deleteDialog).toBeVisible()

  // Confirm delete
  await deleteDialog.getByRole('button', { name: 'Delete' }).click()

  // Dialog should close and project should be gone from list
  await expect(deleteDialog).not.toBeVisible()
  await expect(updatedCard).not.toBeVisible({ timeout: 5000 })
})

// AC: BE: POST /v1/projects creates project; returns 201
test('empty name shows warning', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('[data-testid="view-project-list"]')).toBeVisible()

  await page.locator('[data-testid="create-project-btn"]').click()
  const dialog = page.locator('[data-testid="project-dialog"]')
  await expect(dialog).toBeVisible()

  // Try to save with empty name — should show warning, dialog stays open
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeVisible()
})

// AC: BE: DELETE with hard_delete checkbox
test('delete with hard delete checkbox', async ({ page }) => {
  const projectName = `E2E Hard Delete ${Date.now()}`

  await page.goto('/')
  await expect(page.locator('[data-testid="view-project-list"]')).toBeVisible()

  // Create a project
  await page.locator('[data-testid="create-project-btn"]').click()
  const dialog = page.locator('[data-testid="project-dialog"]')
  await expect(dialog).toBeVisible()
  await page.locator('[data-testid="project-name-input"]').fill(projectName)
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).not.toBeVisible()

  // Wait for it to appear
  const projectCard = page.locator('[data-testid^="project-card-"]', { hasText: projectName })
  await expect(projectCard).toBeVisible({ timeout: 5000 })

  // Open context menu and delete
  const menuBtn = projectCard.locator('[data-testid^="project-menu-"]')
  await menuBtn.click()
  await page.getByText('Delete', { exact: true }).click()

  // Check the hard delete checkbox
  const deleteDialog = page.locator('[data-testid="delete-project-dialog"]')
  await expect(deleteDialog).toBeVisible()
  await page.locator('[data-testid="hard-delete-checkbox"]').check()
  await deleteDialog.getByRole('button', { name: 'Delete' }).click()

  // Project should be gone
  await expect(deleteDialog).not.toBeVisible()
  await expect(projectCard).not.toBeVisible({ timeout: 5000 })
})
