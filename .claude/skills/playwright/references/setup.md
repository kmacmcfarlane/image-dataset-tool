# Playwright Setup Reference

## Installation

```bash
# From the directory that will contain playwright.config.ts
npm install -D @playwright/test

# Install headless Chromium + system dependencies
npx playwright install --with-deps chromium

# Smaller install for CI/headless-only (no full browser, ~150MB vs ~400MB)
npx playwright install --with-deps --only-shell chromium
```

## Configuration

### Minimal config for a project with a separately-running app

This is the most common pattern: the app is started independently (e.g. `docker compose up` or `make up-dev`) and Playwright connects to it.

```typescript
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? 'list' : 'html',

  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
```

### With global setup that waits for the app

Use this when you want tests to wait until the app is healthy before running, but don't want Playwright to manage the app lifecycle.

```typescript
// e2e/global-setup.ts
import { request } from '@playwright/test';

export default async function globalSetup() {
  const maxRetries = 30;
  const delay = 2000;

  for (let i = 0; i < maxRetries; i++) {
    try {
      const ctx = await request.newContext({ baseURL: 'http://localhost:3000' });
      const response = await ctx.get('/api/health');
      await ctx.dispose();
      if (response.ok()) return;
    } catch {
      // App not ready yet
    }
    await new Promise(r => setTimeout(r, delay));
  }
  throw new Error('Application did not become ready within 60 seconds');
}
```

Then reference it in config:
```typescript
globalSetup: require.resolve('./e2e/global-setup'),
```

### With webServer (Playwright manages the app)

Use this when Playwright should start the app before tests and stop it after. Works best with simple `npm run dev` style commands, less ideal with `docker compose`.

```typescript
webServer: {
  command: 'npm run dev',
  url: 'http://localhost:3000',
  reuseExistingServer: !process.env.CI,
  timeout: 30_000,
},
```

**Note:** `webServer` expects a foreground process it can kill on teardown. It does not work well with `docker compose up -d` since that returns immediately and runs in the background.

## Docker/Headless Environment Considerations

### System dependencies
Chromium requires system libraries (libglib, libnss, libatk, libcups, etc.). The `--with-deps` flag handles this via `apt-get`, which requires root/sudo.

If you can't install system deps at runtime, pre-bake them into the Docker image:
```dockerfile
RUN npx -y playwright@1.52.0 install-deps chromium
```

### No display server needed
Playwright runs headless by default. No Xvfb or `DISPLAY` variable required.

### Chromium sandboxing in Docker
If running as root or in a restricted container, disable Chromium's sandbox:
```typescript
use: {
  launchOptions: {
    chromiumSandbox: false,
  },
},
```

### Font rendering
Minimal Docker images may lack fonts. Install `fonts-liberation` and `fonts-noto-color-emoji` for consistent rendering. Only matters for visual regression tests, not functional tests.

## Package.json Scripts

```json
{
  "scripts": {
    "test:e2e": "npx playwright test",
    "test:e2e:list": "npx playwright test --reporter=list",
    "test:e2e:debug": "npx playwright test --debug",
    "test:e2e:trace": "npx playwright test --trace on"
  }
}
```

## Makefile Integration

```makefile
test-e2e:
	cd frontend && npx playwright test --reporter=list

test-e2e-install:
	cd frontend && npx playwright install --with-deps chromium
```

## .gitignore Additions

```
playwright-report/
test-results/
blob-report/
```
