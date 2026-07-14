import { afterEach, describe, expect, it, vi } from 'vitest';

import { subscribeTaskEvents } from './sse';

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  closed = false;
  onmessage?: (event: MessageEvent<string>) => void;
  onerror?: () => void;
  private listeners = new Map<string, Array<(event: MessageEvent<string>) => void>>();

  constructor(readonly url: string) {
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: (event: MessageEvent<string>) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }

  close() {
    this.closed = true;
  }

  emit(type: string, event: Record<string, unknown>) {
    const message = new MessageEvent<string>(type, { data: JSON.stringify(event) });
    if (type === 'message') {
      this.onmessage?.(message);
      return;
    }
    for (const listener of this.listeners.get(type) ?? []) {
      listener(message);
    }
  }
}

function event(sequence: number) {
  return { eventId: `event-${sequence}`, taskId: 'task-1', sessionId: 'session-1', sequence, type: 'task.status_changed', timestamp: '2026-07-13T15:00:00Z', payload: { status: 'running' } };
}

describe('subscribeTaskEvents', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    FakeEventSource.instances = [];
  });

  it('keeps the connection through accepted events and reconnects from the last contiguous sequence after a gap', () => {
    vi.useFakeTimers();
    vi.stubGlobal('window', { setTimeout, clearTimeout });
    vi.stubGlobal('EventSource', FakeEventSource);
    const received: number[] = [];
    const urls: number[] = [];

    const subscription = subscribeTaskEvents((after) => {
      urls.push(after);
      return `/events?afterSequence=${after}`;
    }, () => 0, (incoming) => received.push(incoming.sequence), vi.fn());

    const first = FakeEventSource.instances[0];
    first.emit('task.status_changed', event(1));
    first.emit('task.status_changed', event(1));
    first.emit('task.status_changed', event(3));

    expect(received).toEqual([1]);
    expect(first.closed).toBe(true);
    expect(urls).toEqual([0]);

    vi.advanceTimersByTime(500);
    expect(urls).toEqual([0, 1]);
    subscription.close();
  });

  it('resumes from the last accepted sequence after a transport failure', () => {
    vi.useFakeTimers();
    vi.stubGlobal('window', { setTimeout, clearTimeout });
    vi.stubGlobal('EventSource', FakeEventSource);
    const urls: number[] = [];

    const subscription = subscribeTaskEvents((after) => {
      urls.push(after);
      return `/events?afterSequence=${after}`;
    }, () => 0, vi.fn(), vi.fn());

    const first = FakeEventSource.instances[0];
    first.emit('task.status_changed', event(1));
    first.onerror?.();
    vi.advanceTimersByTime(500);

    expect(urls).toEqual([0, 1]);
    subscription.close();
  });

  it('closes permanently when it receives a terminal event', () => {
    vi.stubGlobal('window', { setTimeout, clearTimeout });
    vi.stubGlobal('EventSource', FakeEventSource);
    const subscription = subscribeTaskEvents(() => '/events?afterSequence=0', () => 0, vi.fn(), vi.fn(), (incoming) => incoming.type === 'task.completed');

    const first = FakeEventSource.instances[0];
    first.emit('task.completed', { ...event(1), type: 'task.completed' });
    first.onerror?.();

    expect(first.closed).toBe(true);
    expect(FakeEventSource.instances).toHaveLength(1);
    subscription.close();
  });
});
