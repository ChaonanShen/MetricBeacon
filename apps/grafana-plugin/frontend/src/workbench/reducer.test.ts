import { describe, expect, it } from 'vitest';

import { taskEventReducer } from './reducer';
import { initialWorkbenchState } from './types';

function event(sequence: number, type: string, payload: Record<string, unknown> = {}) {
  return { eventId: `event-${sequence}`, taskId: 'task-1', sessionId: 'session-1', sequence, type, timestamp: '2026-07-13T15:00:00Z', payload } as never;
}

describe('taskEventReducer', () => {
  it('deduplicates replayed events and refuses a sequence gap', () => {
    const first = taskEventReducer(initialWorkbenchState, event(1, 'assistant.message.delta', { delta: 'hello' }));
    const duplicate = taskEventReducer(first, event(1, 'assistant.message.delta', { delta: 'again' }));
    const gap = taskEventReducer(first, event(3, 'assistant.message.delta', { delta: 'lost' }));

    expect(first).toMatchObject({ latestSequence: 1, assistantText: 'hello' });
    expect(duplicate).toBe(first);
    expect(gap).toBe(first);
  });

  it('applies the event immediately after the accepted sequence', () => {
    const first = taskEventReducer(initialWorkbenchState, event(1, 'task.status_changed', { status: 'running' }));
    const second = taskEventReducer(first, event(2, 'task.completed'));

    expect(second).toMatchObject({ latestSequence: 2, taskStatus: 'completed' });
  });
});
