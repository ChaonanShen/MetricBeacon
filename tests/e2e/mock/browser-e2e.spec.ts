const { expect, test } = require('../../../apps/grafana-plugin/frontend/node_modules/@playwright/test') as typeof import('@playwright/test');

const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const realMetrics = process.env.REAL_METRICS === '1';
const chartTitles = ['CPU 使用率', '内存可用率', '系统负载'];

test('submits, restores, and renders the complete mock workbench', async ({ page }) => {
  await page.setViewportSize({ width: 1800, height: 900 });
  let sessionPostCount = 0;
  let taskPostCount = 0;
  let submittedTask: Record<string, any> | undefined;
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().includes('/resources/sessions')) sessionPostCount++;
    if (request.method() === 'POST' && request.url().includes('/resources/tasks')) {
      taskPostCount++;
      submittedTask = request.postDataJSON();
    }
  });
  const login = await page.context().request.post('/login', { data: { user, password } });
  expect(login.ok()).toBeTruthy();

  await page.goto('/a/mini-torchbearing-app/workbench');
  await expect(page.getByRole('heading', { name: '指标分析工作台', level: 1 })).toBeVisible();
  await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1);
  await expect(page.getByRole('navigation', { name: '产品功能' })).toBeVisible();
  await expect(page.getByRole('button', { name: '工作台' })).toHaveAttribute('aria-current', 'page');
  for (const item of ['知识库', 'Playbook', 'Skill', '晋升']) {
    await expect(page.getByRole('button', { name: item })).toBeDisabled();
  }
  await expect(page.getByLabel('分析上下文')).toBeVisible();
  await expect(page.getByLabel('聊天')).toBeVisible();
  await expect(page.getByLabel('默认时间范围')).toHaveCount(0);
  await expect(page.getByLabel('采样分辨率')).toHaveCount(0);
  await page.getByRole('button', { name: '查看最近 30 分钟 CPU 使用率' }).click();
  await expect(page.getByLabel('分析请求')).toHaveValue('查看最近 30 分钟 CPU 使用率');
  expect(sessionPostCount).toBe(0);
  expect(taskPostCount).toBe(0);
  await page.getByLabel('分析请求').fill('帮我看看最近 5 分钟 node_exporter 的 CPU、内存和系统负载，每隔 5s 一个点');
  await page.getByLabel('分析请求').press('Enter');

  await expect(page).toHaveURL(/sessionId=[^&]+&taskId=[^&]+/);
  expect(sessionPostCount).toBe(1);
  expect(taskPostCount).toBe(1);
  const originalSessionId = new URL(page.url()).searchParams.get('sessionId');
  expect(originalSessionId).toBeTruthy();
  expect(submittedTask?.analysisContext).toEqual({ datasourceUid: 'prometheus-main' });
  await expect(page.getByText(/已查询 node_exporter/)).toBeVisible();
  await expect(page.getByLabel('分析上下文')).toContainText('Prometheus');
  await expect(page.getByLabel('分析上下文')).toContainText('CPU、内存、系统负载');
  await expect(page.getByLabel('分析上下文')).toContainText('已完成');
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
  await expect(page.getByText('分析上下文', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '详情' })).toHaveCount(0);
  await ensureSessionMenuOpen(page);
  await expect(sessionItem(page, '帮我看看最近 5 分钟')).toHaveAttribute('aria-current', 'page');

  await expectThreePaneDesktopLayout(page);
  const widePanels = await panelBoxes(page);
  expect(new Set(widePanels.map((box) => Math.round(box.x))).size).toBeLessThanOrEqual(2);
  expect(widePanels[2].y).toBeGreaterThan(widePanels[0].y);
  expect(widePanels[2].width).toBeGreaterThan(widePanels[0].width * 1.8);
  expect(widePanels[3].y).toBeGreaterThan(widePanels[2].y + widePanels[2].height);

  await page.getByTestId('timeseries-panel').first().locator('summary').click();
  await expect(page.getByTestId('timeseries-panel').first().locator('pre')).toBeVisible();
  await expectPlotsWithinPanels(page);

  await page.setViewportSize({ width: 1800, height: 560 });
  await expectConversationScrollsIndependently(page);

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
  await ensureSessionMenuOpen(page);
  await expect(sessionItem(page, '帮我看看最近 5 分钟')).toBeVisible();
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

  const historyItems = page.locator('.mtb-session-item');
  await expect(historyItems.nth(0)).toContainText('新对话只看内存');
  await expect(historyItems.nth(1)).toContainText('帮我看看最近 5 分钟');

  await sessionItem(page, '帮我看看最近 5 分钟').click();
  await expect(page).toHaveURL((url) => url.searchParams.get('sessionId') === originalSessionId && !url.searchParams.has('taskId'));
  await expect(sessionItem(page, '帮我看看最近 5 分钟')).toHaveAttribute('aria-current', 'page');
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(4);
  await expect(page.getByTestId('chart-group')).toHaveCount(2);
  await expect(page.getByText('你：只看 CPU')).toBeVisible();
  await expect(page.getByText('你：新对话只看内存')).toHaveCount(0);

  await page.getByLabel('分析请求').fill('回到旧会话只看负载');
  await page.getByRole('button', { name: '开始分析' }).click();
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(5);
  await expect(page.getByTestId('chart-group')).toHaveCount(3);
  await expect(page.getByText('你：回到旧会话只看负载')).toBeVisible();
  await expect(historyItems.nth(0)).toContainText('帮我看看最近 5 分钟');
  await expect(historyItems.nth(1)).toContainText('新对话只看内存');

  await sessionItem(page, '新对话只看内存').click();
  await expect(page).toHaveURL((url) => url.searchParams.get('sessionId') === freshSessionId && !url.searchParams.has('taskId'));
  await expect(sessionItem(page, '新对话只看内存')).toHaveAttribute('aria-current', 'page');
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(1);
  await expect(page.getByTestId('chart-group')).toHaveCount(1);
  await expect(page.getByText('你：新对话只看内存')).toBeVisible();
  await expect(page.getByText('你：回到旧会话只看负载')).toHaveCount(0);

  await page.reload();
  await expect(page.getByTestId('timeseries-panel')).toHaveCount(1);
  await expect(page.getByText('你：新对话只看内存')).toBeVisible();
  await ensureSessionMenuOpen(page);
  await expect(sessionItem(page, '新对话只看内存')).toHaveAttribute('aria-current', 'page');

  await page.goto('/a/mini-torchbearing-app/workbench?theme=dark&sessionId=missing-session&taskId=missing-task');
  await expect(page).toHaveURL((url) => url.searchParams.get('theme') === 'dark' && !url.searchParams.has('sessionId') && !url.searchParams.has('taskId'));
  await expect(page.getByRole('status').filter({ hasText: '已清除当前运行环境中不存在的旧会话' })).toHaveText('已清除当前运行环境中不存在的旧会话，请重新提交分析。');
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
  const [chat, context, canvas] = await Promise.all([
    page.getByTestId('chat-pane').boundingBox(),
    page.getByTestId('context-pane').boundingBox(),
    page.getByTestId('chart-canvas').boundingBox(),
  ]);
  expect(chat).not.toBeNull();
  expect(context).not.toBeNull();
  expect(canvas).not.toBeNull();
  expect(canvas!.x).toBeLessThan(context!.x);
  expect(context!.x).toBeLessThan(chat!.x);
  expect(chat!.y).toBeCloseTo(context!.y, 0);
  expect(context!.y).toBeCloseTo(canvas!.y, 0);
  expect(context!.width).toBeGreaterThanOrEqual(240);
  expect(context!.width).toBeLessThanOrEqual(280);
  expect(chat!.width).toBeGreaterThanOrEqual(340);
  expect(chat!.width).toBeLessThanOrEqual(380);
}

