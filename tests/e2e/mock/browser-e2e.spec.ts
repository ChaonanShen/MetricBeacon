const { expect, test } = require('../../../apps/grafana-plugin/frontend/node_modules/@playwright/test') as typeof import('@playwright/test');

const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const chartTitles = ['CPU 使用率', '内存可用率', '系统负载'];

test('submits, restores, and renders the complete mock workbench', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
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
  await expect(page.getByTestId('timeseries-plot')).toHaveCount(3);
  await expect(page.getByText('PromQL', { exact: true })).toHaveCount(3);
  await expect(page.getByText('已加载', { exact: true })).toHaveCount(3);
  await expect(page.getByText('node-a:9100', { exact: true })).toHaveCount(3);
  await expect(page.getByText('node-b:9100', { exact: true })).toHaveCount(3);
  await expect(page.getByText('undefined', { exact: true })).toHaveCount(0);
  await expectPlotsWithinPanels(page);
  await expectNoHorizontalOverflow(page);

  await page.getByLabel('分析请求').fill('只看 CPU');
  await page.getByRole('button', { name: '开始分析' }).click();
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(6);
  await expect(page.getByText('你：只看 CPU')).toBeVisible();

  const widePanels = await panelBoxes(page);
  expect(new Set(widePanels.map((box) => Math.round(box.y))).size).toBe(1);

  await page.getByTestId('timeseries-panel').first().locator('summary').click();
  await expect(page.getByTestId('timeseries-panel').first().locator('pre')).toBeVisible();
  await expectPlotsWithinPanels(page);

  await page.setViewportSize({ width: 900, height: 900 });
  await expect(page.getByTestId('timeseries-plot').first()).toBeVisible();
  const narrowPanels = await panelBoxes(page);
  expect(narrowPanels[2].y).toBeGreaterThan(narrowPanels[0].y);
  await expectPlotsWithinPanels(page);
  await expectNoHorizontalOverflow(page);

  await page.reload();
  await expect(page.getByText('已生成 node_exporter 的 CPU、内存和系统负载视图。')).toBeVisible();
  for (const title of chartTitles) {
    await expect(page.getByText(title, { exact: true })).toBeVisible();
  }
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(6);
  await expect(page.getByText('undefined', { exact: true })).toHaveCount(0);
  await expectPlotsWithinPanels(page);
  await expectNoHorizontalOverflow(page);
});

async function expectPlotsWithinPanels(page: import('@playwright/test').Page) {
  const panels = page.getByTestId('timeseries-panel');
  const plots = page.getByTestId('timeseries-plot');
  for (let index = 0; index < await panels.count(); index++) {
    const panel = await panels.nth(index).boundingBox();
    const plot = await plots.nth(index).boundingBox();
    expect(panel).not.toBeNull();
    expect(plot).not.toBeNull();
    expect(plot!.x).toBeGreaterThanOrEqual(panel!.x);
    expect(plot!.y).toBeGreaterThanOrEqual(panel!.y);
    expect(plot!.x + plot!.width).toBeLessThanOrEqual(panel!.x + panel!.width + 1);
    expect(plot!.y + plot!.height).toBeLessThanOrEqual(panel!.y + panel!.height + 1);
  }
}

async function panelBoxes(page: import('@playwright/test').Page) {
  const panels = page.getByTestId('timeseries-panel');
  const boxes = await Promise.all(Array.from({ length: await panels.count() }, (_, index) => panels.nth(index).boundingBox()));
  if (boxes.some((box) => box === null)) {
    throw new Error('chart panel is not visible');
  }
  return boxes as Array<NonNullable<(typeof boxes)[number]>>;
}

async function expectNoHorizontalOverflow(page: import('@playwright/test').Page) {
  const viewport = await page.evaluate(() => ({ clientWidth: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth }));
  expect(viewport.scrollWidth).toBeLessThanOrEqual(viewport.clientWidth);
}
