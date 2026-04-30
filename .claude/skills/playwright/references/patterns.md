# Playwright Test Patterns

## Smoke Tests

Minimal tests that verify the application is alive and functional. These should be fast and stable — the first tests to run in any E2E suite.

### App loads and renders

```typescript
import { test, expect } from '@playwright/test';

test('app loads and renders', async ({ page }) => {
  await page.goto('/');
  // Verify the page title or a known root element
  await expect(page).toHaveTitle(/My App/);
  // Verify the app shell renders (not just a blank page)
  await expect(page.locator('#app')).toBeVisible();
});
```

### Health endpoint via API context

```typescript
test('health endpoint returns 200', async ({ request }) => {
  const response = await request.get('/api/health');
  expect(response.ok()).toBeTruthy();
  expect(response.status()).toBe(200);
});
```

### API endpoint returns valid JSON

```typescript
test('API returns valid data', async ({ request }) => {
  const response = await request.get('/api/items');
  expect(response.ok()).toBeTruthy();

  const body = await response.json();
  expect(Array.isArray(body)).toBeTruthy();
});
```

## Page Interaction Tests

### Click and navigate

```typescript
test('navigate to detail page', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('link', { name: 'Settings' }).click();
  await expect(page).toHaveURL(/.*settings/);
  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
});
```

### Fill a form and submit

```typescript
test('create a new item', async ({ page }) => {
  await page.goto('/items/new');
  await page.getByLabel('Name').fill('Test Item');
  await page.getByLabel('Description').fill('A test item');
  await page.getByRole('button', { name: 'Save' }).click();

  // Verify success state
  await expect(page.getByText('Item created')).toBeVisible();
});
```

### Select from a dropdown

```typescript
test('select an option', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('combobox', { name: 'Category' }).selectOption('electronics');
  // For custom dropdowns (like Naive UI NSelect), click to open then click option:
  // await page.getByTestId('category-select').click();
  // await page.getByText('Electronics').click();
});
```

## Waiting for Async Content

### Wait for network response

```typescript
test('data loads after selection', async ({ page }) => {
  await page.goto('/');

  // Wait for the API call to complete after an action
  const responsePromise = page.waitForResponse(resp =>
    resp.url().includes('/api/items') && resp.status() === 200
  );
  await page.getByRole('button', { name: 'Load' }).click();
  await responsePromise;

  // Now assert on the loaded content
  await expect(page.getByTestId('item-list')).not.toBeEmpty();
});
```

### Wait for element to appear

```typescript
test('loading spinner resolves', async ({ page }) => {
  await page.goto('/');
  // Wait for loading to finish (spinner disappears, content appears)
  await expect(page.getByTestId('loading')).toBeHidden({ timeout: 10_000 });
  await expect(page.getByTestId('content')).toBeVisible();
});
```

## API Testing Patterns

Playwright's `request` fixture sends HTTP requests directly (no browser). Useful for testing backend endpoints.

### POST with JSON body

```typescript
test('create item via API', async ({ request }) => {
  const response = await request.post('/api/items', {
    data: {
      name: 'Test',
      value: 42,
    },
  });
  expect(response.status()).toBe(201);

  const body = await response.json();
  expect(body.id).toBeTruthy();
  expect(body.name).toBe('Test');
});
```

### Verify error responses

```typescript
test('invalid request returns 400', async ({ request }) => {
  const response = await request.post('/api/items', {
    data: { /* missing required fields */ },
  });
  expect(response.status()).toBe(400);

  const body = await response.json();
  expect(body.message).toBeTruthy();
});
```

### Test endpoint with path params

```typescript
test('get item by ID', async ({ request }) => {
  const response = await request.get('/api/items/123');
  expect(response.ok()).toBeTruthy();

  const body = await response.json();
  expect(body.id).toBe('123');
});
```

## Test Organization

### Group related tests

```typescript
test.describe('Item Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/items');
  });

  test('lists existing items', async ({ page }) => {
    await expect(page.getByTestId('item-list')).toBeVisible();
  });

  test('creates a new item', async ({ page }) => {
    // ...
  });

  test('deletes an item', async ({ page }) => {
    // ...
  });
});
```

### Separate smoke tests from feature tests

```
e2e/
  smoke.spec.ts        # Fast, always-run: app loads, health check, core API
  items.spec.ts        # Feature-specific: item CRUD workflow
  settings.spec.ts     # Feature-specific: settings page
```

## Locator Best Practices

Prefer these locators (most stable to least):
1. `page.getByRole()` — semantic, accessible
2. `page.getByText()` — user-visible text
3. `page.getByLabel()` — form labels
4. `page.getByTestId()` — explicit test hooks (`data-testid` attribute)
5. `page.locator('.class')` — CSS selectors (more brittle)

Avoid:
- XPath selectors
- Deep CSS selectors tied to component structure
- Index-based selectors (`nth-child`)
