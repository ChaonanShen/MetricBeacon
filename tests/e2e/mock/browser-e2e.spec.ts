const { expect, test } = require('../../../apps/grafana-plugin/frontend/node_modules/@playwright/test') as typeof import('@playwright/test');

const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const realMetrics = process.env.REAL_METRICS === '1';
const chartTitles = ['CPU 使用率', '内存可用率', '系统负载'];

test('submits, restores, and renders the complete mock workbench', async ({ page }) => {
  await page.setViewportSize({ width: 1800, height: 900 });
  const login = await page.context().request.post('/login', { data: { user, password } });
  expect(login.ok()).toBeTruthy();

  await page.goto('/a/mini-torchbearing-app/workbench');
  await expect(page.getByRole('heading', { name: 'Mini Torchbearing Workbench' })).toBeVisible();
  await expect(page.getByLabel('默认时间范围')).toHaveCount(0);
  await expect(page.getByLabel('采样分辨率')).toHaveCount(0);
  let submittedTask: Record<string, any> | undefined;
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().includes('/resources/tasks')) submittedTask = request.postDataJSON();
  });
  await page.getByLabel('分析请求').fill('帮我看看最近 5 分钟 node_exporter 的 CPU、内存和系统负载，每隔 5s 一个点');
  await page.getByRole('button', { name: '开始分析' }).click();

  await expect(page).toHaveURL(/sessionId=[^&]+&taskId=[^&]+/);
  const originalSessionId = new URL(page.url()).searchParams.get('sessionId');
  expect(originalSessionId).toBeTruthy();
  expect(submittedTask?.analysisContext).toEqual({ datasourceUid: 'prometheus-main' });
  await expect(page.getByText(/已查询 node_exporter/)).toBeVisible();
  const chartCanvas = page.getByTestId('chart-canvas');
  for (const title of chartTitles) {
    await expect(chartCanvas.getByText(title, { exact: true })).toBeVisible();
  }
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(3);
  await expect(page.getByTestId('timeseries-plot')).toHaveCount(3);
  await expect(chartCanvas.getByText('PromQL', { exact: true })).toHaveCount(3);
  await expect(chartCanvas.getByText('已加载', { exact: true })).toHaveCount(3);
  if (!realMetrics) {
    await expect(page.getByText('node-a:9100', { exact: true })).toHaveCount(3);
    await expect(page.getByText('node-b:9100', { exact: true })).toHaveCount(3);
  }
  await expect(page.getByText('undefined', { exact: true })).toHaveCount(0);
  await expectPlotsWithinPanels(page);
  await expectNoHorizontalOverflow(page);

  await page.getByLabel('分析请求').fill('只看 CPU');
  await page.getByRole('button', { name: '开始分析' }).click();
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(4);
  await expect(page.getByText('你：只看 CPU')).toBeVisible();

  const groups = page.getByTestId('chart-group');
  await expect(groups).toHaveCount(2);
  await expect(groups.nth(0)).toContainText('帮我看看最近 5 分钟');
  await expect(groups.nth(1)).toContainText('只看 CPU');
  await expect(groups.nth(0).getByTestId('timeseries-panel')).toHaveCount(3);
  await expect(groups.nth(1).getByTestId('timeseries-panel')).toHaveCount(1);

  await expectThreePaneDesktopLayout(page);
  const widePanels = await panelBoxes(page);
  expect(new Set(widePanels.map((box) => Math.round(box.x))).size).toBeLessThanOrEqual(2);
  expect(widePanels[2].y).toBeGreaterThan(widePanels[0].y);
  expect(widePanels[2].width).toBeGreaterThan(widePanels[0].width * 1.8);
  expect(widePanels[3].y).toBeGreaterThan(widePanels[2].y + widePanels[2].height);

  await page.getByRole('button', { name: '详情' }).nth(1).click();
  const contextPane = page.getByTestId('context-pane');
  await expect(contextPane.getByText('内存可用率', { exact: true })).toBeVisible();
  await expect(contextPane.getByText('PromQL', { exact: true })).toBeVisible();
  await expect(contextPane.getByText('序列数', { exact: true })).toBeVisible();
  await expect(contextPane.getByText('有效采样间隔', { exact: true })).toBeVisible();
  await expect(contextPane.getByText('CPU rate window', { exact: true })).toBeVisible();
  await expect(contextPane.getByText('实际样本范围', { exact: true })).toBeVisible();
  await expect(contextPane.getByText(realMetrics ? '1' : '2', { exact: true })).toBeVisible();

  await page.getByTestId('timeseries-panel').first().locator('summary').click();
  await expect(page.getByTestId('timeseries-panel').first().locator('pre')).toBeVisible();
  await expectPlotsWithinPanels(page);

  await page.setViewportSize({ width: 900, height: 900 });
  await expect(page.getByTestId('timeseries-plot').first()).toBeVisible();
  await expectThreePaneNarrowLayout(page);
  const narrowPanels = await panelBoxes(page);
  expect(narrowPanels[2].y).toBeGreaterThan(narrowPanels[0].y);
  await expectPlotsWithinPanels(page);
  await expectNoHorizontalOverflow(page);

  await page.reload();
  await expect(page.getByText(/已查询 node_exporter/).first()).toBeVisible();
  for (const title of chartTitles) {
    await expect(page.getByText(title, { exact: true }).first()).toBeVisible();
  }
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(4);
  await expect(page.getByTestId('chart-group')).toHaveCount(2);
  await expect(page.getByTestId('chart-group').nth(0)).toContainText('帮我看看最近 5 分钟');
  await expect(page.getByTestId('chart-group').nth(1)).toContainText('只看 CPU');
  await expect(page.getByText('undefined', { exact: true })).toHaveCount(0);
  await expectPlotsWithinPanels(page);
  await expectNoHorizontalOverflow(page);

  await page.setViewportSize({ width: 1800, height: 900 });
  await page.getByRole('button', { name: '新建对话' }).click();
  await expect(page).toHaveURL((url) => !url.searchParams.has('sessionId') && !url.searchParams.has('taskId'));
  await expect(page.getByLabel('分析请求')).toHaveValue('');
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(0);
  await expect(page.getByTestId('chart-group')).toHaveCount(0);
  await expect(page.getByText('你：只看 CPU')).toHaveCount(0);
  await expect(page.getByText('0 轮分析 · 0 张图表')).toBeVisible();
  await expect(contextPane.getByText('尚未开始', { exact: true })).toBeVisible();
  await expect(contextPane.getByText('选择一张图表以查看只读详情。')).toBeVisible();
  const oldSession = await page.context().request.get(`/api/plugins/mini-torchbearing-app/resources/sessions/${encodeURIComponent(originalSessionId!)}`);
  expect(oldSession.ok()).toBeTruthy();

  await page.getByLabel('分析请求').fill('新对话只看内存');
  await page.getByRole('button', { name: '开始分析' }).click();
  await expect(page).toHaveURL(/sessionId=[^&]+&taskId=[^&]+/);
  const freshSessionId = new URL(page.url()).searchParams.get('sessionId');
  expect(freshSessionId).toBeTruthy();
  expect(freshSessionId).not.toBe(originalSessionId);
  await expect(page.getByTestId('chart-group')).toHaveCount(1);
  await expect(page.getByTestId('chart-group')).toContainText('新对话只看内存');
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(1);
  await expect(page.getByText('你：只看 CPU')).toHaveCount(0);

  await page.goto('/a/mini-torchbearing-app/workbench?theme=dark&sessionId=missing-session&taskId=missing-task');
  await expect(page).toHaveURL((url) => url.searchParams.get('theme') === 'dark' && !url.searchParams.has('sessionId') && !url.searchParams.has('taskId'));
  await expect(page.getByRole('status')).toHaveText('已清除当前运行环境中不存在的旧会话，请重新提交分析。');
  await page.getByLabel('分析请求').fill('从当前运行模式重新分析 node_exporter');
  await page.getByRole('button', { name: '开始分析' }).click();
  await expect(page).toHaveURL(/sessionId=[^&]+&taskId=[^&]+/);
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(3);
  await expect(page.getByRole('alert')).toHaveCount(0);
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

