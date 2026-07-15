import { useTheme2 } from '@grafana/ui';
import type { ReactNode } from 'react';

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
  return <main className="mtb-workbench" data-testid="workbench-root">
    <style>{workbenchThemeCSS(theme)}</style>
    <WorkbenchHeader context={context} />
    <div className="mtb-workbench-layout">
      <div className="mtb-workbench-session-slot">{sessionPane}</div>
      <div className="mtb-workbench-conversation-slot">{conversationPane}</div>
      <div className="mtb-workbench-canvas-slot">{canvas}</div>
    </div>
  </main>;
}
