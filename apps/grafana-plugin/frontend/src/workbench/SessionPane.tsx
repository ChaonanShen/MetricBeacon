import { Box, Button, Spinner, Stack, Text } from '@grafana/ui';

import type { Session } from '../api/resource';
import { WorkbenchPane } from './WorkbenchPane';

type Props = {
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

export function SessionPane({ sessions, selectedSessionId, loading, loadingMore, hasMore, error, switchingDisabled, onNewConversation, onSelectSession, onLoadMore }: Props) {
  return <WorkbenchPane aria-label="会话" data-testid="session-pane" height="100%">
    <style>{sessionPaneCSS}</style>
    <Stack direction="column" gap={2} height="100%" minHeight={0}>
      <Box padding={3} paddingBottom={0}>
        <Stack direction="column" gap={2}>
          <Box display="flex" justifyContent="space-between" alignItems="center" gap={2}>
            <Text element="h2" variant="h4">会话</Text>
            {loading && !loadingMore && <Spinner inline size="sm" />}
          </Box>
          <Button variant="primary" size="sm" onClick={onNewConversation} disabled={switchingDisabled}>新建对话</Button>
        </Stack>
      </Box>
      {error && <Box paddingX={3}><Text role="alert" color="error">{error}</Text></Box>}
      <div className="mtb-session-scroll" data-testid="session-scroll-container">
        {sessions.length === 0 && !loading
          ? <Text color="secondary">暂无历史会话</Text>
          : <Stack direction="column" gap={1}>
            {sessions.map((session) => <button
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
          </Stack>}
        {hasMore && <Box marginTop={2}><Button variant="secondary" size="sm" onClick={onLoadMore} disabled={loadingMore}>{loadingMore ? '加载中…' : '加载更多'}</Button></Box>}
      </div>
    </Stack>
  </WorkbenchPane>;
}

const sessionPaneCSS = `
.mtb-session-scroll { flex: 1 1 auto; min-height: 0; overflow-y: auto; overflow-x: hidden; padding: 0 24px 24px; scrollbar-gutter: stable; }
.mtb-session-item { display: flex; flex-direction: column; gap: 4px; width: 100%; min-width: 0; padding: 10px 12px; border: 1px solid transparent; border-radius: 6px; background: transparent; color: inherit; text-align: left; cursor: pointer; }
.mtb-session-item:hover:not(:disabled) { background: rgba(128, 128, 128, 0.10); }
.mtb-session-item.is-selected { border-color: rgba(128, 128, 128, 0.48); background: rgba(128, 128, 128, 0.16); }
.mtb-session-item:disabled { cursor: not-allowed; opacity: 0.65; }
.mtb-session-title { display: -webkit-box; min-width: 0; overflow: hidden; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow-wrap: anywhere; font-weight: 500; }
.mtb-session-time { overflow: hidden; color: currentColor; font-size: 12px; opacity: 0.68; text-overflow: ellipsis; white-space: nowrap; }
`;
