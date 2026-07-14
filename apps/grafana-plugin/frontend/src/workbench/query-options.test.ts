import { describe, expect, it } from 'vitest';

import { createTaskInput } from './query-options';

describe('createTaskInput', () => {
  it('submits the structured auto resolution default', () => {
    expect(createTaskInput('session-1', '  查看 CPU  ', '30m', 'auto')).toEqual({
      sessionId: 'session-1',
      message: '查看 CPU',
      analysisContext: {
        datasourceUid: 'prometheus-main',
        timeRange: { relativeDuration: '30m' },
        resolution: { mode: 'auto' },
      },
    });
  });

  it('submits an explicit registered step', () => {
    expect(createTaskInput('session-1', '查看 CPU', '5m', '5').analysisContext).toMatchObject({
      timeRange: { relativeDuration: '5m' },
      resolution: { stepSeconds: 5 },
    });
  });
});
