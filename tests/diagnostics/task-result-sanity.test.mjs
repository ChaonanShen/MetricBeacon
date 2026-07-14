import assert from 'node:assert/strict';
import test from 'node:test';

import { analyzeTaskEvents, canonicalExpressions, formatTaskSummary } from './task-result-sanity.mjs';

function completedTask({ value = 20, expression = canonicalExpressions.cpu, terminal = 'task.completed', includeExecution = true } = {}) {
  const base = (type, payload = {}) => ({ taskId: 'task-1', sessionId: 'session-1', type, payload });
  const events = [
    base('task.created'),
    base('tool.started', { toolCallId: 'tool-1' }),
    base('tool.completed', { toolCallId: 'tool-1' }),
    base('chart.created', { chart: { id: 'chart-1', queries: [{ expression }] } }),
  ];
  if (includeExecution) events.push(base('chart.execution_completed', { chartId: 'chart-1', execution: { status: 'success', seriesCount: 1, durationMs: 5, series: [{ labels: { instance: 'node:9100' }, points: [{ timestamp: '2026-07-14T00:00:00Z', value }] }] } }));
  events.push(base('assistant.message.completed', { message: { content: 'done' } }), base(terminal));
  return events.map((event, index) => ({ ...event, sequence: index + 1 }));
}

test('summarizes a complete durable task result', () => {
  const summary = analyzeTaskEvents(completedTask(), { expectedViews: ['cpu'], expectedToolCalls: 1 });
  assert.equal(summary.events, 7);
  assert.equal(summary.views.cpu.latestMin, 20);
  assert.deepEqual(formatTaskSummary('task', summary), [
    '[task] events=7 toolCalls=1 charts=1 terminal=task.completed',
    '[task] view=cpu series=1 samples=1 min=20 max=20 latest=20',
  ]);
});

for (const [name, mutate, message] of [
  ['sequence gap', (events) => { events[2].sequence = 4; }, 'sequence expected'],
  ['mixed task identity', (events) => { events[2].taskId = 'task-2'; }, 'mixed task'],
  ['unpaired tool', (events) => { events[2].payload.toolCallId = 'tool-2'; }, 'did not pair'],
  ['duplicate tool start', (events) => { events.splice(2, 0, { ...events[1], sequence: 3 }); events.forEach((event, index) => { event.sequence = index + 1; }); }, 'duplicate ids'],
  ['missing execution', () => {}, 'one-to-one'],
  ['unexpected view', () => {}, 'did not match expected'],
  ['unreasonable metric', () => {}, 'outside 0..100'],
  ['failed terminal', () => {}, 'successful terminal'],
]) {
  test(`rejects ${name} with safe event counts`, () => {
    const options = name === 'missing execution' ? { includeExecution: false }
      : name === 'unreasonable metric' ? { value: 101 }
        : name === 'failed terminal' ? { terminal: 'task.failed' }
          : {};
    const events = completedTask(options);
    mutate(events);
    const expectedViews = name === 'unexpected view' ? ['memory'] : ['cpu'];
    assert.throws(() => analyzeTaskEvents(events, { expectedViews, expectedToolCalls: 1 }), (error) => {
      assert.match(error.message, new RegExp(message));
      assert.match(error.message, /eventTypes=/);
      return true;
    });
  });
}
