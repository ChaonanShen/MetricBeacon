import { describe, expect, it, vi } from 'vitest';

vi.mock('@grafana/data', () => ({
  dateTime: (value: string) => ({ value }),
}));

import { chartTimeRange } from './time-range';

describe('chartTimeRange', () => {
  const chart = {
    queries: [{ timeRange: { from: '2026-07-13T14:30:00Z', to: '2026-07-13T15:00:00Z' } }],
  } as never;

  it('prefers the persisted execution sample range', () => {
    const range = chartTimeRange(chart, { sampleRange: { from: '2026-07-13T14:40:00Z', to: '2026-07-13T14:50:00Z' } } as never);

    expect(range).toMatchObject({
      from: { value: '2026-07-13T14:40:00Z' },
      to: { value: '2026-07-13T14:50:00Z' },
      raw: { from: '2026-07-13T14:40:00Z', to: '2026-07-13T14:50:00Z' },
    });
  });

  it('falls back to the persisted query range', () => {
    expect(chartTimeRange(chart)).toMatchObject({ raw: { from: '2026-07-13T14:30:00Z', to: '2026-07-13T15:00:00Z' } });
  });
});
