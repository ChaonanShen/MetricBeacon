import { getBackendSrv } from '@grafana/runtime';

import type { components } from './generated/plugin-resource';

export type Session = components['schemas']['session.schema'];
export type Task = components['schemas']['task.schema'];
export type CreateTask = components['schemas']['create-task-request.schema'];
export type GeneratedTaskEvent = components['schemas']['task-events.schema'];

const resourceBase = '/api/plugins/mini-torchbearing-app/resources';

export const resourceClient = {
  createSession(title: string): Promise<Session> {
    return getBackendSrv().post<Session>(`${resourceBase}/sessions`, { title });
  },
  getSession(sessionId: string): Promise<Session> {
    return getBackendSrv().get<Session>(`${resourceBase}/sessions/${encodeURIComponent(sessionId)}`);
  },
  createTask(input: CreateTask, idempotencyKey: string): Promise<Task> {
    return getBackendSrv().post<Task>(`${resourceBase}/tasks`, input, { headers: { 'Idempotency-Key': idempotencyKey } });
  },
  getTask(taskId: string): Promise<Task> {
    return getBackendSrv().get<Task>(`${resourceBase}/tasks/${encodeURIComponent(taskId)}`);
  },
  eventURL(taskId: string, afterSequence: number): string {
    return `${resourceBase}/tasks/${encodeURIComponent(taskId)}/events?afterSequence=${afterSequence}`;
  },
};
