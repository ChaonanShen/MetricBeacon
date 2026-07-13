import type { GeneratedTaskEvent } from '../api/resource';
import { initialWorkbenchState, type ChartWire, type ExecutionWire, type WorkbenchState } from './types';

type EventPayload = Record<string, unknown>;
type Event = Omit<GeneratedTaskEvent, 'payload'> & { payload: EventPayload };
type Action = Event | { type: '__reset' };

export function taskEventReducer(state: WorkbenchState, incoming: Action): WorkbenchState {
	if (incoming.type === '__reset') {
		return initialWorkbenchState;
	}
  if (incoming.sequence <= state.latestSequence) {
    return state;
  }
  const next: WorkbenchState = { ...state, latestSequence: incoming.sequence };
  switch (incoming.type) {
    case 'assistant.message.delta':
      return { ...next, assistantText: `${next.assistantText}${String(incoming.payload.delta ?? '')}` };
    case 'assistant.message.completed': {
      const message = incoming.payload.message as { content?: string } | undefined;
      return { ...next, assistantText: message?.content ?? next.assistantText };
    }
    case 'task.status_changed':
      return { ...next, taskStatus: incoming.payload.status as WorkbenchState['taskStatus'] };
    case 'task.completed':
      return { ...next, taskStatus: 'completed' };
    case 'task.failed': {
      const error = incoming.payload.error as { code?: string; message?: string } | undefined;
      return { ...next, taskStatus: 'failed', error: { code: error?.code ?? 'internal_error', message: error?.message ?? '分析失败' } };
    }
    case 'chart.created': {
      const chart = incoming.payload.chart as ChartWire | undefined;
      if (!chart?.id) {
        return next;
      }
      return { ...next, charts: { ...next.charts, [chart.id]: { chart } } };
    }
    case 'chart.execution_completed': {
      const chartId = String(incoming.payload.chartId ?? '');
      const current = next.charts[chartId];
      if (!chartId || !current) {
        return next;
      }
      return { ...next, charts: { ...next.charts, [chartId]: { ...current, execution: incoming.payload.execution as ExecutionWire } } };
    }
    default:
      return next;
  }
}

export function resetWorkbench(): Action {
	return { type: '__reset' };
}
