import { Button, Spinner } from '@grafana/ui';
import type { RefObject } from 'react';

import type { Session, Task } from '../api/resource';

export type SessionControls = {
  sessions: Session[];
  selectedSessionId?: string;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  error?: string;
  switchingDisabled: boolean;
  onNewConversation: () => void;
  onSelectSession: (sessionId: string) => void;
  onLoadMore: () => void;
};

export type IncidentControls = {
  incidents: Task[];
  selectedTaskId?: string;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  error?: string;
  switchingDisabled: boolean;
  onSelectIncident: (task: Task) => void;
  onLoadMore: () => void;
};

type Props = SessionControls & {
	incidents: IncidentControls;
	mode: 'sessions' | 'incidents';
	onModeChange: (mode: 'sessions' | 'incidents') => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  toggleRef: RefObject<HTMLButtonElement>;
};

export function SessionMenu({ sessions, selectedSessionId, loading, loadingMore, hasMore, error, switchingDisabled, onNewConversation, onSelectSession, onLoadMore, incidents, mode, onModeChange, open, onOpenChange, toggleRef }: Props) {
  const showingIncidents = mode === 'incidents';
  return <section className="mtb-session-menu" aria-label="会话选择">
    <div className="mtb-session-menu-actions">
      <button ref={toggleRef} type="button" className="mtb-session-menu-toggle" data-testid="session-menu-toggle" aria-expanded={open} aria-controls="mtb-session-list" onClick={() => onOpenChange(!open)}>
        <span>{showingIncidents ? '事件' : '会话'}</span>
        <span className="mtb-session-menu-current">{showingIncidents ? (incidents.selectedTaskId ? '当前组织事件' : '组织事件') : (selectedSessionId ? '历史会话' : '新对话')}</span>
        <span aria-hidden="true">{open ? '▴' : '▾'}</span>
      </button>
      {!showingIncidents && <Button variant="secondary" size="sm" onClick={onNewConversation} disabled={switchingDisabled}>新建对话</Button>}
      {(showingIncidents ? incidents.loading && !incidents.loadingMore : loading && !loadingMore) && <Spinner inline size="sm" />}
    </div>
    {open && <div id="mtb-session-list" className="mtb-session-list" data-testid="session-scroll-container">
      <div className="mtb-selector-tabs"><button type="button" className={!showingIncidents ? 'is-current' : ''} onClick={() => onModeChange('sessions')}>个人会话</button><button type="button" className={showingIncidents ? 'is-current' : ''} onClick={() => onModeChange('incidents')}>组织事件</button></div>
      {showingIncidents ? <>
        {incidents.error && <p role="alert" className="mtb-inline-error">{incidents.error}</p>}
        {incidents.incidents.length === 0 && !incidents.loading ? <p className="mtb-muted">暂无组织事件</p> : incidents.incidents.map((task) => <button type="button" key={task.id} className={`mtb-session-item${task.id === incidents.selectedTaskId ? ' is-selected' : ''}`} aria-current={task.id === incidents.selectedTaskId ? 'page' : undefined} disabled={incidents.switchingDisabled} onClick={() => incidents.onSelectIncident(task)}><span className="mtb-session-title">{task.incidentPlan?.alertName ?? 'Incident'} · {task.incidentPlan?.serviceRef ?? 'unknown service'}</span><span className="mtb-session-time">{task.status} · {new Date(task.updatedAt).toLocaleString()}</span></button>)}
        {incidents.hasMore && <Button variant="secondary" size="sm" onClick={incidents.onLoadMore} disabled={incidents.loadingMore}>{incidents.loadingMore ? '加载中…' : '加载更多事件'}</Button>}
      </> : <>
      {error && <p role="alert" className="mtb-inline-error">{error}</p>}
      {sessions.length === 0 && !loading
        ? <p className="mtb-muted">暂无历史会话</p>
        : sessions.map((session) => <button
          type="button"
          key={session.id}
          className={`mtb-session-item${session.id === selectedSessionId ? ' is-selected' : ''}`}
          aria-current={session.id === selectedSessionId ? 'page' : undefined}
          disabled={switchingDisabled}
          onClick={() => onSelectSession(session.id)}
        >
          <span className="mtb-session-title" title={session.title}>{session.title}</span>
          <span className="mtb-session-time">{new Date(session.updatedAt).toLocaleString()}</span>
        </button>)}
      {hasMore && <Button variant="secondary" size="sm" onClick={onLoadMore} disabled={loadingMore}>{loadingMore ? '加载中…' : '加载更多'}</Button>}
      </>}
    </div>}
  </section>;
}
