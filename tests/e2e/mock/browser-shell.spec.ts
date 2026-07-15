const { expect, test } = require('../../../apps/grafana-plugin/frontend/node_modules/@playwright/test') as typeof import('@playwright/test');

const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';

test('renders the production shell without prototype side effects across themes and widths', async ({ page }) => {
  await page.setViewportSize({ width: 1800, height: 900 });
  const requests: Array<{ method: string; url: string }> = [];
  page.on('request', (request) => requests.push({ method: request.method(), url: request.url() }));
  const login = await page.context().request.post('/login', { data: { user, password } });
  expect(login.ok()).toBeTruthy();

  await page.goto('/a/mini-torchbearing-app/workbench?theme=dark');
  await expect(page.getByRole('heading', { name: '指标分析工作台', level: 1 })).toBeVisible();
  await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1);
  await expect(page.getByLabel('Canvas 图表画布')).toContainText('空 Canvas');
  await expect(page.getByLabel('分析上下文')).toContainText('提交后确定');
  await expect(page.getByLabel('聊天')).toContainText('开始一次指标分析');
  for (const item of ['知识库', 'Playbook', 'Skill', '晋升']) await expect(page.getByRole('button', { name: item })).toBeDisabled();

  await page.getByRole('button', { name: '会话', exact: true }).click();
  await expect(page.getByTestId('session-menu-toggle')).toBeFocused();
  await expect(page.getByTestId('session-menu-toggle')).toHaveAttribute('aria-expanded', 'true');

  await page.getByRole('button', { name: '分析最近 30 分钟内存可用率' }).click();
  await expect(page.getByLabel('分析请求')).toHaveValue('分析最近 30 分钟内存可用率');
  expect(requests.filter((request) => request.method === 'POST' && request.url.includes('/resources/'))).toHaveLength(0);

  const darkTokens = await themeTokens(page);
  await page.goto('/a/mini-torchbearing-app/workbench?theme=light');
  const lightTokens = await themeTokens(page);
  expect(lightTokens.background).not.toBe(darkTokens.background);
  expect(lightTokens.accent).not.toBe(darkTokens.accent);

  for (const viewport of [{ width: 1800, height: 900 }, { width: 1366, height: 768 }, { width: 900, height: 900 }]) {
    await page.setViewportSize(viewport);
    await expect(page.getByTestId('workbench-root')).toBeVisible();
    await expectNoHorizontalOverflow(page);
  }
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

  const gawKeys = await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('gaw_')));
  expect(gawKeys).toEqual([]);
  const external = requests.filter(({ url }) => url.startsWith('http') && new URL(url).origin !== new URL(page.url()).origin);
  expect(external).toEqual([]);
  const resourceRequests = requests.filter(({ url }) => url.includes('/resources/'));
  expect(resourceRequests.every(({ url }) => new URL(url).pathname.startsWith('/api/plugins/mini-torchbearing-app/resources/'))).toBe(true);
});

async function themeTokens(page: import('@playwright/test').Page) {
  return page.getByTestId('workbench-root').evaluate((node) => {
    const styles = getComputedStyle(node);
    return { background: styles.getPropertyValue('--mtb-bg').trim(), accent: styles.getPropertyValue('--mtb-accent').trim() };
  });
}

async function expectNoHorizontalOverflow(page: import('@playwright/test').Page) {
  const dimensions = await page.evaluate(() => ({ clientWidth: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth }));
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth);
}
