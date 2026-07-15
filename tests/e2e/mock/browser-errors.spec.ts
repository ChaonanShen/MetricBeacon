const { expect, test } = require('../../../apps/grafana-plugin/frontend/node_modules/@playwright/test') as typeof import('@playwright/test');

const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const taskURL = '**/api/plugins/mini-torchbearing-app/resources/tasks';

test('keeps retry identity and session context after a dependency error', async ({ page }) => {
  const login = await page.context().request.post('/login', { data: { user, password } });
  expect(login.ok()).toBeTruthy();
  let failedTaskRequests = 0;
  let failedIdempotencyKey: string | undefined;
  await page.route(taskURL, async (route) => {
    failedTaskRequests++;
    failedIdempotencyKey = route.request().headers()['idempotency-key'];
    await route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ error: { code: 'dependency_unavailable', message: 'AI Core unavailable', retryable: true, requestId: 'browser-error-test' } }),
    });
  });

  await page.goto('/a/mini-torchbearing-app/workbench?theme=dark');
  await page.getByLabel('分析请求').fill('只看 CPU');
  await page.getByRole('button', { name: '开始分析' }).click();
  await expect(page.getByRole('alert')).toContainText('dependency_unavailable: AI Core unavailable');
  await expect(page).toHaveURL((url) => !url.searchParams.has('taskId') && url.searchParams.get('theme') === 'dark');
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(0);
  expect(failedTaskRequests).toBe(1);
  expect(failedIdempotencyKey).toBeTruthy();

  await page.unroute(taskURL);
  let retryIdempotencyKey: string | undefined;
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().endsWith('/resources/tasks')) retryIdempotencyKey = request.headers()['idempotency-key'];
  });
  await page.getByRole('button', { name: '开始分析' }).click();
  await expect(page).toHaveURL(/taskId=[^&]+/);
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(1);
  expect(retryIdempotencyKey).toBe(failedIdempotencyKey);
});
