import { describe, expect, it, vi } from 'vitest';

vi.mock('@grafana/data', () => ({
  FieldType: { time: 'time', number: 'number' },
  createDataFrame: (frame: unknown) => frame,
}));

import { ChartWireToDataFrame } from './mapper';

describe('ChartWireToDataFrame', () => {
  it('keeps persisted timestamps, values, and labels aligned', () => {
    const frame = ChartWireToDataFrame([
      {
        name: 'cpu',
        labels: { instance: 'mock:9100' },
        points: [
          { timestamp: '2026-07-13T15:00:00Z', value: 0.25 },
          { timestamp: '2026-07-13T15:01:00Z', value: 0.5 },
        ],
      },
    ]);

    expect(frame.fields.map((field) => field.name)).toEqual(['Time', 'cpu']);
    expect(Array.from(frame.fields[0].values)).toEqual([Date.parse('2026-07-13T15:00:00Z'), Date.parse('2026-07-13T15:01:00Z')]);
    expect(Array.from(frame.fields[1].values)).toEqual([0.25, 0.5]);
    expect(frame.fields[1].labels).toEqual({ instance: 'mock:9100' });
  });
});
