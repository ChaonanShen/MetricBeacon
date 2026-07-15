import type { WorkbenchContextView } from './workbench-view';

type Props = {
  context: WorkbenchContextView;
};

export function WorkbenchHeader({ context }: Props) {
  return <header className="mtb-workbench-header">
    <div>
      <p className="mtb-workbench-eyebrow">AI Metrics Workbench</p>
      <h1 className="mtb-workbench-title">指标分析工作台</h1>
    </div>
    <span className="mtb-workbench-status" role="status">{context.sessionTitle} · {context.status}</span>
  </header>;
}
