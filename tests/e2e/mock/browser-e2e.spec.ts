const { expect, test } = require('../../../apps/grafana-plugin/frontend/node_modules/@playwright/test') as typeof import('@playwright/test');

const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const chartTitles = ['CPU 使用率', '内存可用率', '系统负载'];

test('submits, restores, and renders the complete mock workbench', async ({ page }) => {
  const login = await page.context().request.post('/login', { data: { user, password } });
  expect(login.ok()).toBeTruthy();

  await page.goto('/a/mini-torchbearing-app/workbench');
  await expect(page.getByRole('heading', { name: 'Mini Torchbearing Workbench' })).toBeVisible();
  await page.getByLabel('分析请求').fill('帮我看看 node_exporter 最近 30 分钟的 CPU、内存和系统负载');
  await page.getByRole('button', { name: '开始分析' }).click();

  await expect(page).toHaveURL(/sessionId=[^&]+&taskId=[^&]+/);
  await expect(page.getByText('已生成 node_exporter 的 CPU、内存和系统负载视图。')).toBeVisible();
  for (const title of chartTitles) {
    await expect(page.getByText(title, { exact: true })).toBeVisible();
  }
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(3);
  await expect(page.getByText('PromQL', { exact: true })).toHaveCount(3);

  await page.reload();
  await expect(page.getByText('已生成 node_exporter 的 CPU、内存和系统负载视图。')).toBeVisible();
  for (const title of chartTitles) {
    await expect(page.getByText(title, { exact: true })).toBeVisible();
  }
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(3);
});
