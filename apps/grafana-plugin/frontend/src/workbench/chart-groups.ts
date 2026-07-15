import type { Message, Task } from '../api/resource';
import type { WorkbenchChart, WorkbenchState } from './types';

export type ChartGroup = {
  taskId: string;
  createdAt: string;
  prompt: string;
  charts: WorkbenchChart[];
};

export function deriveChartGroups(tasksNewestFirst: Task[], messages: Message[], runtimeByTaskId: Record<string, WorkbenchState>): ChartGroup[] {
  const messagesById = new Map(messages.map((message) => [message.id, message]));
  return [...tasksNewestFirst].reverse().flatMap((task) => {
    const charts = Object.values(runtimeByTaskId[task.id]?.charts ?? {});
    if (charts.length === 0) return [];
    const input = messagesById.get(task.inputMessageId);
    const fallback = messages.find((message) => message.taskId === task.id && message.role === 'user');
    return [{
      taskId: task.id,
      createdAt: task.createdAt,
      prompt: input?.role === 'user' ? input.content : fallback?.content ?? '分析请求',
      charts,
    }];
  });
}

export function autoFocusTask(groups: ChartGroup[], activeTaskId: string | undefined, lastFocusedTaskId: string | undefined, historyReady: boolean, historyFocused: boolean) {
  const active = activeTaskId ? groups.find((group) => group.taskId === activeTaskId) : undefined;
  if (active && active.taskId !== lastFocusedTaskId) return { taskId: active.taskId, behavior: 'smooth' as const };
  const latest = groups.at(-1);
  if (historyReady && !historyFocused && latest) return { taskId: latest.taskId, behavior: 'auto' as const };
  return undefined;
}

export function compensatedScrollTop(beforeTop: number, beforeHeight: number, afterHeight: number): number {
  return beforeTop + Math.max(0, afterHeight - beforeHeight);
}
