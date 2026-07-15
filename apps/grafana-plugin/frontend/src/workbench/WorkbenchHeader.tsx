import type { WorkbenchContextView } from './workbench-view';
import { productNavigation } from './workbench-view';

type Props = {
  context: WorkbenchContextView;
  onOpenSessions: () => void;
  onOpenIncidents: () => void;
};

export function WorkbenchHeader({ context, onOpenSessions, onOpenIncidents }: Props) {
  return <header className="mtb-workbench-header">
    <div className="mtb-workbench-heading">
      <p className="mtb-workbench-eyebrow">AI Metrics Workbench</p>
      <h1 className="mtb-workbench-title">指标分析工作台</h1>
    </div>
    <nav className="mtb-product-nav" aria-label="产品功能">
      {productNavigation.map((item) => {
        if (item.id === 'workbench') {
          return <button key={item.id} type="button" className="mtb-product-nav-item is-current" aria-current="page">{item.label}</button>;
        }
        if (item.id === 'sessions') {
          return <button key={item.id} type="button" className="mtb-product-nav-item" onClick={onOpenSessions}>{item.label}</button>;
        }
        if (item.id === 'incidents') {
          return <button key={item.id} type="button" className="mtb-product-nav-item" onClick={onOpenIncidents}>{item.label}</button>;
        }
        return <button key={item.id} type="button" className="mtb-product-nav-item" disabled title="尚未开放">{item.label}</button>;
      })}
    </nav>
    <span className={`mtb-workbench-status tone-${context.statusTone}`} role="status">{context.sessionTitle} · {context.status}</span>
  </header>;
}
