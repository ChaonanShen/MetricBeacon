import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import test from 'node:test';

import { probeDeepSeek } from './deepseek-smoke.mjs';

async function withFakeDeepSeek(t, behavior, run) {
  const requests = [];
  const server = createServer(async (request, response) => {
    let body = '';
    for await (const chunk of request) {
      body += chunk;
    }
    requests.push({ method: request.method, url: request.url, authorization: request.headers.authorization, body });
    const reply = behavior(request, body);
    response.writeHead(reply.status ?? 200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(reply.body));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve())));
  const address = server.address();
  await run(`http://127.0.0.1:${address.port}`, requests);
}

test('checks model availability and receives the strict pong response', async (t) => {
  await withFakeDeepSeek(t, (request, rawBody) => {
    if (request.url === '/models') {
      return { body: { data: [{ id: 'deepseek-v4-flash' }] } };
    }
    const body = JSON.parse(rawBody);
    assert.equal(body.model, 'deepseek-v4-flash');
    assert.deepEqual(body.thinking, { type: 'disabled' });
    return { body: { choices: [{ message: { content: '{"status":"ok","answer":"pong"}' } }] } };
  }, async (baseURL, requests) => {
    const result = await probeDeepSeek({ apiKey: 'test-key', baseURL });
    assert.equal(result.model, 'deepseek-v4-flash');
    assert.deepEqual(result.response, { status: 'ok', answer: 'pong' });
    assert.equal(requests.length, 2);
    assert.ok(requests.every((request) => request.authorization === 'Bearer test-key'));
  });
});

test('stops when the configured model is unavailable', async (t) => {
  await withFakeDeepSeek(t, () => ({ body: { data: [{ id: 'another-model' }] } }), async (baseURL, requests) => {
    await assert.rejects(() => probeDeepSeek({ apiKey: 'test-key', baseURL }), /configured model .* was not available/);
    assert.equal(requests.length, 1);
  });
});

test('classifies a chat HTTP failure without returning its body', async (t) => {
  await withFakeDeepSeek(t, (request) => request.url === '/models'
    ? { body: { data: [{ id: 'deepseek-v4-flash' }] } }
    : { status: 503, body: { error: { message: 'sensitive upstream detail' } } }, async (baseURL) => {
    await assert.rejects(() => probeDeepSeek({ apiKey: 'test-key', baseURL }), (error) => {
      assert.equal(error.message, 'chat: HTTP 503');
      assert.doesNotMatch(error.message, /sensitive upstream detail|test-key/);
      return true;
    });
  });
});

test('rejects non-JSON model content', async (t) => {
  await withFakeDeepSeek(t, (request) => request.url === '/models'
    ? { body: { data: [{ id: 'deepseek-v4-flash' }] } }
    : { body: { choices: [{ message: { content: 'pong' } }] } }, async (baseURL) => {
    await assert.rejects(() => probeDeepSeek({ apiKey: 'test-key', baseURL }), /response content was not strict JSON/);
  });
});