async function expectThreePaneDesktopLayout(page: import('@playwright/test').Page) {
  const [conversation, canvas, context] = await Promise.all([
    page.getByTestId('conversation-pane').boundingBox(),
    page.getByTestId('chart-canvas').boundingBox(),
    page.getByTestId('context-pane').boundingBox(),
  ]);
  expect(conversation).not.toBeNull();
  expect(canvas).not.toBeNull();
  expect(context).not.toBeNull();
  expect(conversation!.y).toBeCloseTo(canvas!.y, 0);
  expect(canvas!.y).toBeCloseTo(context!.y, 0);
  expect(conversation!.x).toBeLessThan(canvas!.x);
  expect(canvas!.x).toBeLessThan(context!.x);
}

async function expectThreePaneNarrowLayout(page: import('@playwright/test').Page) {
  const [conversation, canvas, context] = await Promise.all([
    page.getByTestId('conversation-pane').boundingBox(),
    page.getByTestId('chart-canvas').boundingBox(),
    page.getByTestId('context-pane').boundingBox(),
  ]);
  expect(conversation).not.toBeNull();
  expect(canvas).not.toBeNull();
  expect(context).not.toBeNull();
  expect(conversation!.y).toBeLessThan(canvas!.y);
  expect(canvas!.y).toBeLessThan(context!.y);
}

async function expectNoHorizontalOverflow(page: import('@playwright/test').Page) {
  const viewport = await page.evaluate(() => ({ clientWidth: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth }));
  expect(viewport.scrollWidth).toBeLessThanOrEqual(viewport.clientWidth);
}
