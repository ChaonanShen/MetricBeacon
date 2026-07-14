import { describe, expect, it } from 'vitest';

import { initialSessionWorkbenchState, sessionReducer } from './session-reducer';

const task = { id: 'task-1', sessionId: 'session-1', status: 'created', inputMessageId: 'message-1', datasourceUid: 'mock-prometheus', timeRange: { from: '2026-07-14T00:00:00Z', to: '2026-07-14T00:30:00Z' }, latestSequence: 0, error: null, createdAt: '2026-07-14T00:00:00Z', startedAt: null, completedAt: null, updatedAt: '2026-07-14T00:00:00Z', version: 1 } as never;
const userMessage = { id: 'message-1', sessionId: 'session-1', taskId: 'task-1', role: 'user', content: 'show cpu', createdAt: '2026-07-14T00:00:00Z' } as never;

describe('sessionReducer', () => {
  it('merges keyset history, initializes task runtimes, and retains an active task', () => {
    const state = sessionReducer(initialSessionWorkbenchState, { type: 'history.loaded', messages: [userMessage], tasks: [task], messageNextPageToken: 'older-messages', taskNextPageToken: 'older-tasks' });

    expect(state.messageOrder).toEqual(['message-1']);
    expect(state.runtimeByTaskId['task-1'].latestSequence).toBe(0);
    expect(state.activeTaskId).toBe('task-1');
    expect(state.messageNextPageToken).toBe('older-messages');
  });

  it('marks a terminal event as inactive without removing its runtime history', () => {
    const loaded = sessionReducer(initialSessionWorkbenchState, { type: 'history.loaded', messages: [userMessage], tasks: [task], messageNextPageToken: null, taskNextPageToken: null });
    const completed = sessionReducer(loaded, { type: 'task.event', event: { eventId: 'event-1', taskId: 'task-1', sessionId: 'session-1', sequence: 1, type: 'task.completed', timestamp: '2026-07-14T00:00:01Z', payload: {} } as never });

    expect(completed.activeTaskId).toBeUndefined();
    expect(completed.tasksById['task-1'].status).toBe('completed');
    expect(completed.runtimeByTaskId['task-1'].latestSequence).toBe(1);
  });
});
