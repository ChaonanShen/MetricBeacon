import assert from 'node:assert/strict';

import { analyzeTaskEvents, formatTaskSummary } from '../../diagnostics/task-result-sanity.mjs';

const base = process.env.GRAFANA_URL ?? 'http://127.0.0.1:3000';
const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const authorization = `Basic ${Buffer.from(`${user}:${password}`).toString('base64')}`;
const resourceBase = `${base}/api/plugins/mini-torchbearing-app/resources`;
const realMetrics = process.env.REAL_METRICS === '1';
const cases = [
  { message: '查看最近30秒 CPU 使用率', views: ['cpu'], rangeSeconds: 30, step: 5, window: 30 },
  { message: '查看最近1分钟cpu的使用率变化，每隔5s采集个数据', views: ['cpu'], rangeSeconds: 60, step: 5, window: 30 },
  { message: '最近30分钟 CPU，每隔30s采集一个点', views: ['cpu'], rangeSeconds: 1800, step: 30, window: 60 },
  { message: '查看最近30min里cpu和负载变化图，间隔5分钟采集一个数据点', views: ['cpu', 'load'], rangeSeconds: 1800, step: 300, window: 60 },
  { message: '查看最近五分钟内 CPU 变化数据，每个5s画一个点', views: ['cpu'], rangeSeconds: 300, step: 5, window: 30 },
  { message: '画出三种 node_exporter 监测数据的变化图吧', views: ['cpu', 'memory', 'load'], rangeSeconds: 1800, step: 10, window: 60 },
];

function headers(extra = {}) {
  return { authorization, ...extra };
}

async function requestJSON(url, init = {}) {
  const response = await fetch(url, { ...init, headers: headers(init.headers) });
  const body = await response.text();
  return { response, body: body ? JSON.parse(body) : undefined };
}

function parseSSE(raw) {
  const blocks = raw.split(/\r?\n\r?\n/);
  if (!/\r?\n\r?\n$/.test(raw)) {
    blocks.pop();
  }
  return blocks.flatMap((block) => {
    const data = block.split(/\r?\n/).filter((line) => line.startsWith('data:')).map((line) => line.slice(5).trim()).join('\n');
    return data ? [JSON.parse(data)] : [];
  });
}

async function streamEvents(url) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10_000);
  try {
    const response = await fetch(url, { headers: headers(), signal: controller.signal });
    if (response.status !== 200) {
      assert.fail(`SSE request failed: ${await response.text()}`);
    }
    assert.ok(response.body, 'SSE response did not include a body');
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let raw = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      raw += decoder.decode(value, { stream: true });
      const events = parseSSE(raw);
      if (events.some((event) => event.type === 'task.completed' || event.type === 'task.failed')) {
        await reader.cancel();
        return events;
      }
    }
    return parseSSE(raw);
  } finally {
    clearTimeout(timeout);
  }
}

const settings = await requestJSON(`${base}/api/plugins/mini-torchbearing-app/settings`);
assert.equal(settings.response.status, 200);

const session = await requestJSON(`${resourceBase}/sessions`, {
  method: 'POST',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ title: 'Mock E2E' }),
});
assert.ok(session.response.ok, `Create Session failed: ${JSON.stringify(session.body)}`);

const createTaskBody = {
  sessionId: session.body.id,
  message: cases[0].message,
  analysisContext: { datasourceUid: 'prometheus-main' },
};
const idempotencyKey = `mock-e2e-task-${crypto.randomUUID()}`;
const task = await requestJSON(`${resourceBase}/tasks`, {
  method: 'POST',
  headers: { 'content-type': 'application/json', 'idempotency-key': idempotencyKey },
  body: JSON.stringify(createTaskBody),
});
assert.equal(task.response.status, 202, `Create Task failed: ${JSON.stringify(task.body)}`);

