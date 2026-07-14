import { formatMetricSummary, summarizeMetricSeries } from './metric-sanity.mjs';

export const canonicalExpressions = {
  cpu: '100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))',
  memory: '100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes',
  load: 'node_load1',
};

function eventTypeCounts(events) {
  const counts = {};
  for (const event of events) counts[event?.type ?? 'missing'] = (counts[event?.type ?? 'missing'] ?? 0) + 1;
  return counts;
}

function fail(message, events) {
  throw new Error(`${message}; eventTypes=${JSON.stringify(eventTypeCounts(events))}`);
}

function equalSets(left, right) {
  return left.size === right.size && [...left].every((value) => right.has(value));
}

export function analyzeTaskEvents(events, { expectedViews, expectedToolCalls } = {}) {
  if (!Array.isArray(events) || events.length === 0) fail('task result had no events', []);
  const taskId = events[0]?.taskId;
  const sessionId = events[0]?.sessionId;
  if (typeof taskId !== 'string' || typeof sessionId !== 'string') fail('task result was missing task/session identity', events);
  for (let index = 0; index < events.length; index += 1) {
    const event = events[index];
    if (event.taskId !== taskId || event.sessionId !== sessionId) fail('task result mixed task/session identities', events);
    if (event.sequence !== index + 1) fail(`task event sequence expected ${index + 1}, received ${event.sequence}`, events);
  }
  if (events.at(-1)?.type !== 'task.completed' || events.filter((event) => event.type === 'task.completed').length !== 1 || events.some((event) => event.type === 'task.failed')) {
    fail('task result did not have one successful terminal event', events);
  }

  const started = events.filter((event) => event.type === 'tool.started');
  const completed = events.filter((event) => event.type === 'tool.completed');
  const failed = events.filter((event) => event.type === 'tool.failed');
  if (started.length === 0) fail('completed task contained no tool calls', events);
  if (failed.length > 0) fail('completed task contained failed tools', events);
  const startedIDs = started.map((event) => event.payload?.toolCallId);
  const completedIDs = completed.map((event) => event.payload?.toolCallId);
  if (startedIDs.some((id) => typeof id !== 'string') || new Set(startedIDs).size !== startedIDs.length) fail('tool starts had missing or duplicate ids', events);
  if (completedIDs.some((id) => typeof id !== 'string') || new Set(completedIDs).size !== completedIDs.length) fail('tool completions had missing or duplicate ids', events);
  if (!equalSets(new Set(startedIDs), new Set(completedIDs))) fail('tool start/completion ids did not pair', events);
  if (expectedToolCalls !== undefined && started.length !== expectedToolCalls) fail(`expected ${expectedToolCalls} tool calls, received ${started.length}`, events);

  const charts = events.filter((event) => event.type === 'chart.created').map((event) => event.payload?.chart);
  const executions = events.filter((event) => event.type === 'chart.execution_completed');
  const executionByChart = new Map();
  for (const event of executions) {
    const chartId = event.payload?.chartId;
    if (typeof chartId !== 'string' || executionByChart.has(chartId)) fail('chart executions had missing or duplicate chart ids', events);
    executionByChart.set(chartId, event.payload?.execution);
  }
  if (charts.length === 0 || charts.length !== executions.length) fail('charts and executions were not one-to-one', events);

  const views = {};
  for (const chart of charts) {
    if (typeof chart?.id !== 'string' || !Array.isArray(chart.queries) || chart.queries.length !== 1) fail('chart did not contain exactly one query', events);
    const expression = chart.queries[0]?.expression;
    const view = Object.entries(canonicalExpressions).find(([, canonical]) => expression === canonical)?.[0];
    if (!view || views[view]) fail('chart expression was unknown or duplicated', events);
    const execution = executionByChart.get(chart.id);
    if (execution?.status !== 'success' || !Array.isArray(execution.series) || execution.seriesCount !== execution.series.length) {
      fail(`chart execution for ${view} was not a consistent success`, events);
    }
    if (!Number.isFinite(execution.durationMs) || execution.durationMs < 0) fail(`chart execution for ${view} had invalid duration`, events);
    try {
      views[view] = summarizeMetricSeries(view, execution.series);
    } catch (error) {
      fail(`chart execution for ${view} failed semantic validation: ${error instanceof Error ? error.message : String(error)}`, events);
    }
  }
  if (expectedViews) {
    const expected = new Set(expectedViews);
    if (expected.size !== expectedViews.length || !equalSets(new Set(Object.keys(views)), expected)) {
      fail(`task views did not match expected ${[...expected].sort().join(',')}`, events);
    }
  }

  const messages = events.filter((event) => event.type === 'assistant.message.completed');
  if (messages.length !== 1 || typeof messages[0].payload?.message?.content !== 'string' || !messages[0].payload.message.content.trim()) {
    fail('task result did not contain one completed assistant message', events);
  }
  return { events: events.length, toolCalls: started.length, charts: charts.length, terminal: 'task.completed', eventTypes: eventTypeCounts(events), views };
}

export function formatTaskSummary(prefix, summary) {
  return [
    `[${prefix}] events=${summary.events} toolCalls=${summary.toolCalls} charts=${summary.charts} terminal=${summary.terminal}`,
    ...Object.values(summary.views).map((view) => formatMetricSummary(prefix, view)),
  ];
}
