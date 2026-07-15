import { describe, expect, it } from 'vitest';

import type { ExecutionWire } from './types';
import { deriveChartStatus } from './chart-view';

describe('deriveChartStatus', () => {
  it('maps the absence of a durable execution to an in-progress view', () => {
    expect(deriveChartStatus()).toEqual({ tone: 'info', text: '查询中' });
  });

  it.each([
    ['success', { tone: 'success', text: '已加载' }],
    ['failed', { tone: 'error', text: '加载失败' }],
    ['unexpected', { tone: 'neutral', text: '状态未知' }],
  ] as const)('maps %s without inventing chart data', (status, expected) => {
    expect(deriveChartStatus(execution(status))).toEqual(expected);
  });
});

function execution(status: string): ExecutionWire {
  return { id: 'execution-1', status, series: [], seriesCount: 0 };
}
