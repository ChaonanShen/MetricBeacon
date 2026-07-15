import { describe, expect, it } from 'vitest';

import { createInitialSessionWorkbenchState, initialSessionWorkbenchState, sessionReducer, type SessionAction } from './session-reducer';

const task = { id: 'task-1', sessionId: 'session-1', status: 'created', inputMessageId: 'message-1', datasourceUid: 'prometheus-main', timeRange: { from: '2026-07-14T00:00:00Z', to: '2026-07-14T00:30:00Z' }, latestSequence: 0, error: null, createdAt: '2026-07-14T00:00:00Z', startedAt: null, completedAt: null, updatedAt: '2026-07-14T00:00:00Z', version: 1 } as never;
const userMessage = { id: 'message-1', sessionId: 'session-1', taskId: 'task-1', role: 'user', content: 'show cpu', createdAt: '2026-07-14T00:00:00Z' } as never;
const selected = () => createInitialSessionWorkbenchState('session-1');
const historyLoaded: SessionAction = { type: 'history.loaded', sessionId: 'session-1', messages: [userMessage], tasks: [task], messageNextPageToken: null, taskNextPageToken: null };

describe('sessionReducer', () => {
  it('merges keyset history, initializes task runtimes, and retains an active task', () => {
    const state = sessionReducer(selected(), { ...historyLoaded, messageNextPageToken: 'older-messages', taskNextPageToken: 'older-tasks' });

    expect(state.messageOrder).toEqual(['message-1']);
    expect(state.runtimeByTaskId['task-1'].latestSequence).toBe(0);
    expect(state.replayedTaskIds).toEqual({});
    expect(state.activeTaskId).toBe('task-1');
    expect(state.messageNextPageToken).toBe('older-messages');
  });

  it('marks a terminal event as inactive without removing its runtime history', () => {
    const loaded = sessionReducer(selected(), historyLoaded);
    const completed = sessionReducer(loaded, { type: 'task.event', event: { eventId: 'event-1', taskId: 'task-1', sessionId: 'session-1', sequence: 1, type: 'task.completed', timestamp: '2026-07-14T00:00:01Z', payload: {} } as never });

    expect(completed.activeTaskId).toBeUndefined();
    expect(completed.tasksById['task-1'].status).toBe('completed');
    expect(completed.runtimeByTaskId['task-1'].latestSequence).toBe(1);
  });

  it('keeps a locally active task subscribed until its terminal event is reduced', () => {
    const created = sessionReducer(selected(), { type: 'task.created', sessionId: 'session-1', task });
    const completedFromHistory = Object.assign({}, task, { status: 'completed', latestSequence: 12 }) as never;
    const refreshed = sessionReducer(created, { type: 'history.loaded', sessionId: 'session-1', messages: [], tasks: [completedFromHistory], messageNextPageToken: null, taskNextPageToken: null });

    expect(refreshed.activeTaskId).toBe('task-1');
    expect(refreshed.tasksById['task-1'].status).toBe('created');

    const completed = sessionReducer(refreshed, { type: 'task.event', event: { eventId: 'event-1', taskId: 'task-1', sessionId: 'session-1', sequence: 1, type: 'task.completed', timestamp: '2026-07-14T00:00:01Z', payload: {} } as never });
    expect(completed.activeTaskId).toBeUndefined();
    expect(completed.tasksById['task-1'].status).toBe('completed');
  });

  it('records a completed finite replay without advancing its event sequence', () => {
    const loaded = sessionReducer(selected(), historyLoaded);
    const replayed = sessionReducer(loaded, { type: 'task.replayed', sessionId: 'session-1', taskId: 'task-1', targetSequence: 5 });

    expect(replayed.replayedTaskIds).toEqual({ 'task-1': true });
    expect(replayed.runtimeByTaskId['task-1'].latestSequence).toBe(0);
  });

  it('selects a fresh conversation and clears all restored state', () => {
    const loaded = sessionReducer(selected(), { ...historyLoaded, messageNextPageToken: 'older-messages', taskNextPageToken: 'older-tasks' });
    const cleared = sessionReducer(loaded, { type: 'session.selected' });

    expect(cleared).toEqual(initialSessionWorkbenchState);
  });

  it('rejects late history, replay, task creation, and events from the previous Session', () => {
    const switched = sessionReducer(selected(), { type: 'session.selected', sessionId: 'session-2' });
    const afterHistory = sessionReducer(switched, historyLoaded);
    const afterReplay = sessionReducer(afterHistory, { type: 'task.replayed', sessionId: 'session-1', taskId: 'task-1', targetSequence: 5 });
    const afterCreate = sessionReducer(afterReplay, { type: 'task.created', sessionId: 'session-1', task });
    const afterEvent = sessionReducer(afterCreate, { type: 'task.event', event: { eventId: 'event-late', taskId: 'task-1', sessionId: 'session-1', sequence: 1, type: 'assistant.message.delta', timestamp: '2026-07-14T00:00:01Z', payload: { delta: 'old conversation' } } as never });

    expect(afterHistory).toBe(switched);
    expect(afterReplay).toBe(switched);
    expect(afterCreate).toBe(switched);
    expect(afterEvent).toBe(switched);
  });
});
