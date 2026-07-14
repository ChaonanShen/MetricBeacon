import { createDataFrame, FieldType, type DataFrame } from '@grafana/data';

import type { ChartSeriesWire } from './types';

// ChartWireToDataFrame is deliberately a pure adapter: it receives only
// persisted TaskEvent series and creates Grafana-native data frames.
export function ChartWireToDataFrame(series: ChartSeriesWire[], unit?: string): DataFrame {
  const timestamps = series[0]?.points.map((point) => new Date(point.timestamp).valueOf()) ?? [];
  return createDataFrame({
    name: 'node_exporter mock',
    fields: [
      { name: 'Time', type: FieldType.time, values: timestamps },
      ...series.map((item) => ({
        name: item.name,
        type: FieldType.number,
        labels: item.labels,
        config: { displayName: item.name, ...(unit ? { unit } : {}) },
        values: item.points.map((point) => point.value),
      })),
    ],
  });
}
