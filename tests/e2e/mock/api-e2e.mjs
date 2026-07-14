import assert from 'node:assert/strict';

const base = process.env.GRAFANA_URL ?? 'http://127.0.0.1:3000';
const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const authorization = `Basic ${Buffer.from(`${user}:${password}`).toString('base64')}`;
const resourceBase = `${base}/api/plugins/mini-torchbearing-app/resources`;

function headers(extra = {}) {
  return { authorization, ...extra };
}

async function requestJSON(url, init = {}) {
  const response = await fetch(url, { ...init, headers: headers(init.headers) });
  const body = await response.text();
  return { response, body: body ? JSON.parse(body) : undefined };
}

function parseSSE(raw) {
  return raw.split(/\r?\n\r?\n/).flatMap((block) => {
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
      if (raw.includes('"type":"task.completed"') || raw.includes('"type":"task.failed"')) {
        await reader.cancel();
        break;
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
  message: 'show node exporter',
  analysisContext: { datasourceUid: 'prometheus-main', timeRange: { relativeDuration: '30m' } },
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
assert.deepEqual(events.map((event) => event.sequence), Array.from({ length: events.length }, (_, index) => index + 1), 'SSE sequences must be continuous from one');
assert.equal(events.filter((event) => event.type === 'tool.started').length, 7);
assert.equal(events.filter((event) => event.type === 'tool.completed').length, 7);
assert.equal(events.filter((event) => event.type === 'chart.created').length, 3);
assert.equal(events.filter((event) => event.type === 'chart.execution_completed').length, 3);
assert.equal(events.at(-1)?.type, 'task.completed');
assert.ok(events.some((event) => event.type === 'assistant.message.completed' && event.payload.message.content.includes('CPU、内存和系统负载')));
if (process.env.REAL_METRICS === '1') {
  const executions = events.filter((event) => event.type === 'chart.execution_completed').map((event) => event.payload.execution);
  assert.equal(executions.length, 3);
  for (const execution of executions) {
    assert.ok(execution.seriesCount > 0, 'real Prometheus query returned no series');
    assert.ok(execution.series.every((series) => series.points.length > 0), 'real Prometheus query returned an empty series');
  }
}

const replayAfter = events.at(-6).sequence;
const replay = await streamEvents(`${resourceBase}/tasks/${encodeURIComponent(task.body.id)}/events?afterSequence=${replayAfter}`);
assert.deepEqual(replay, events.filter((event) => event.sequence > replayAfter), 'SSE replay must return exactly the durable suffix');

for (const message of ['show only CPU', 'show memory again']) {
  const nextTask = await requestJSON(`${resourceBase}/tasks`, {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'idempotency-key': `mock-e2e-task-${crypto.randomUUID()}` },
    body: JSON.stringify({ ...createTaskBody, message }),
  });
  assert.equal(nextTask.response.status, 202, `Create follow-up Task failed: ${JSON.stringify(nextTask.body)}`);
  const nextEvents = await streamEvents(`${resourceBase}/tasks/${encodeURIComponent(nextTask.body.id)}/events?afterSequence=0`);
  assert.equal(nextEvents.at(-1)?.type, 'task.completed', 'follow-up Task stream must terminate');
}

const messagePage = await requestJSON(`${resourceBase}/sessions/${encodeURIComponent(session.body.id)}/messages?pageSize=50`);
assert.equal(messagePage.response.status, 200);
assert.equal(messagePage.body.items.length, 6, 'three completed Tasks must persist three user and three assistant Messages');
assert.equal(messagePage.body.nextPageToken, null);
const taskPage = await requestJSON(`${resourceBase}/sessions/${encodeURIComponent(session.body.id)}/tasks?pageSize=20`);
assert.equal(taskPage.response.status, 200);
assert.equal(taskPage.body.items.length, 3, 'three completed Tasks must remain in Session history');
assert.equal(taskPage.body.nextPageToken, null);
const finiteReplay = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(task.body.id)}/events/replay?afterSequence=0&pageSize=200`);
assert.equal(finiteReplay.response.status, 200);
assert.equal(finiteReplay.body.targetSequence, events.at(-1).sequence);
assert.deepEqual(finiteReplay.body.items.map((event) => event.sequence), events.map((event) => event.sequence));
assert.equal(finiteReplay.body.nextPageToken, null, 'terminal Task finite replay must not keep a follow cursor');
