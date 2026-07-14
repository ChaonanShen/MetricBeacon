import { getBackendSrv } from '@grafana/runtime';

import type { components } from './generated/plugin-resource';

export type Session = components['schemas']['session.schema'];
export type Message = components['schemas']['message.schema'];
export type Task = components['schemas']['task.schema'];
export type CreateTask = components['schemas']['create-task-request.schema'];
export type GeneratedTaskEvent = components['schemas']['task-events.schema'];
export type MessagePage = components['schemas']['message-page.schema'];
export type TaskPage = components['schemas']['task-page.schema'];
export type TaskEventReplayPage = components['schemas']['task-event-replay-page.schema'];

const resourceBase = '/api/plugins/mini-torchbearing-app/resources';

function queryString(values: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined) query.set(key, String(value));
  }
  return `?${query.toString()}`;
}

export const resourceClient = {
  createSession(title: string): Promise<Session> {
    return getBackendSrv().post<Session>(`${resourceBase}/sessions`, { title });
  },
  getSession(sessionId: string): Promise<Session> {
    return getBackendSrv().get<Session>(`${resourceBase}/sessions/${encodeURIComponent(sessionId)}`);
  },
  listMessages(sessionId: string, pageToken?: string): Promise<MessagePage> {
    return getBackendSrv().get<MessagePage>(`${resourceBase}/sessions/${encodeURIComponent(sessionId)}/messages${queryString({ pageSize: 50, pageToken })}`);
  },
  listTasks(sessionId: string, pageToken?: string): Promise<TaskPage> {
    return getBackendSrv().get<TaskPage>(`${resourceBase}/sessions/${encodeURIComponent(sessionId)}/tasks${queryString({ pageSize: 20, pageToken })}`);
  },
  createTask(input: CreateTask, idempotencyKey: string): Promise<Task> {
    return getBackendSrv().post<Task>(`${resourceBase}/tasks`, input, { headers: { 'Idempotency-Key': idempotencyKey } });
  },
  getTask(taskId: string): Promise<Task> {
    return getBackendSrv().get<Task>(`${resourceBase}/tasks/${encodeURIComponent(taskId)}`);
  },
  replayEvents(taskId: string, pageToken?: string): Promise<TaskEventReplayPage> {
    return getBackendSrv().get<TaskEventReplayPage>(`${resourceBase}/tasks/${encodeURIComponent(taskId)}/events/replay${queryString({ pageSize: 200, pageToken, afterSequence: pageToken ? undefined : 0 })}`);
  },
  eventURL(taskId: string, afterSequence: number): string {
    return `${resourceBase}/tasks/${encodeURIComponent(taskId)}/events?afterSequence=${afterSequence}`;
  },
};
