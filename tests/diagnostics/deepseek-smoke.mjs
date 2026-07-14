import { pathToFileURL } from 'node:url';

async function requestJSON(fetchImpl, url, options, stage, timeoutMs) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    let response;
    try {
      response = await fetchImpl(url, { ...options, signal: controller.signal });
    } catch {
      throw new Error(`${stage}: request failed`);
    }
    if (!response.ok) {
      throw new Error(`${stage}: HTTP ${response.status}`);
    }
    try {
      return await response.json();
    } catch {
      throw new Error(`${stage}: response was not JSON`);
    }
  } finally {
    clearTimeout(timeout);
  }
}

export async function probeDeepSeek({
  apiKey,
  baseURL = 'https://api.deepseek.com',
  model = 'deepseek-v4-flash',
  fetchImpl = fetch,
  timeoutMs = 30_000,
}) {
  if (!apiKey?.trim()) {
    throw new Error('DEEPSEEK_API_KEY is required');
  }
  if (!baseURL?.trim() || !model?.trim()) {
    throw new Error('DEEPSEEK_BASE_URL and DEEPSEEK_MODEL are required');
  }
  const endpoint = baseURL.trim().replace(/\/+$/, '');
  const headers = { Authorization: `Bearer ${apiKey.trim()}` };
  const models = await requestJSON(fetchImpl, `${endpoint}/models`, { headers }, 'models', timeoutMs);
  const available = Array.isArray(models?.data) ? models.data.some((item) => item?.id === model.trim()) : false;
  if (!available) {
    throw new Error(`models: configured model ${model.trim()} was not available`);
  }

  const startedAt = Date.now();
  const completion = await requestJSON(fetchImpl, `${endpoint}/chat/completions`, {
    method: 'POST',
    headers: { ...headers, 'Content-Type': 'application/json' },
    body: JSON.stringify({
      model: model.trim(),
      messages: [
        { role: 'system', content: 'Return only compact JSON. Do not use Markdown or add fields.' },
        { role: 'user', content: 'Reply with exactly {"status":"ok","answer":"pong"}.' },
      ],
      max_tokens: 64,
      temperature: 0,
      thinking: { type: 'disabled' },
    }),
  }, 'chat', timeoutMs);
  const content = completion?.choices?.[0]?.message?.content;
  if (typeof content !== 'string') {
    throw new Error('chat: response content was missing');
  }
  let answer;
  try {
    answer = JSON.parse(content);
  } catch {
    throw new Error('chat: response content was not strict JSON');
  }
  if (answer?.status !== 'ok' || answer?.answer !== 'pong') {
    throw new Error('chat: response did not match the diagnostic contract');
  }
  return { model: model.trim(), latencyMs: Date.now() - startedAt, response: answer };
}

async function main() {
  const result = await probeDeepSeek({
    apiKey: process.env.DEEPSEEK_API_KEY,
    baseURL: process.env.DEEPSEEK_BASE_URL,
    model: process.env.DEEPSEEK_MODEL,
  });
  process.stdout.write(`[deepseek] model=${result.model} latencyMs=${result.latencyMs} response=${JSON.stringify(result.response)}\n`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
    process.exitCode = 1;
  });
}
