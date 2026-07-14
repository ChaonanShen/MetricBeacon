import type { CreateTask } from '../api/resource';

export const rangeOptions = [
  { value: '30s', label: '最近 30 秒' },
  { value: '1m', label: '最近 1 分钟' },
  { value: '5m', label: '最近 5 分钟' },
  { value: '30m', label: '最近 30 分钟' },
  { value: '1h', label: '最近 1 小时' },
  { value: '6h', label: '最近 6 小时' },
] as const;

export const resolutionOptions = [
  { value: 'auto', label: '自动（最多约 300 点）' },
  { value: '5', label: '每 5 秒' },
  { value: '10', label: '每 10 秒' },
  { value: '15', label: '每 15 秒' },
  { value: '30', label: '每 30 秒' },
  { value: '60', label: '每 1 分钟' },
  { value: '120', label: '每 2 分钟' },
  { value: '300', label: '每 5 分钟' },
] as const;

export type RangeOption = typeof rangeOptions[number]['value'];
export type ResolutionOption = typeof resolutionOptions[number]['value'];

export function createTaskInput(sessionId: string, message: string, range: RangeOption, resolution: ResolutionOption): CreateTask {
  const resolved = resolution === 'auto'
    ? { mode: 'auto' as const }
    : { stepSeconds: Number(resolution) as 5 | 10 | 15 | 30 | 60 | 120 | 300 };
  return {
    sessionId,
    message: message.trim(),
    analysisContext: {
      datasourceUid: 'prometheus-main',
      timeRange: { relativeDuration: range },
      resolution: resolved,
    },
  };
}
