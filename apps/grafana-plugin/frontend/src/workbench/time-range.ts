import { dateTime, type TimeRange } from '@grafana/data';

import type { ChartWire, ExecutionWire } from './types';

export function chartTimeRange(chart: ChartWire, execution?: ExecutionWire): TimeRange | undefined {
  const range = execution?.sampleRange ?? chart.queries?.[0]?.timeRange;
  if (!range) {
    return undefined;
  }
  return {
    from: dateTime(range.from),
    to: dateTime(range.to),
    raw: { from: range.from, to: range.to },
  };
}
