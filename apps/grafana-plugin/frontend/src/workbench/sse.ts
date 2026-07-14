import type { GeneratedTaskEvent } from '../api/resource';

type Event = Omit<GeneratedTaskEvent, 'payload'> & { payload: Record<string, unknown> };

export type Subscription = { close(): void };

export function subscribeTaskEvents(url: (afterSequence: number) => string, afterSequence: () => number, onEvent: (event: Event) => void, onError: () => void, isTerminal: (event: Event) => boolean = () => false): Subscription {
  let source: EventSource | undefined;
  let closed = false;
  let attempt = 0;
  let reconnectTimer: number | undefined;
  let lastAcceptedSequence = afterSequence();

  const reconnect = () => {
    if (closed || reconnectTimer !== undefined) {
      return;
    }
    attempt += 1;
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = undefined;
      connect();
    }, Math.min(5_000, 250 * 2 ** attempt));
    onError();
  };

  const connect = () => {
    if (closed) {
      return;
    }
    const currentSource = new EventSource(url(lastAcceptedSequence));
    source = currentSource;
    currentSource.onmessage = (message) => receive(currentSource, message.data);
    for (const eventType of ['task.created', 'task.status_changed', 'assistant.message.started', 'assistant.message.delta', 'assistant.message.completed', 'tool.started', 'tool.completed', 'tool.failed', 'metric.candidates_created', 'chart.created', 'chart.execution_completed', 'task.completed', 'task.failed']) {
      currentSource.addEventListener(eventType, (message) => receive(currentSource, (message as MessageEvent<string>).data));
    }
    currentSource.onerror = () => {
      if (closed || source !== currentSource) {
        return;
      }
      currentSource.close();
      reconnect();
    };
  };
  const receive = (currentSource: EventSource, raw: string) => {
    try {
      const event = JSON.parse(raw) as Event;
      if (event.sequence <= lastAcceptedSequence) {
        return;
      }
      if (event.sequence !== lastAcceptedSequence + 1) {
        currentSource.close();
        reconnect();
        return;
      }
      lastAcceptedSequence = event.sequence;
      attempt = 0;
      onEvent(event);
      if (isTerminal(event)) {
        closed = true;
        currentSource.close();
      }
    } catch {
      currentSource.close();
      reconnect();
    }
  };
  connect();
  return { close: () => {
    closed = true;
    if (reconnectTimer !== undefined) {
      window.clearTimeout(reconnectTimer);
    }
    source?.close();
  } };
}
