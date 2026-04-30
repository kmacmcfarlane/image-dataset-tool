---
name: playwright
description: End-to-end testing with Playwright. Use when user asks to write E2E tests, add browser tests, create smoke tests, test the running application, verify frontend rendering, or run Playwright. Covers test authoring, configuration, running tests, and debugging failures. Do NOT use for unit tests (use Vitest/Ginkgo directly).
disable-model-invocation: false
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
---

# Playwright E2E Testing

Playwright is a browser automation framework for end-to-end testing. It launches a real (headless) Chromium browser, navigates pages, interacts with elements, and asserts on outcomes across the full application stack.

Before writing tests, consult `references/patterns.md` for test patterns and `references/setup.md` for project setup and configuration details.

## Critical: Prerequisites

Before running any Playwright tests:

1. **Playwright must be installed** in the project. Check for `@playwright/test` in `package.json`. If missing, install it:
   ```bash
   npm install -D @playwright/test
   npx playwright install --with-deps chromium
   ```
2. **The application must be running.** Playwright tests run against a live app — they do not start the app themselves. Verify the app is up before running tests.
3. **A `playwright.config.ts` must exist.** If the project doesn't have one, create it using the patterns in `references/setup.md`.

## Workflow

### Step 1: Verify Environment

Check that the application is running and Playwright is installed:
- Look for `playwright.config.ts` in the project (check both project root and `frontend/`)
- Check `package.json` for `@playwright/test` dependency
- Verify the application is reachable at the configured `baseURL`

If not installed, guide the user through setup per `references/setup.md`.

### Step 2: Write Tests

Tests live in the directory specified by `testDir` in `playwright.config.ts` (typically `e2e/`).

Key patterns:
- **Page tests** use the `page` fixture to navigate and interact with the browser
- **API tests** use the `request` fixture to call endpoints directly (no browser needed)
- Both can coexist in the same test file
- Use `test.describe()` for grouping and `test.beforeEach()` for shared setup

```typescript
import { test, expect } from '@playwright/test';

test('page loads correctly', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('h1')).toBeVisible();
});

test('API responds', async ({ request }) => {
  const response = await request.get('/api/health');
  expect(response.ok()).toBeTruthy();
});
```

Consult `references/patterns.md` for detailed patterns including:
- Smoke tests (app loads, API reachable)
- Form interaction tests
- Navigation and routing tests
- API response validation
- Waiting for async content

### Step 3: Run Tests

```bash
npx playwright test                    # run all tests
npx playwright test --reporter=list    # plain-text output (best for CI/agents)
npx playwright test smoke.spec.ts      # run specific file
npx playwright test --grep "health"    # run tests matching pattern
```

### Step 4: Debug Failures

When tests fail:
- Check the test output for the assertion that failed and the expected vs actual values
- Use `--trace on` to capture a trace for every test, then inspect with `npx playwright show-trace`
- Add `await page.screenshot({ path: 'debug.png' })` before the failing assertion to see page state
- Check if the app is actually running and healthy
- Check the browser console for JavaScript errors: `page.on('console', msg => console.log(msg.text()))`

## Common Issues

### "Browser not installed"
Run `npx playwright install --with-deps chromium`. In Docker containers, the `--with-deps` flag requires root/sudo for system library installation. Alternative: use `--only-shell` for headless-only (smaller footprint).

### Tests hang or timeout
The application is likely not running or not reachable at the configured `baseURL`. Verify with `curl` before running tests.

### "Protocol error" or "Target closed"
Chromium sandbox issues in Docker. Add to `playwright.config.ts`:
```typescript
use: {
  launchOptions: {
    chromiumSandbox: false,
  },
},
```

### Tests pass locally but fail in CI/Docker
Usually missing system fonts or libraries. Install `fonts-liberation` for consistent text rendering. For headless-only environments, use `npx playwright install --only-shell chromium`.
