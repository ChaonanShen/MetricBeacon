import type { GeneratedTaskEvent } from '../api/resource';

type Event = Omit<GeneratedTaskEvent, 'payload'> & { payload: Record<string, unknown> };

export type Subscription = { close(): void };

export function subscribeTaskEvents(url: (afterSequence: number) => string, afterSequence: () => number, onEvent: (event: Event) => void, onError: () => void): Subscription {
  let source: EventSource | undefined;
  let closed = false;
  let attempt = 0;
  const connect = () => {
    if (closed) {
      return;
    }
    source = new EventSource(url(afterSequence()));
    source.onmessage = (message) => receive(message.data);
    for (const eventType of ['task.created', 'task.status_changed', 'assistant.message.started', 'assistant.message.delta', 'assistant.message.completed', 'tool.started', 'tool.completed', 'tool.failed', 'metric.candidates_created', 'chart.created', 'chart.execution_completed', 'task.completed', 'task.failed']) {
      source.addEventListener(eventType, (message) => receive((message as MessageEvent<string>).data));
    }
    source.onerror = () => {
      source?.close();
      if (closed) {
        return;
      }
      attempt += 1;
      window.setTimeout(connect, Math.min(5_000, 250 * 2 ** attempt));
      onError();
    };
  };
  const receive = (raw: string) => {
    try {
      const event = JSON.parse(raw) as Event;
      attempt = 0;
      onEvent(event);
    } catch {
      onError();
    }
  };
  connect();
  return { close: () => { closed = true; source?.close(); } };
}
