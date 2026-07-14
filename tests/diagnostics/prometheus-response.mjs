import { pathToFileURL } from 'node:url';

import { formatMetricSummary, summarizeMetricSeries } from './metric-sanity.mjs';

async function readStdin() {
  let raw = '';
  process.stdin.setEncoding('utf8');
  for await (const chunk of process.stdin) {
    raw += chunk;
  }
  return raw;
}

export function summarizePrometheusResponse(raw, view) {
  let response;
  try {
    response = JSON.parse(raw);
  } catch {
    throw new Error(`${view}: Prometheus returned invalid JSON`);
  }
  if (response.status !== 'success') {
    throw new Error(`${view}: Prometheus status was not success`);
  }
  if (response.data?.resultType !== 'vector') {
    throw new Error(`${view}: expected vector, received ${response.data?.resultType ?? 'missing'}`);
  }
  const result = response.data?.result;
  if (!Array.isArray(result) || result.length === 0) {
    throw new Error(`${view}: Prometheus returned no series`);
  }
  const normalized = result.map((series) => {
    const sample = series?.value;
    if (!Array.isArray(sample) || sample.length !== 2 || !Number.isFinite(Number(sample[0])) || !Number.isFinite(Number(sample[1]))) {
      throw new Error(`${view}: Prometheus returned an invalid sample`);
    }
    return { labels: series.metric, points: [{ timestamp: Number(sample[0]), value: Number(sample[1]) }] };
  });
  return { resultType: response.data.resultType, ...summarizeMetricSeries(view, normalized) };
}

async function main() {
  const view = process.argv[2]?.trim();
  if (!view) {
    throw new Error('view argument is required');
  }
  const summary = summarizePrometheusResponse(await readStdin(), view);
  process.stdout.write(`${formatMetricSummary('prometheus', summary, summary.resultType)}\n`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
    process.exitCode = 1;
  });
}
