import type { Task } from '../api/resource';

export type ViewTone = 'neutral' | 'info' | 'success' | 'warning' | 'error';

export type WorkbenchContextView = {
  sessionTitle: string;
  datasource: string;
  timeRange: string;
  views: string;
  step: string;
  status: string;
  statusTone: ViewTone;
  hasTask: boolean;
};

export const examplePrompts = [
  '查看最近 30 分钟 CPU 使用率',
  '分析最近 30 分钟内存可用率',
  '查看最近 30 分钟系统负载',
] as const;

export const productNavigation = [
  { id: 'workbench', label: '工作台', enabled: true },
  { id: 'sessions', label: '会话', enabled: true },
  { id: 'incidents', label: '事件', enabled: true },
  { id: 'knowledge', label: '知识库', enabled: false },
  { id: 'playbook', label: 'Playbook', enabled: false },
  { id: 'skill', label: 'Skill', enabled: false },
  { id: 'promotion', label: '晋升', enabled: false },
] as const;

const taskStatusView: Record<Task['status'], { label: string; tone: ViewTone }> = {
  created: { label: '已创建', tone: 'info' },
  planning: { label: '规划中', tone: 'info' },
  running_tools: { label: '查询中', tone: 'warning' },
  waiting_approval: { label: '等待审批', tone: 'warning' },
  executing: { label: '执行中', tone: 'warning' },
  reconciling: { label: '核对执行结果', tone: 'warning' },
  validating: { label: '校验中', tone: 'warning' },
  completed: { label: '已完成', tone: 'success' },
  failed: { label: '失败', tone: 'error' },
  cancelled: { label: '已取消', tone: 'neutral' },
};

export function deriveWorkbenchContext(sessionTitle: string | undefined, tasks: Task[], activeTask?: Task): WorkbenchContextView {
  const contextTask = activeTask ?? newestTask(tasks);
  if (!contextTask) {
    return {
      sessionTitle: sessionTitle?.trim() || '新对话',
      datasource: 'Prometheus',
      timeRange: '提交后确定',
      views: '提交后确定',
      step: '提交后确定',
      status: '等待分析',
      statusTone: 'neutral',
      hasTask: false,
    };
  }

  const status = taskStatusView[contextTask.status];
  if (contextTask.kind === 'incident_remediation') {
    return {
      sessionTitle: sessionTitle?.trim() || '未命名事件',
      datasource: contextTask.incidentPlan?.serviceRef ?? 'Order Service',
      timeRange: '事件处置',
      views: contextTask.incidentPlan?.playbook.id ?? '—',
      step: '—',
      status: status.label,
      statusTone: status.tone,
      hasTask: true,
    };
  }
  if (!contextTask.datasourceUid || !contextTask.timeRange || !contextTask.queryPlan) {
    return {
      sessionTitle: sessionTitle?.trim() || '未命名会话', datasource: '—', timeRange: '—', views: '—', step: '—',
      status: status.label, statusTone: status.tone, hasTask: true,
    };
  }
  return {
    sessionTitle: sessionTitle?.trim() || '未命名会话',
    datasource: contextTask.datasourceUid === 'prometheus-main' ? 'Prometheus' : contextTask.datasourceUid,
    timeRange: formatTimeRange(contextTask.timeRange),
    views: contextTask.queryPlan.views.length > 0 ? contextTask.queryPlan.views.map(formatView).join('、') : '—',
    step: `${contextTask.queryPlan.stepSeconds} 秒`,
    status: status.label,
    statusTone: status.tone,
    hasTask: true,
  };
}

function newestTask(tasks: Task[]): Task | undefined {
  return tasks.reduce<Task | undefined>((newest, task) => {
    if (!newest) return task;
    return task.createdAt > newest.createdAt || (task.createdAt === newest.createdAt && task.id > newest.id) ? task : newest;
  }, undefined);
}

function formatInstant(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return '—';
  return parsed.toISOString().replace('T', ' ').replace('.000Z', ' UTC');
}

function formatTimeRange(range: NonNullable<Task['timeRange']>): string {
  const from = formatInstant(range.from);
  const to = formatInstant(range.to);
  return from === '—' || to === '—' ? '—' : `${from} — ${to}`;
}

function formatView(view: NonNullable<Task['queryPlan']>['views'][number]): string {
  if (view === 'cpu') return 'CPU';
  if (view === 'memory') return '内存';
  return '系统负载';
}
