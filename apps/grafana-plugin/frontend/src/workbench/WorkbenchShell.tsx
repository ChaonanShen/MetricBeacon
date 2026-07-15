import { useTheme2 } from '@grafana/ui';
import { useRef, useState, type ReactNode } from 'react';

import { ContextPane } from './ContextPane';
import { WorkbenchHeader } from './WorkbenchHeader';
import type { WorkbenchContextView } from './workbench-view';
import { workbenchThemeCSS } from './workbench-styles';

type Props = {
  context: WorkbenchContextView;
  sessionPane: ReactNode;
  conversationPane: ReactNode;
  canvas: ReactNode;
};

export function WorkbenchShell({ context, sessionPane, conversationPane, canvas }: Props) {
  const theme = useTheme2();
  const [contextExpanded, setContextExpanded] = useState(false);
  const sessionsRef = useRef<HTMLDivElement>(null);
  const openSessions = () => {
    sessionsRef.current?.focus();
    sessionsRef.current?.scrollIntoView({ block: 'nearest' });
  };
  return <main className="mtb-workbench" data-testid="workbench-root">
    <style>{workbenchThemeCSS(theme)}</style>
    <WorkbenchHeader context={context} onOpenSessions={openSessions} />
    <div className="mtb-workbench-layout">
      <section className="mtb-workbench-chat-slot" aria-label="聊天">
        <div ref={sessionsRef} className="mtb-workbench-session-slot" tabIndex={-1}>{sessionPane}</div>
        <div className="mtb-workbench-conversation-slot">{conversationPane}</div>
      </section>
      <div className={`mtb-workbench-context-slot${contextExpanded ? ' is-expanded' : ''}`}>
        <ContextPane context={context} expanded={contextExpanded} onToggle={() => setContextExpanded((current) => !current)} />
      </div>
      <div className="mtb-workbench-canvas-slot">{canvas}</div>
    </div>
  </main>;
}
