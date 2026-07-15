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

  it('builds a bounded Incident timeline from public event summaries', () => {
    const diagnosed = taskEventReducer(initialWorkbenchState, event(1, 'diagnosis.completed', { primaryHypothesis: 'worker_stopped', confidence: 0.99 }));
    const intent = taskEventReducer(diagnosed, event(2, 'intent.prepared', { beforeConcurrency: 0, afterConcurrency: 2, intentDigest: 'not-rendered-here' }));
    const verified = taskEventReducer(intent, event(3, 'verification.business', { durationMs: 203 }));

    expect(verified.incidentTimeline).toEqual([
      { sequence: 1, type: 'diagnosis.completed', title: '只读诊断完成', detail: 'worker_stopped · 置信度 99%' },
      { sequence: 2, type: 'intent.prepared', title: '受控修复 Intent/Diff 已生成', detail: 'concurrency 0 → 2' },
      { sequence: 3, type: 'verification.business', title: '真实订单业务探针通过', detail: '203 ms' },
    ]);
    expect(JSON.stringify(verified.incidentTimeline)).not.toContain('not-rendered-here');
  });
});