const repeatedTask = await requestJSON(`${resourceBase}/tasks`, {
  method: 'POST',
  headers: { 'content-type': 'application/json', 'idempotency-key': idempotencyKey },
  body: JSON.stringify(createTaskBody),
});
assert.equal(repeatedTask.response.status, 202);
assert.equal(repeatedTask.body.id, task.body.id, 'same wire intent and key must return the original Task');

const conflict = await requestJSON(`${resourceBase}/tasks`, {
  method: 'POST',
  headers: { 'content-type': 'application/json', 'idempotency-key': idempotencyKey },
  body: JSON.stringify({ ...createTaskBody, message: 'different node exporter request' }),
});
assert.equal(conflict.response.status, 409);
assert.equal(conflict.body.error?.code, 'idempotency_conflict');

const events = await streamEvents(`${resourceBase}/tasks/${encodeURIComponent(task.body.id)}/events?afterSequence=0`);
assert.ok(events.length > 0, 'Task SSE did not return any durable events');
const taskSummary = assertBoundedResult(task.body, events, cases[0]);
for (const line of formatTaskSummary('task', taskSummary)) console.log(line);

const replayAfter = events.at(-6).sequence;
const replay = await streamEvents(`${resourceBase}/tasks/${encodeURIComponent(task.body.id)}/events?afterSequence=${replayAfter}`);
assert.deepEqual(replay, events.filter((event) => event.sequence > replayAfter), 'SSE replay must return exactly the durable suffix');

for (const testCase of cases.slice(1)) {
  const nextTask = await requestJSON(`${resourceBase}/tasks`, {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'idempotency-key': `mock-e2e-task-${crypto.randomUUID()}` },
    body: JSON.stringify({ ...createTaskBody, message: testCase.message }),
  });
  assert.equal(nextTask.response.status, 202, `Create follow-up Task failed: ${JSON.stringify(nextTask.body)}`);
  const nextEvents = await streamEvents(`${resourceBase}/tasks/${encodeURIComponent(nextTask.body.id)}/events?afterSequence=0`);
  const summary = assertBoundedResult(nextTask.body, nextEvents, testCase);
  for (const line of formatTaskSummary('task', summary)) console.log(line);
}

