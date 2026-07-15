import type { GeneratedTaskEvent } from '../api/resource';
import { initialWorkbenchState, type ChartWire, type ExecutionWire, type IncidentTimelineItem, type WorkbenchState } from './types';

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
  if (incoming.sequence !== state.latestSequence + 1) {
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
    default: {
      const timeline = incidentTimelineItem(incoming);
      return timeline ? { ...next, incidentTimeline: [...next.incidentTimeline, timeline] } : next;
    }
  }
}

function incidentTimelineItem(event: Event): IncidentTimelineItem | undefined {
  const text = (value: unknown) => typeof value === 'string' ? value : undefined;
  switch (event.type) {
    case 'alert.received': return { sequence: event.sequence, type: event.type, title: 'Prometheus 告警已接收', detail: text(event.payload.alertName) };
    case 'playbook.resolved': return { sequence: event.sequence, type: event.type, title: '已匹配版本化 Playbook', detail: text(event.payload.playbookId) };
    case 'assets.pinned': return { sequence: event.sequence, type: event.type, title: 'Knowledge、Skill 与 Playbook 已固定' };
    case 'diagnosis.completed': return { sequence: event.sequence, type: event.type, title: '只读诊断完成', detail: [text(event.payload.primaryHypothesis), typeof event.payload.confidence === 'number' ? `置信度 ${Math.round(event.payload.confidence * 100)}%` : undefined].filter(Boolean).join(' · ') };
    case 'intent.prepared': return { sequence: event.sequence, type: event.type, title: '受控修复 Intent/Diff 已生成', detail: `concurrency ${String(event.payload.beforeConcurrency ?? '—')} → ${String(event.payload.afterConcurrency ?? '—')}` };
    case 'approval.requested': return { sequence: event.sequence, type: event.type, title: '等待人工审批' };
    case 'approval.decided': return { sequence: event.sequence, type: event.type, title: '审批决定已持久化', detail: text(event.payload.status) };
    case 'remediation.started': return { sequence: event.sequence, type: event.type, title: '类型化修复开始', detail: text(event.payload.operationId) };
    case 'remediation.reconciled': return { sequence: event.sequence, type: event.type, title: '执行回执已核对', detail: text(event.payload.state) };
    case 'verification.runtime': return { sequence: event.sequence, type: event.type, title: '运行状态验证通过', detail: `worker ${String(event.payload.configuredConcurrency ?? '—')} / active ${String(event.payload.activeWorkers ?? '—')}` };
    case 'verification.metrics': return { sequence: event.sequence, type: event.type, title: 'Prometheus 恢复指标验证通过', detail: '连续两个固定 30 秒窗口' };
    case 'verification.business': return { sequence: event.sequence, type: event.type, title: '真实订单业务探针通过', detail: `${String(event.payload.durationMs ?? '—')} ms` };
    case 'audit.recorded': return { sequence: event.sequence, type: event.type, title: '权威审计记录已持久化', detail: [text(event.payload.action), text(event.payload.outcome)].filter(Boolean).join(' · ') };
    default: return undefined;
  }
}

export function resetWorkbench(): Action {
	return { type: '__reset' };
}
