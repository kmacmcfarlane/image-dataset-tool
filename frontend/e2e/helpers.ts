import { Page } from '@playwright/test'

/**
 * The localStorage key used by the router guard to determine if setup is complete.
 * Must match SETUP_COMPLETE_KEY in src/constants.ts.
 */
export const SETUP_COMPLETE_KEY = 'setup_wizard_complete'

/**
 * Mark setup as complete in localStorage so the router guard does not redirect
 * to /setup. Call this before navigating in any test that is NOT testing the
 * setup wizard itself.
 *
 * Strategy: navigate to /setup first (skipSetupGuard: true, loads without
 * redirect), set the flag, then let the caller do their real navigation.
 *
 * Usage: call inside beforeEach.
 */
export async function bypassSetupWizard(page: Page): Promise<void> {
  // /setup loads without the guard so we can safely set localStorage.
  await page.goto('/setup')
  await page.evaluate((key) => localStorage.setItem(key, 'true'), SETUP_COMPLETE_KEY)
}
