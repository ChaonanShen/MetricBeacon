import { describe, expect, it, vi } from 'vitest';

import { readWorkbenchRoute, replaceWorkbenchRoute } from './route';

describe('workbench route', () => {
  it('reads only non-empty session and task identifiers', () => {
    expect(readWorkbenchRoute('?sessionId=%20session-1%20&taskId=task-1')).toEqual({ sessionId: 'session-1', taskId: 'task-1' });
    expect(readWorkbenchRoute('?sessionId=&taskId=%20')).toEqual({});
  });

  it('replaces the current history entry while preserving other query parameters', () => {
    const replaceState = vi.fn();
    vi.stubGlobal('window', {
      location: { href: 'http://grafana.local/a/mini-torchbearing-app/workbench?theme=dark' },
      history: { state: { from: 'grafana' }, replaceState },
    });

    replaceWorkbenchRoute('session-1', 'task-1');

    const [, , next] = replaceState.mock.calls[0];
    expect(next.toString()).toBe('http://grafana.local/a/mini-torchbearing-app/workbench?theme=dark&sessionId=session-1&taskId=task-1');
    expect(replaceState).toHaveBeenCalledWith({ from: 'grafana' }, '', next);
  });
});
