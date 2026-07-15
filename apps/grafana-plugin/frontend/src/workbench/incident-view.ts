import type { Approval, Task } from '../api/resource';
import type { IncidentTimelineItem, WorkbenchState } from './types';

export type IncidentView = {
  taskId: string;
  alertName: string;
  serviceRef: string;
  phase: string;
  status: Task['status'];
  hypothesis: string;
  confidence: string;
  alternatives: string[];
  evidenceRefs: string[];
  assets: Array<{ kind: string; id: string; version: string; digest: string }>;
  intent?: {
    digest: string;
    capabilityId: string;
    instanceEpoch: string;
    expectedVersion: number;
    beforeConcurrency: number;
    afterConcurrency: number;
    risk: string;
  };
  approval?: Approval;
  timeline: IncidentTimelineItem[];
  canDecide: boolean;
};

export function deriveIncidentView(task: Task, runtime: WorkbenchState | undefined, approval: Approval | undefined, isAdmin: boolean): IncidentView | undefined {
  const plan = task.incidentPlan;
  if (task.kind !== 'incident_remediation' || !plan) return undefined;
  const intent = plan.intent;
  return {
    taskId: task.id,
    alertName: plan.alertName,
    serviceRef: plan.serviceRef,
    phase: plan.phase,
    status: runtime?.taskStatus ?? task.status,
    hypothesis: plan.diagnosis?.primaryHypothesis ?? '等待只读诊断',
    confidence: plan.diagnosis ? `${Math.round(plan.diagnosis.confidence * 100)}%` : '—',
    alternatives: plan.diagnosis?.alternativeHypotheses ?? [],
    evidenceRefs: plan.diagnosis?.evidenceRefs ?? [],
    assets: plan.assetRefs.map(({ kind, id, version, digest }) => ({ kind, id, version, digest })),
    intent: intent ? { digest: intent.digest, capabilityId: intent.capabilityId, instanceEpoch: intent.instanceEpoch, expectedVersion: intent.expectedVersion, beforeConcurrency: intent.beforeConcurrency, afterConcurrency: intent.afterConcurrency, risk: intent.risk } : undefined,
    approval,
    timeline: runtime?.incidentTimeline ?? [],
    canDecide: isAdmin && task.status === 'waiting_approval' && approval?.status === 'pending' && Boolean(intent),
  };
}
