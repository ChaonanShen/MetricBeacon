import { useTheme2 } from '@grafana/ui';
import { useRef, useState, type ReactNode } from 'react';

import { ChatPane, type ChatControls } from './ChatPane';
import { ContextPane } from './ContextPane';
import type { SessionControls } from './SessionMenu';
import type { IncidentControls } from './SessionMenu';
import { WorkbenchHeader } from './WorkbenchHeader';
import type { WorkbenchContextView } from './workbench-view';
import { workbenchThemeCSS } from './workbench-styles';

type Props = {
  context: WorkbenchContextView;
  sessions: SessionControls;
  incidents: IncidentControls;
  chat: ChatControls;
  canvas: ReactNode;
};

export function WorkbenchShell({ context, sessions, incidents, chat, canvas }: Props) {
  const theme = useTheme2();
  const [contextExpanded, setContextExpanded] = useState(false);
  const [sessionMenuOpen, setSessionMenuOpen] = useState(false);
  const [selectorMode, setSelectorMode] = useState<'sessions' | 'incidents'>('sessions');
  const sessionMenuToggleRef = useRef<HTMLButtonElement>(null);
  const openSessions = () => {
	setSelectorMode('sessions');
    setSessionMenuOpen(true);
    requestAnimationFrame(() => {
      sessionMenuToggleRef.current?.focus();
      sessionMenuToggleRef.current?.scrollIntoView({ block: 'nearest' });
    });
  };
  const openIncidents = () => {
    setSelectorMode('incidents');
    setSessionMenuOpen(true);
    requestAnimationFrame(() => {
      sessionMenuToggleRef.current?.focus();
      sessionMenuToggleRef.current?.scrollIntoView({ block: 'nearest' });
    });
  };
  return <main className="mtb-workbench" data-testid="workbench-root">
    <style>{workbenchThemeCSS(theme)}</style>
    <WorkbenchHeader context={context} onOpenSessions={openSessions} onOpenIncidents={openIncidents} />
    <div className="mtb-workbench-layout">
      <div className="mtb-workbench-chat-slot">
        <ChatPane chat={chat} sessions={sessions} incidents={incidents} selectorMode={selectorMode} onSelectorModeChange={setSelectorMode} sessionMenuOpen={sessionMenuOpen} onSessionMenuOpenChange={setSessionMenuOpen} sessionMenuToggleRef={sessionMenuToggleRef} />
      </div>
      <div className={`mtb-workbench-context-slot${contextExpanded ? ' is-expanded' : ''}`}>
        <ContextPane context={context} expanded={contextExpanded} onToggle={() => setContextExpanded((current) => !current)} />
      </div>
      <div className="mtb-workbench-canvas-slot">{canvas}</div>
    </div>
  </main>;
}
