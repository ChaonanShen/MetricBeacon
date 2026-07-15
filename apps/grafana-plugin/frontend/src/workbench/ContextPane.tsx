import type { WorkbenchContextView } from './workbench-view';

type Props = {
  context: WorkbenchContextView;
  expanded: boolean;
  onToggle: () => void;
};

export function ContextPane({ context, expanded, onToggle }: Props) {
  return <aside className={`mtb-context-pane${expanded ? ' is-expanded' : ''}`} aria-label="分析上下文" data-testid="context-pane">
    <div className="mtb-context-header">
      <div className="mtb-context-heading">
        <span className="mtb-context-kicker">Context</span>
        <h2>上下文</h2>
      </div>
      <button type="button" className="mtb-context-toggle" aria-expanded={expanded} aria-controls="mtb-context-details" onClick={onToggle}>
        <span aria-hidden="true">{expanded ? '−' : '+'}</span>
        <span className="mtb-visually-hidden">{expanded ? '收起分析上下文' : '展开分析上下文'}</span>
      </button>
    </div>
    <div id="mtb-context-details" className="mtb-context-details">
      <section aria-labelledby="mtb-session-context-title">
        <h3 id="mtb-session-context-title">当前会话</h3>
        <p className="mtb-context-session-title">{context.sessionTitle}</p>
      </section>
      <dl className="mtb-context-list">
        <div><dt>Datasource</dt><dd>{context.datasource}</dd></div>
        <div><dt>时间范围</dt><dd>{context.timeRange}</dd></div>
        <div><dt>分析视图</dt><dd>{context.views}</dd></div>
        <div><dt>查询步长</dt><dd>{context.step}</dd></div>
        <div><dt>状态</dt><dd><span className={`mtb-context-badge tone-${context.statusTone}`}>{context.status}</span></dd></div>
      </dl>
      <div className="mtb-context-note">
        <strong>Grafana 资源上下文</strong>
        <p>Folder、Dashboard、Service 与权限上下文尚未接入当前实现。</p>
      </div>
    </div>
  </aside>;
}
