import assert from 'node:assert/strict';

const base = process.env.GRAFANA_URL;
assert.ok(base, 'GRAFANA_URL is required');
const user = process.env.GRAFANA_ADMIN_USER ?? 'admin';
const password = process.env.GRAFANA_ADMIN_PASSWORD ?? 'admin';
const authorization = `Basic ${Buffer.from(`${user}:${password}`).toString('base64')}`;
const resourceBase = `${base}/api/plugins/mini-torchbearing-app/resources`;

function headers(extra = {}) {
  return { authorization, ...extra };
}

async function requestJSON(url, init = {}) {
  const response = await fetch(url, { ...init, headers: headers(init.headers) });
  const raw = await response.text();
  return { response, body: raw ? JSON.parse(raw) : undefined };
}

async function waitFor(description, read, accept, timeoutMs = 120_000) {
  const deadline = Date.now() + timeoutMs;
  let latest;
  while (Date.now() < deadline) {
    latest = await read();
    if (accept(latest)) return latest;
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  assert.fail(`${description} timed out; latest=${JSON.stringify(latest)}`);
}

function parseSSE(raw) {
  const blocks = raw.split(/\r?\n\r?\n/);
  if (!/\r?\n\r?\n$/.test(raw)) blocks.pop();
  return blocks.flatMap((block) => {
    const data = block.split(/\r?\n/).filter((line) => line.startsWith('data:')).map((line) => line.slice(5).trim()).join('\n');
    return data ? [JSON.parse(data)] : [];
  });
}

async function streamToTerminal(taskId) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(new Error('Incident SSE timed out')), 120_000);
  try {
    const response = await fetch(`${resourceBase}/tasks/${encodeURIComponent(taskId)}/events?afterSequence=0`, { headers: headers(), signal: controller.signal });
    if (response.status !== 200) assert.fail(`Incident SSE failed: ${await response.text()}`);
    assert.ok(response.body);
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let raw = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) return parseSSE(raw);
      raw += decoder.decode(value, { stream: true });
      const events = parseSSE(raw);
      if (events.some((event) => event.type === 'task.completed' || event.type === 'task.failed')) {
        await reader.cancel();
        return events;
      }
    }
  } finally {
    clearTimeout(timeout);
  }
}

const settings = await requestJSON(`${base}/api/plugins/mini-torchbearing-app/settings`);
assert.equal(settings.response.status, 200);

const incident = await waitFor('Incident waiting for approval', async () => {
  const page = await requestJSON(`${resourceBase}/incidents?pageSize=20`);
  assert.equal(page.response.status, 200, JSON.stringify(page.body));
  return page.body.items.find((item) => item.incidentPlan?.alertName === 'OrderQueueBacklog');
}, (item) => item?.status === 'waiting_approval' && item.incidentPlan?.intent);

assert.equal(incident.kind, 'incident_remediation');
assert.equal(incident.incidentPlan.serviceRef, 'order-demo');
assert.equal(incident.incidentPlan.diagnosis.primaryHypothesis, 'worker_stopped');
assert.deepEqual(incident.incidentPlan.diagnosis.alternativeHypotheses, ['slow_processing', 'dependency_errors']);
assert.equal(incident.incidentPlan.intent.capabilityId, 'order_service.restore_worker_concurrency');
assert.equal(incident.incidentPlan.intent.beforeConcurrency, 0);
assert.equal(incident.incidentPlan.intent.afterConcurrency, 2);

