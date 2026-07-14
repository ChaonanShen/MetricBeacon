import { describe, expect, it } from 'vitest';

import { defaultChartId, deriveChartGroups } from './chart-groups';

const task = (id: string, inputMessageId = `message-${id}`) => ({ id, inputMessageId, createdAt: `2026-07-15T10:0${id.slice(-1)}:00Z` }) as never;
const message = (id: string, taskId: string, content: string, role = 'user') => ({ id, taskId, role, content }) as never;
const chart = (id: string) => ({ chart: { id, taskId: id.split('-')[1], title: id } }) as never;

describe('deriveChartGroups', () => {
  it('turns newest-first tasks into oldest-first non-empty groups without mixing charts', () => {
    const groups = deriveChartGroups(
      [task('task-2'), task('task-1')],
      [message('message-task-1', 'task-1', 'first'), message('message-task-2', 'task-2', 'second')],
      { 'task-1': { charts: { a: chart('chart-task-1-a') } }, 'task-2': { charts: { b: chart('chart-task-2-b') } } } as never,
    );
    expect(groups.map(({ taskId }) => taskId)).toEqual(['task-1', 'task-2']);
    expect(groups[0].charts.map(({ chart }) => chart.id)).toEqual(['chart-task-1-a']);
    expect(groups[1].charts.map(({ chart }) => chart.id)).toEqual(['chart-task-2-b']);
    expect(defaultChartId(groups)).toBe('chart-task-2-b');
  });

  it('uses inputMessageId, then task user message, then a stable fallback', () => {
    const groups = deriveChartGroups(
      [task('task-3', 'missing-3'), task('task-2', 'missing-2'), task('task-1', 'preferred')],
      [message('preferred', 'task-1', 'preferred prompt'), message('other', 'task-1', 'other prompt'), message('fallback', 'task-2', 'task fallback'), message('assistant', 'task-3', 'ignore', 'assistant')],
      { 'task-1': { charts: { a: chart('chart-task-1') } }, 'task-2': { charts: { b: chart('chart-task-2') } }, 'task-3': { charts: { c: chart('chart-task-3') } } } as never,
    );
    expect(groups.map(({ prompt }) => prompt)).toEqual(['preferred prompt', 'task fallback', '分析请求']);
  });

  it('ignores empty tasks and adds an incremental chart only to its group', () => {
    const tasks = [task('task-2'), task('task-1')];
    const before = deriveChartGroups(tasks, [], { 'task-1': { charts: { a: chart('chart-task-1-a') } }, 'task-2': { charts: {} } } as never);
    const after = deriveChartGroups(tasks, [], { 'task-1': { charts: { a: chart('chart-task-1-a'), b: chart('chart-task-1-b') } }, 'task-2': { charts: {} } } as never);
    expect(before).toHaveLength(1);
    expect(after.map(({ taskId }) => taskId)).toEqual(['task-1']);
    expect(after[0].charts).toHaveLength(2);
  });
});
