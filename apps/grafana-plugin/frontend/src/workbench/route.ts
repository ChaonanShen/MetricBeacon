export type WorkbenchRoute = { sessionId?: string; taskId?: string };

export function readWorkbenchRoute(search: string): WorkbenchRoute {
  const params = new URLSearchParams(search);
  const sessionId = params.get('sessionId')?.trim() || undefined;
  const taskId = params.get('taskId')?.trim() || undefined;
  return { sessionId, taskId };
}

export function replaceWorkbenchRoute(sessionId: string, taskId: string): void {
  const url = new URL(window.location.href);
  url.searchParams.set('sessionId', sessionId);
  url.searchParams.set('taskId', taskId);
  window.history.replaceState(window.history.state, '', url);
}

export function clearWorkbenchRoute(): void {
  const url = new URL(window.location.href);
  url.searchParams.delete('sessionId');
  url.searchParams.delete('taskId');
  window.history.replaceState(window.history.state, '', url);
}