async function expectThreePaneNarrowLayout(page: import('@playwright/test').Page) {
  const [chat, context, canvas] = await Promise.all([
    page.getByTestId('chat-pane').boundingBox(),
    page.getByTestId('context-pane').boundingBox(),
    page.getByTestId('chart-canvas').boundingBox(),
  ]);
  expect(chat).not.toBeNull();
  expect(context).not.toBeNull();
  expect(canvas).not.toBeNull();
  expect(chat!.y).toBeLessThan(context!.y);
  expect(context!.y).toBeLessThan(canvas!.y);
}

function sessionItem(page: import('@playwright/test').Page, title: string) {
  return page.locator('.mtb-session-item').filter({ hasText: title });
}

async function ensureSessionMenuOpen(page: import('@playwright/test').Page) {
  const toggle = page.getByTestId('session-menu-toggle');
  if (await toggle.getAttribute('aria-expanded') !== 'true') await toggle.click();
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
}

async function expectNoHorizontalOverflow(page: import('@playwright/test').Page) {
  const viewport = await page.evaluate(() => ({ clientWidth: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth }));
  expect(viewport.scrollWidth).toBeLessThanOrEqual(viewport.clientWidth);
}

async function expectConversationScrollsIndependently(page: import('@playwright/test').Page) {
  const pane = page.getByTestId('chat-pane');
  const scroll = page.getByTestId('conversation-scroll-container');
  const input = page.getByLabel('分析请求');
  await expect(pane).toBeVisible();
  await expect(input).toBeVisible();
  await expect.poll(() => scroll.evaluate((node) => node.scrollHeight > node.clientHeight)).toBe(true);
  await scroll.evaluate((node) => { node.scrollTop = node.scrollHeight; });
  await expect.poll(() => scroll.evaluate((node) => node.scrollTop)).toBeGreaterThan(0);
  const [paneBox, inputBox] = await Promise.all([pane.boundingBox(), input.boundingBox()]);
  expect(paneBox).not.toBeNull();
  expect(inputBox).not.toBeNull();
  expect(inputBox!.y + inputBox!.height).toBeLessThanOrEqual(paneBox!.y + paneBox!.height + 1);
}
