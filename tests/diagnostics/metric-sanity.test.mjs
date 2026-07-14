import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMetricSummary, summarizeMetricSeries } from './metric-sanity.mjs';

const series = (values = [20, 30]) => [{
  labels: { instance: 'node:9100' },
  points: values.map((value, index) => ({ timestamp: `2026-07-14T00:0${index}:00Z`, value })),
}];

test('summarizes reasonable CPU, memory, and load values', () => {
  assert.deepEqual(summarizeMetricSeries('cpu', series([0, 100])), { view: 'cpu', series: 1, samples: 2, min: 0, max: 100, latest: 100 });
  assert.equal(summarizeMetricSeries('memory', series([64, 63])).latest, 63);
  assert.equal(summarizeMetricSeries('load', series([0, 12.5])).max, 12.5);
  assert.equal(formatMetricSummary('metric', summarizeMetricSeries('cpu', series([12.34567])), 'matrix'), '[metric] view=cpu resultType=matrix series=1 samples=1 min=12.3457 max=12.3457 latest=12.3457');
});

for (const [name, view, input, message] of [
  ['unsupported view', 'disk', series(), 'unsupported'],
  ['missing series', 'cpu', [], 'no series'],
  ['missing instance', 'cpu', [{ labels: {}, points: [{ timestamp: '2026-07-14T00:00:00Z', value: 1 }] }], 'instance label'],
  ['missing points', 'cpu', [{ labels: { instance: 'node' }, points: [] }], 'no points'],
  ['invalid timestamp', 'cpu', [{ labels: { instance: 'node' }, points: [{ timestamp: 'invalid', value: 1 }] }], 'timestamp was invalid'],
  ['unordered timestamps', 'cpu', [{ labels: { instance: 'node' }, points: [{ timestamp: 2, value: 1 }, { timestamp: 1, value: 2 }] }], 'strictly increasing'],
  ['non-finite value', 'cpu', series(['NaN']), 'not finite'],
  ['CPU above 100', 'cpu', series([100.1]), 'outside 0..100'],
  ['memory below zero', 'memory', series([-0.1]), 'outside 0..100'],
  ['negative load', 'load', series([-1]), 'outside >= 0'],
]) {
  test(`rejects ${name}`, () => assert.throws(() => summarizeMetricSeries(view, input), new RegExp(message)));
}

test('enforces series and sample cardinality limits', () => {
  assert.throws(() => summarizeMetricSeries('cpu', [series()[0], series()[0]], { maxSeries: 1 }), /exceeded 1 series/);
  assert.throws(() => summarizeMetricSeries('cpu', series([1, 2]), { maxSamples: 1 }), /exceeded 1 samples/);
});
