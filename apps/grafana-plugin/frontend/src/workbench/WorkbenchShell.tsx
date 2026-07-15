import { useTheme2 } from '@grafana/ui';
import { useRef, useState, type ReactNode } from 'react';

import { ChatPane, type ChatControls } from './ChatPane';
import { ContextPane } from './ContextPane';
import type { SessionControls } from './SessionMenu';
import { WorkbenchHeader } from './WorkbenchHeader';
import type { WorkbenchContextView } from './workbench-view';
import { workbenchThemeCSS } from './workbench-styles';

type Props = {
  context: WorkbenchContextView;
  sessions: SessionControls;
  chat: ChatControls;
  canvas: ReactNode;
};

export function WorkbenchShell({ context, sessions, chat, canvas }: Props) {
  const theme = useTheme2();
  const [contextExpanded, setContextExpanded] = useState(false);
  const [sessionMenuOpen, setSessionMenuOpen] = useState(false);
  const sessionMenuToggleRef = useRef<HTMLButtonElement>(null);
  const openSessions = () => {
    setSessionMenuOpen(true);
    requestAnimationFrame(() => {
      sessionMenuToggleRef.current?.focus();
      sessionMenuToggleRef.current?.scrollIntoView({ block: 'nearest' });
    });
  };
  return <main className="mtb-workbench" data-testid="workbench-root">
    <style>{workbenchThemeCSS(theme)}</style>
    <WorkbenchHeader context={context} onOpenSessions={openSessions} />
    <div className="mtb-workbench-layout">
      <div className="mtb-workbench-chat-slot">
        <ChatPane chat={chat} sessions={sessions} sessionMenuOpen={sessionMenuOpen} onSessionMenuOpenChange={setSessionMenuOpen} sessionMenuToggleRef={sessionMenuToggleRef} />
      </div>
      <div className={`mtb-workbench-context-slot${contextExpanded ? ' is-expanded' : ''}`}>
        <ContextPane context={context} expanded={contextExpanded} onToggle={() => setContextExpanded((current) => !current)} />
      </div>
      <div className="mtb-workbench-canvas-slot">{canvas}</div>
    </div>
  </main>;
}
