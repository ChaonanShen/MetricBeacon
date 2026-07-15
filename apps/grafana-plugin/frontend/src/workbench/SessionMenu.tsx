import { Button, Spinner } from '@grafana/ui';
import type { RefObject } from 'react';

import type { Session } from '../api/resource';

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

type Props = SessionControls & {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  toggleRef: RefObject<HTMLButtonElement>;
};

export function SessionMenu({ sessions, selectedSessionId, loading, loadingMore, hasMore, error, switchingDisabled, onNewConversation, onSelectSession, onLoadMore, open, onOpenChange, toggleRef }: Props) {
  return <section className="mtb-session-menu" aria-label="会话选择">
    <div className="mtb-session-menu-actions">
      <button ref={toggleRef} type="button" className="mtb-session-menu-toggle" data-testid="session-menu-toggle" aria-expanded={open} aria-controls="mtb-session-list" onClick={() => onOpenChange(!open)}>
        <span>会话</span>
        <span className="mtb-session-menu-current">{selectedSessionId ? '历史会话' : '新对话'}</span>
        <span aria-hidden="true">{open ? '▴' : '▾'}</span>
      </button>
      <Button variant="secondary" size="sm" onClick={onNewConversation} disabled={switchingDisabled}>新建对话</Button>
      {loading && !loadingMore && <Spinner inline size="sm" />}
    </div>
    {open && <div id="mtb-session-list" className="mtb-session-list" data-testid="session-scroll-container">
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
    </div>}
  </section>;
}
