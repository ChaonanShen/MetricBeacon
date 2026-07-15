import assert from 'node:assert/strict';

import { analyzeTaskEvents, canonicalExpressions, formatTaskSummary } from '../../diagnostics/task-result-sanity.mjs';

const base = process.env.GRAFANA_URL ?? 'http://127.0.0.1:3000';
const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const authorization = `Basic ${Buffer.from(`${user}:${password}`).toString('base64')}`;
const resourceBase = `${base}/api/plugins/mini-torchbearing-app/resources`;
const expressions = {
  ...canonicalExpressions,
  cpu: '100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[1m])))',
};

function headers(extra = {}) {
  return { authorization, ...extra };
}

async function requestJSON(url, init = {}) {
  const response = await fetch(url, { ...init, headers: headers(init.headers) });
  const text = await response.text();
  return { response, body: text ? JSON.parse(text) : undefined };
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

async function terminalEvents(taskID) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 75_000);
  try {
    const response = await fetch(`${resourceBase}/tasks/${encodeURIComponent(taskID)}/events?afterSequence=0`, { headers: headers(), signal: controller.signal });
    if (response.status !== 200) {
      assert.fail(`SSE failed: ${await response.text()}`);
    }
    assert.ok(response.body, 'SSE response did not include a body');
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let raw = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
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

async function submit(sessionID, message, expectedViews, expectedPlan = { rangeSeconds: 1800, step: 10, window: 60 }) {
  const response = await requestJSON(`${resourceBase}/tasks`, {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'idempotency-key': `real-agent-${crypto.randomUUID()}` },
    body: JSON.stringify({ sessionId: sessionID, message, analysisContext: { datasourceUid: 'prometheus-main' } }),
  });
  assert.equal(response.response.status, 202, `Create Task failed: ${JSON.stringify(response.body)}`);
  const events = await terminalEvents(response.body.id);
  assert.ok(events.length > 0, 'agent task emitted no events');
  if (events.at(-1)?.type !== 'task.completed') {
    console.error('[agent-task] safeEventTrace=' + JSON.stringify(events.map((event) => ({
      sequence: event.sequence,
      type: event.type,
      toolName: event.payload?.toolName,
      chartKey: event.payload?.chartKey,
      success: event.payload?.outputSummary?.success,
      errorCode: event.payload?.error?.code,
    }))));
  }
  assert.equal(events.at(-1)?.type, 'task.completed', `agent task did not complete: ${JSON.stringify(events.at(-1))}`);
  assert.deepEqual(response.body.queryPlan, { views: expectedViews, stepSeconds: expectedPlan.step, cpuRateWindowSeconds: expectedPlan.window });
  assert.equal(Math.round((Date.parse(response.body.timeRange.to) - Date.parse(response.body.timeRange.from)) / 1000), expectedPlan.rangeSeconds);
  const summary = analyzeTaskEvents(events, { expectedViews, expectedToolCalls: expectedViews.length });
  for (const line of formatTaskSummary('agent-task', summary)) console.log(line);
  assertNoSensitiveMarkers(events);
  const answer = events.find((event) => event.type === 'assistant.message.completed')?.payload.message.content;
  assert.ok(answer.includes(`step=${expectedPlan.step}s`) && answer.includes(`CPU rate window=${expectedPlan.window}s`), `local answer omitted effective parameters: ${answer}`);
  return { task: response.body, events, summary };
}

function assertNoSensitiveMarkers(value) {
  const serialized = JSON.stringify(value);
  for (const marker of ['http://prometheus:9090', 'reasoning-marker', 'raw-series-marker', 'private reasoning']) {
    assert.ok(!serialized.includes(marker), `external event leaked prohibited marker ${marker}`);
  }
}

function chartExpressions(events) {
  return events.filter((event) => event.type === 'chart.created').map((event) => event.payload.chart.queries[0].expression);
}

const session = await requestJSON(`${resourceBase}/sessions`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ title: 'Real Agent smoke' }) });
assert.equal(session.response.status, 201, `Create Session failed: ${JSON.stringify(session.body)}`);

const overview = await submit(session.body.id, '请概览 node_exporter 的 CPU、内存和系统负载。', ['cpu', 'memory', 'load']);
assert.deepEqual(chartExpressions(overview.events).sort(), [expressions.cpu, expressions.memory, expressions.load].sort());

const cpu = await submit(session.body.id, '只看 CPU。', ['cpu']);
assert.deepEqual(chartExpressions(cpu.events), [expressions.cpu]);

const messages = await requestJSON(`${resourceBase}/sessions/${encodeURIComponent(session.body.id)}/messages?pageSize=50`);
const tasks = await requestJSON(`${resourceBase}/sessions/${encodeURIComponent(session.body.id)}/tasks?pageSize=20`);
assert.equal(messages.response.status, 200);
assert.equal(tasks.response.status, 200);
assert.equal(messages.body.items.length, 4, 'refresh history must include two user and two assistant messages');
assert.equal(tasks.body.items.length, 2, 'refresh history must include both tasks');
assertNoSensitiveMarkers({ messages: messages.body, tasks: tasks.body });

const replay = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(cpu.task.id)}/events/replay?afterSequence=0&pageSize=200`);
assert.equal(replay.response.status, 200);
assert.equal(replay.body.targetSequence, cpu.events.at(-1).sequence, 'terminal replay must end at the terminal durable sequence');
assert.deepEqual(replay.body.items.map((event) => event.sequence), cpu.events.map((event) => event.sequence));
assert.equal(replay.body.nextPageToken, null, 'terminal replay must not leave a follow cursor');

const repeatedSession = await requestJSON(`${resourceBase}/sessions`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ title: 'Repeated planner history' }) });
assert.equal(repeatedSession.response.status, 201, `Create repeated session failed: ${JSON.stringify(repeatedSession.body)}`);
const repeatedMessage = '查看最近10min的cpu和内存变化图，每隔2min采集一个数据点';
for (let index = 0; index < 8; index += 1) {
  await submit(repeatedSession.body.id, repeatedMessage, ['cpu', 'memory'], { rangeSeconds: 600, step: 120, window: 30 });
}
const repeatedMessages = await requestJSON(`${resourceBase}/sessions/${encodeURIComponent(repeatedSession.body.id)}/messages?pageSize=50`);
const repeatedTasks = await requestJSON(`${resourceBase}/sessions/${encodeURIComponent(repeatedSession.body.id)}/tasks?pageSize=20`);
assert.equal(repeatedMessages.response.status, 200);
assert.equal(repeatedTasks.response.status, 200);
assert.equal(repeatedMessages.body.items.length, 16, 'repeated session must persist eight user/assistant pairs');
assert.equal(repeatedTasks.body.items.length, 8, 'repeated session must persist eight tasks');
assertNoSensitiveMarkers({ messages: repeatedMessages.body, tasks: repeatedTasks.body });
