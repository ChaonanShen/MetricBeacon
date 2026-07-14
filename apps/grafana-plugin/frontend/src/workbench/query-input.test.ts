import { describe, expect, it } from 'vitest';

import { createTaskInput } from './query-input';

describe('createTaskInput', () => {
  it('submits only natural language and the fixed datasource', () => {
    const input = createTaskInput('session-1', '  查看 CPU，每隔 5s 一个点  ');
    expect(input).toEqual({
      sessionId: 'session-1',
      message: '查看 CPU，每隔 5s 一个点',
      analysisContext: { datasourceUid: 'prometheus-main' },
    });
    expect(input.analysisContext).not.toHaveProperty('timeRange');
    expect(input.analysisContext).not.toHaveProperty('resolution');
  });
});
