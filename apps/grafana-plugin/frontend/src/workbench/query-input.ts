import type { CreateTask } from '../api/resource';

export function createTaskInput(sessionId: string, message: string): CreateTask {
  return {
    sessionId,
    message: message.trim(),
    analysisContext: { datasourceUid: 'prometheus-main' },
  };
}
