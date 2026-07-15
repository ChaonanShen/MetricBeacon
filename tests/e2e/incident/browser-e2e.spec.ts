const { expect, test } = require('../../../apps/grafana-plugin/frontend/node_modules/@playwright/test') as typeof import('@playwright/test');

const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';

test('restores the completed Incident evidence, diff, approval and durable timeline', async ({ page }) => {
  await page.setViewportSize({ width: 1800, height: 900 });
  const login = await page.context().request.post('/login', { data: { user, password } });
  expect(login.ok()).toBeTruthy();

  await page.goto('/a/mini-torchbearing-app/workbench');
  await page.getByRole('button', { name: '事件', exact: true }).click();
  await expect(page.getByTestId('session-scroll-container').getByRole('button', { name: '组织事件', exact: true })).toHaveClass(/is-current/);
  const item = page.locator('.mtb-session-item').filter({ hasText: 'OrderQueueBacklog · order-demo' });
  await expect(item).toBeVisible();
  await item.click();

  await expect(page).toHaveURL(/sessionId=[^&]+/);
  const canvas = page.getByTestId('incident-canvas');
  await expect(canvas).toBeVisible();
  await expect(canvas).toContainText('worker_stopped');
  await expect(canvas).toContainText('slow_processing');
  await expect(canvas).toContainText('dependency_errors');
  await expect(canvas).toContainText('order_service.get_worker_state');
  await expect(canvas).toContainText('worker concurrency');
  await expect(canvas).toContainText('order_service.restore_worker_concurrency');
  await expect(canvas).toContainText('状态：approved');
  await expect(canvas.getByRole('button', { name: '批准并执行' })).toHaveCount(0);
  await expect(canvas.getByRole('button', { name: '拒绝' })).toHaveCount(0);

  for (const step of ['Prometheus 告警已接收', '已匹配版本化 Playbook', '只读诊断完成', '受控修复 Intent/Diff 已生成', '审批决定已持久化', '类型化修复开始', '执行回执已核对', '运行状态验证通过', 'Prometheus 恢复指标验证通过', '真实订单业务探针通过']) {
    await expect(canvas.getByText(step, { exact: true })).toBeVisible();
  }
  await expect(page.getByLabel('分析请求')).toHaveCount(0);

  await page.reload();
  await expect(page.getByTestId('incident-canvas')).toContainText('状态：approved');
  await expect(page.getByText('真实订单业务探针通过', { exact: true })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});
