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
    assert.equal(response.status, 200, `SSE request failed: ${await response.text()}`);
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
  analysisContext: { datasourceUid: 'mock-prometheus', timeRange: { relativeDuration: '30m' } },
};
const idempotencyKey = 'mock-e2e-task';
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
assert.equal(conflict.body.code, 'idempotency_conflict');

const events = await streamEvents(`${resourceBase}/tasks/${encodeURIComponent(task.body.id)}/events?afterSequence=0`);
assert.ok(events.length > 0, 'Task SSE did not return any durable events');
assert.deepEqual(events.map((event) => event.sequence), Array.from({ length: events.length }, (_, index) => index + 1), 'SSE sequences must be continuous from one');
assert.equal(events.filter((event) => event.type === 'tool.started').length, 7);
assert.equal(events.filter((event) => event.type === 'tool.completed').length, 7);
assert.equal(events.filter((event) => event.type === 'chart.created').length, 3);
assert.equal(events.filter((event) => event.type === 'chart.execution_completed').length, 3);
assert.equal(events.at(-1)?.type, 'task.completed');
assert.ok(events.some((event) => event.type === 'assistant.message.completed' && event.payload.message.content.includes('CPU、内存和系统负载')));

const replayAfter = events.at(-6).sequence;
const replay = await streamEvents(`${resourceBase}/tasks/${encodeURIComponent(task.body.id)}/events?afterSequence=${replayAfter}`);
assert.deepEqual(replay, events.filter((event) => event.sequence > replayAfter), 'SSE replay must return exactly the durable suffix');
