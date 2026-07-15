import type { GeneratedTaskEvent, Message, Task } from '../api/resource';
import { initialWorkbenchState, type WorkbenchState } from './types';
import { taskEventReducer } from './reducer';

type Event = Omit<GeneratedTaskEvent, 'payload'> & { payload: Record<string, unknown> };

export type SessionWorkbenchState = {
  sessionId?: string;
  messagesById: Record<string, Message>;
  messageOrder: string[];
  tasksById: Record<string, Task>;
  taskOrder: string[];
  runtimeByTaskId: Record<string, WorkbenchState>;
  replayedTaskIds: Record<string, true>;
  activeTaskId?: string;
  messageNextPageToken: string | null;
  taskNextPageToken: string | null;
};

export function createInitialSessionWorkbenchState(sessionId?: string): SessionWorkbenchState {
  return { sessionId, messagesById: {}, messageOrder: [], tasksById: {}, taskOrder: [], runtimeByTaskId: {}, replayedTaskIds: {}, messageNextPageToken: null, taskNextPageToken: null };
}

export const initialSessionWorkbenchState = createInitialSessionWorkbenchState();

export type SessionAction =
  | { type: 'session.selected'; sessionId?: string }
  | { type: 'history.loaded'; sessionId: string; messages: Message[]; tasks: Task[]; messageNextPageToken: string | null; taskNextPageToken: string | null }
  | { type: 'task.created'; sessionId: string; task: Task }
  | { type: 'task.replayed'; sessionId: string; taskId: string; targetSequence: number }
  | { type: 'task.event'; event: Event };

export function sessionReducer(state: SessionWorkbenchState, action: SessionAction): SessionWorkbenchState {
  switch (action.type) {
    case 'session.selected':
      return createInitialSessionWorkbenchState(action.sessionId);
    case 'history.loaded': {
      if (action.sessionId !== state.sessionId) return state;
      const messagesById = { ...state.messagesById };
      for (const message of action.messages) messagesById[message.id] = message;
      const tasksById = { ...state.tasksById };
      for (const task of action.tasks) {
        const current = tasksById[task.id];
        tasksById[task.id] = task.id === state.activeTaskId && current && !isTerminal(current.status) ? current : task;
      }
      const taskOrder = orderTasks(tasksById);
      return {
        ...state,
        messagesById,
        messageOrder: orderMessages(messagesById),
        tasksById,
        taskOrder,
        runtimeByTaskId: initializeRuntimes(state.runtimeByTaskId, taskOrder),
        activeTaskId: findActiveTask(tasksById, taskOrder),
        messageNextPageToken: action.messageNextPageToken,
        taskNextPageToken: action.taskNextPageToken,
      };
    }
    case 'task.created': {
      if (action.sessionId !== state.sessionId || action.task.sessionId !== state.sessionId) return state;
      const tasksById = { ...state.tasksById, [action.task.id]: action.task };
      const taskOrder = orderTasks(tasksById);
      return { ...state, tasksById, taskOrder, runtimeByTaskId: initializeRuntimes(state.runtimeByTaskId, taskOrder), activeTaskId: action.task.id };
    }
    case 'task.replayed': {
      if (action.sessionId !== state.sessionId) return state;
      if (state.replayedTaskIds[action.taskId]) return state;
      return { ...state, replayedTaskIds: { ...state.replayedTaskIds, [action.taskId]: true } };
    }
    case 'task.event': {
      if (action.event.sessionId !== state.sessionId) return state;
      const task = state.tasksById[action.event.taskId];
      if (!task) return state;
      const current = state.runtimeByTaskId[action.event.taskId] ?? initialWorkbenchState;
      const runtime = taskEventReducer(current, action.event);
      if (runtime === current) return state;
      const status = terminalStatus(action.event.type, runtime.taskStatus ?? task.status);
      const nextTask = status === task.status ? task : { ...task, status };
      const tasksById = nextTask === task ? state.tasksById : { ...state.tasksById, [task.id]: nextTask };
      const activeTaskId = isTerminal(status) && state.activeTaskId === task.id ? undefined : state.activeTaskId;
      return { ...state, tasksById, activeTaskId, runtimeByTaskId: { ...state.runtimeByTaskId, [task.id]: runtime } };
    }
  }
}

export function isTerminal(status: Task['status']): boolean {
  return status === 'completed' || status === 'failed';
}

function terminalStatus(type: string, current: Task['status']): Task['status'] {
  if (type === 'task.completed') return 'completed';
  if (type === 'task.failed') return 'failed';
  return current;
}

function findActiveTask(tasks: Record<string, Task>, order: string[]): string | undefined {
  return order.find((id) => !isTerminal(tasks[id].status));
}

function initializeRuntimes(current: Record<string, WorkbenchState>, taskIDs: string[]): Record<string, WorkbenchState> {
  const next = { ...current };
  for (const id of taskIDs) next[id] ??= initialWorkbenchState;
  return next;
}

function orderMessages(messages: Record<string, Message>): string[] {
  return Object.values(messages).sort((left, right) => left.createdAt.localeCompare(right.createdAt) || left.id.localeCompare(right.id)).map(({ id }) => id);
}

function orderTasks(tasks: Record<string, Task>): string[] {
  return Object.values(tasks).sort((left, right) => right.createdAt.localeCompare(left.createdAt) || right.id.localeCompare(left.id)).map(({ id }) => id);
}