const approval = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(incident.id)}/approval`);
assert.equal(approval.response.status, 200, JSON.stringify(approval.body));
assert.equal(approval.body.status, 'pending');
assert.equal(approval.body.intentDigest, incident.incidentPlan.intent.digest);

const decisionBody = {
  decision: 'approve',
  reason: 'Incident golden E2E approval',
  expectedTaskVersion: incident.version,
  expectedApprovalVersion: approval.body.version,
  intentDigest: approval.body.intentDigest,
};
const stale = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(incident.id)}/approval`, {
  method: 'POST',
  headers: { 'content-type': 'application/json', 'idempotency-key': `incident-stale-${crypto.randomUUID()}` },
  body: JSON.stringify({ ...decisionBody, expectedTaskVersion: incident.version + 1 }),
});
assert.equal(stale.response.status, 409, JSON.stringify(stale.body));
assert.equal(stale.body.error?.code, 'resource_conflict');

const idempotencyKey = `incident-approve-${crypto.randomUUID()}`;
const approved = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(incident.id)}/approval`, {
  method: 'POST',
  headers: { 'content-type': 'application/json', 'idempotency-key': idempotencyKey },
  body: JSON.stringify(decisionBody),
});
assert.equal(approved.response.status, 202, JSON.stringify(approved.body));
assert.equal(approved.body.status, 'approved');
assert.equal(approved.body.version, approval.body.version + 1);

const retry = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(incident.id)}/approval`, {
  method: 'POST',
  headers: { 'content-type': 'application/json', 'idempotency-key': idempotencyKey },
  body: JSON.stringify(decisionBody),
});
assert.equal(retry.response.status, 202, JSON.stringify(retry.body));
assert.deepEqual(retry.body, approved.body, 'approval retry must return the original decision');

const events = await streamToTerminal(incident.id);
assert.ok(events.length > 0);
assert.deepEqual(events.map((event) => event.sequence), Array.from({ length: events.length }, (_, index) => index + 1), 'Incident events must be contiguous');
assert.equal(events.at(-1)?.type, 'task.completed', JSON.stringify(events.at(-1)));
const eventTypes = events.map((event) => event.type);
for (const required of ['alert.received', 'playbook.resolved', 'assets.pinned', 'diagnosis.completed', 'intent.prepared', 'approval.requested', 'approval.decided', 'remediation.started', 'remediation.reconciled', 'verification.runtime', 'verification.metrics', 'verification.business', 'task.completed']) {
  assert.ok(eventTypes.includes(required), `missing durable event ${required}`);
}
for (const unique of ['remediation.started', 'remediation.reconciled', 'verification.runtime', 'verification.metrics', 'verification.business']) {
  assert.equal(eventTypes.filter((type) => type === unique).length, 1, `${unique} must occur exactly once`);
}
assert.ok(eventTypes.indexOf('approval.decided') < eventTypes.indexOf('remediation.started'));
assert.ok(eventTypes.indexOf('verification.runtime') < eventTypes.indexOf('verification.metrics'));
assert.ok(eventTypes.indexOf('verification.metrics') < eventTypes.indexOf('verification.business'));

const completed = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(incident.id)}`);
assert.equal(completed.response.status, 200);
assert.equal(completed.body.status, 'completed');
assert.equal(completed.body.incidentPlan.phase, 'completed');
assert.equal(completed.body.incidentPlan.intent.digest, approval.body.intentDigest);

const replay = await requestJSON(`${resourceBase}/tasks/${encodeURIComponent(incident.id)}/events/replay?afterSequence=0&pageSize=200`);
assert.equal(replay.response.status, 200);
assert.equal(replay.body.targetSequence, events.at(-1).sequence);
assert.deepEqual(replay.body.items.map((event) => event.sequence), events.map((event) => event.sequence));
assert.equal(replay.body.nextPageToken, null);

const publicEvidence = JSON.stringify({ incident, approval: approved.body, events, completed: completed.body });
for (const prohibited of ['fault-controller', 'worker-stopped/activate', 'ground_truth', 'incident-remediation-development-token', 'incident-approval-evidence-development-key-v1']) {
  assert.equal(publicEvidence.includes(prohibited), false, `public Incident data leaked ${prohibited}`);
}

console.log(`incident_task=${incident.id}`);
console.log(`incident_events=${events.length}`);
console.log('incident golden API E2E passed');
