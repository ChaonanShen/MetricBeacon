import assert from 'node:assert/strict';
import test from 'node:test';

import { summarizePrometheusResponse } from './prometheus-response.mjs';

test('summarizes a finite non-empty Prometheus vector', () => {
  const raw = JSON.stringify({ status: 'success', data: { resultType: 'vector', result: [
    { metric: { instance: 'node-exporter:9100' }, value: [1_784_040_000, '12.5'] },
    { metric: { instance: 'other:9100' }, value: [1_784_040_000, '18.25'] },
  ] } });
  assert.deepEqual(summarizePrometheusResponse(raw, 'cpu'), { view: 'cpu', resultType: 'vector', series: 2, samples: 2, min: 12.5, max: 18.25, latestMin: 12.5, latestMax: 18.25 });
});

for (const [name, raw, message] of [
  ['invalid JSON', '{', 'invalid JSON'],
  ['upstream error', JSON.stringify({ status: 'error' }), 'status was not success'],
  ['wrong result type', JSON.stringify({ status: 'success', data: { resultType: 'matrix', result: [] } }), 'expected vector'],
  ['empty vector', JSON.stringify({ status: 'success', data: { resultType: 'vector', result: [] } }), 'returned no series'],
  ['non-finite sample', JSON.stringify({ status: 'success', data: { resultType: 'vector', result: [{ value: [1_784_040_000, 'NaN'] }] } }), 'invalid sample'],
  ['missing instance label', JSON.stringify({ status: 'success', data: { resultType: 'vector', result: [{ metric: {}, value: [1_784_040_000, '12'] }] } }), 'instance label'],
  ['unreasonable CPU', JSON.stringify({ status: 'success', data: { resultType: 'vector', result: [{ metric: { instance: 'node' }, value: [1_784_040_000, '101'] }] } }), 'outside 0..100'],
]) {
  test(`rejects ${name}`, () => assert.throws(() => summarizePrometheusResponse(raw, 'cpu'), new RegExp(message)));
}