const messagePage = await requestJSON(`${resourceBase}/sessions/${encodeURIComponent(session.body.id)}/messages?pageSize=50`);
assert.equal(messagePage.response.status, 200);
assert.equal(messagePage.body.items.length, cases.length * 2, 'each completed Task must persist one user and one assistant Message');
assert.equal(messagePage.body.nextPageToken, null);
const taskPage = await requestJSON(`${resourceBase}/sessions/${encodeURIComponent(session.body.id)}/tasks?pageSize=20`);
assert.equal(taskPage.response.status, 200);
assert.equal(taskPage.body.items.length, cases.length, 'all completed Tasks must remain in Session history');
assert.equal(taskPage.body.nextPageToken, null);
const sessionPage = await requestJSON(`${resourceBase}/sessions?pageSize=20`);
assert.equal(sessionPage.response.status, 200);
const listedSession = sessionPage.body.items.find((item) => item.id === session.body.id);
assert.ok(listedSession, 'completed API Session must appear in owner history');
assert.equal(listedSession.title, 'Mock E2E');
assert.ok(listedSession.version >= cases.length + 1, `Session version did not track accepted Tasks: ${listedSession.version}`);
assert.ok(Date.parse(listedSession.updatedAt) >= Date.parse(listedSession.createdAt), 'Session activity moved backwards');
const finiteReplay = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(task.body.id)}/events/replay?afterSequence=0&pageSize=200`);
assert.equal(finiteReplay.response.status, 200);
assert.equal(finiteReplay.body.targetSequence, events.at(-1).sequence);
assert.deepEqual(finiteReplay.body.items.map((event) => event.sequence), events.map((event) => event.sequence));
assert.equal(finiteReplay.body.nextPageToken, null, 'terminal Task finite replay must not keep a follow cursor');

function assertBoundedResult(taskValue, taskEvents, expected) {
  assert.equal(taskEvents.at(-1)?.type, 'task.completed', `Task did not complete: ${JSON.stringify(taskEvents.at(-1))}`);
  assert.equal(Math.round((Date.parse(taskValue.timeRange.to) - Date.parse(taskValue.timeRange.from)) / 1000), expected.rangeSeconds);
  assert.deepEqual(taskValue.queryPlan, { views: expected.views, stepSeconds: expected.step, cpuRateWindowSeconds: expected.window });
  const summary = analyzeTaskEvents(taskEvents, { expectedViews: expected.views, expectedToolCalls: expected.views.length });
  const chartEvents = taskEvents.filter((event) => event.type === 'chart.created');
  const executionEvents = new Map(taskEvents.filter((event) => event.type === 'chart.execution_completed').map((event) => [event.payload.chartId, event.payload.execution]));
  assert.equal(chartEvents.length, expected.views.length);
  for (const chartEvent of chartEvents) {
    const chart = chartEvent.payload.chart;
    const query = chart.queries[0];
    assert.equal(query.stepSeconds, expected.step);
    assert.equal(Date.parse(query.timeRange.from), Date.parse(taskValue.timeRange.from));
    assert.equal(Date.parse(query.timeRange.to), Date.parse(taskValue.timeRange.to));
    if (chart.title === 'CPU 使用率') {
      const suffix = expected.window === 30 ? '[30s]' : expected.window === 60 ? '[1m]' : '[5m]';
      assert.ok(query.expression.includes(suffix), `CPU query did not use ${suffix}: ${query.expression}`);
    }
    const execution = executionEvents.get(chart.id);
    assert.ok(execution, `chart ${chart.id} had no execution`);
    assert.equal(execution.status, 'success');
    assert.ok(execution.actualSampleRange, 'successful query did not persist its actual sample range');
    for (const series of execution.series) assertSeries(series, taskValue.timeRange, expected);
    const timestamps = execution.series.flatMap((series) => series.points.map((point) => Date.parse(point.timestamp)));
    assert.equal(Date.parse(execution.actualSampleRange.from), Math.min(...timestamps));
    assert.equal(Date.parse(execution.actualSampleRange.to), Math.max(...timestamps));
  }
  const answer = taskEvents.find((event) => event.type === 'assistant.message.completed')?.payload.message.content;
  assert.equal(typeof answer, 'string');
  assert.ok(answer.includes(`step=${expected.step}s`), `answer omitted effective step: ${answer}`);
  assert.ok(answer.includes(`CPU rate window=${expected.window}s`), `answer omitted effective CPU window: ${answer}`);
  assert.ok(answer.includes('实际数据'), `answer omitted actual data range: ${answer}`);
  return summary;
}

function assertSeries(series, requestedRange, expected) {
  assert.ok(series.points.length > 0, 'query returned an empty series');
  const timestamps = series.points.map((point) => Date.parse(point.timestamp));
  assert.ok(timestamps.every(Number.isFinite));
  assert.ok(series.points.every((point) => Number.isFinite(point.value)));
  const boundaryToleranceMs = realMetrics ? 1000 : 0;
  assert.ok(timestamps[0] >= Date.parse(requestedRange.from) - boundaryToleranceMs && timestamps.at(-1) <= Date.parse(requestedRange.to) + boundaryToleranceMs);
  for (let index = 1; index < timestamps.length; index += 1) {
    assert.equal((timestamps[index] - timestamps[index - 1]) / 1000, expected.step, 'sample interval did not match effective step');
  }
  if (!realMetrics) {
    assert.equal(series.points.length, Math.floor(expected.rangeSeconds / expected.step) + 1);
    assert.equal(timestamps[0], Date.parse(requestedRange.from));
    assert.equal(timestamps.at(-1), Date.parse(requestedRange.to));
  }
}
