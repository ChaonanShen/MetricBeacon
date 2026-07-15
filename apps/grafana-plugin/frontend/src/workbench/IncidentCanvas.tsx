import { Button, Spinner } from '@grafana/ui';

import type { IncidentView } from './incident-view';

type Props = {
  incident?: IncidentView;
  loading: boolean;
  deciding: boolean;
  error?: string;
  onApprove: () => void;
  onReject: () => void;
};

export function IncidentCanvas({ incident, loading, deciding, error, onApprove, onReject }: Props) {
  if (!incident) return <section className="mtb-canvas-pane"><div className="mtb-canvas-empty"><strong>选择一个组织事件</strong><p>从“事件”列表打开 Incident 后，可查看诊断证据、受控 Diff、审批和恢复验证时间线。</p></div></section>;
  return <section className="mtb-canvas-pane mtb-incident-canvas" aria-label="事件处置" data-testid="incident-canvas">
    <header className="mtb-canvas-header"><div><span className="mtb-pane-kicker">Incident</span><h2>{incident.alertName}</h2></div><span className="mtb-context-badge tone-warning">{incident.status}</span></header>
    <div className="mtb-incident-scroll">
      {error && <p role="alert" className="mtb-inline-error">{error}</p>}
      <section className="mtb-incident-summary"><h3>业务影响与诊断</h3><dl className="mtb-incident-grid"><div><dt>服务</dt><dd>{incident.serviceRef}</dd></div><div><dt>阶段</dt><dd>{incident.phase}</dd></div><div><dt>主假设</dt><dd>{incident.hypothesis}</dd></div><div><dt>置信度</dt><dd>{incident.confidence}</dd></div></dl>
        {incident.alternatives.length > 0 && <p className="mtb-muted">已排查的相似原因：{incident.alternatives.join('、')}</p>}
      </section>
      <section className="mtb-incident-summary"><h3>只读证据与固定资产</h3><ul>{incident.evidenceRefs.map((value) => <li key={value}><code>{value}</code></li>)}</ul><details><summary>固定版本 Knowledge / Skill / Playbook</summary><ul>{incident.assets.map((asset) => <li key={`${asset.kind}:${asset.id}`}><strong>{asset.kind}</strong> · {asset.id}@{asset.version} · <code>{asset.digest.slice(0, 19)}…</code></li>)}</ul></details></section>
      <section className="mtb-incident-summary"><h3>Intent / Diff</h3>{incident.intent ? <><div className="mtb-intent-diff"><span>worker concurrency</span><strong>{incident.intent.beforeConcurrency}</strong><span aria-hidden="true">→</span><strong>{incident.intent.afterConcurrency}</strong></div><dl className="mtb-incident-grid"><div><dt>目标 epoch</dt><dd><code>{incident.intent.instanceEpoch}</code></dd></div><div><dt>期望版本</dt><dd>{incident.intent.expectedVersion}</dd></div><div><dt>能力</dt><dd><code>{incident.intent.capabilityId}</code></dd></div><div><dt>风险</dt><dd>{incident.intent.risk}</dd></div></dl><p className="mtb-digest">Intent digest: <code>{incident.intent.digest}</code></p></> : <p className="mtb-muted">当前诊断未生成写操作。</p>}</section>
      <section className="mtb-incident-summary"><h3>人工审批</h3>{loading ? <Spinner inline size="sm" /> : incident.approval ? <><p>状态：<strong>{incident.approval.status}</strong> · 版本 {incident.approval.version}</p><p className="mtb-muted">有效期至 {new Date(incident.approval.expiresAt).toLocaleString()}</p>{incident.canDecide ? <div className="mtb-approval-actions"><Button onClick={onApprove} disabled={deciding}>批准并执行</Button><Button variant="destructive" onClick={onReject} disabled={deciding}>拒绝</Button>{deciding && <Spinner inline size="sm" />}</div> : incident.approval.status === 'pending' ? <p className="mtb-muted">仅 Grafana Admin 可以批准或拒绝；等待人工决定。</p> : null}</> : <p className="mtb-muted">尚未进入审批阶段。</p>}</section>
      <section className="mtb-incident-summary"><h3>可恢复执行时间线</h3>{incident.timeline.length === 0 ? <p className="mtb-muted">等待事件回放。</p> : <ol className="mtb-incident-timeline">{incident.timeline.map((item) => <li key={item.sequence}><span>{item.sequence}</span><div><strong>{item.title}</strong>{item.detail && <p>{item.detail}</p>}</div></li>)}</ol>}</section>
    </div>
  </section>;
}
