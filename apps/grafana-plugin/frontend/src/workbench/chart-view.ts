import type { ExecutionWire } from './types';

export type ChartStatusView = {
  tone: 'info' | 'success' | 'error' | 'neutral';
  text: string;
};

export function deriveChartStatus(execution?: ExecutionWire): ChartStatusView {
  if (!execution) return { tone: 'info', text: '查询中' };
  if (execution.status === 'success') return { tone: 'success', text: '已加载' };
  if (execution.status === 'failed') return { tone: 'error', text: '加载失败' };
  return { tone: 'neutral', text: '状态未知' };
}
