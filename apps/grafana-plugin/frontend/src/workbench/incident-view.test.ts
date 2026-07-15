import { describe, expect, it } from 'vitest';

import type { Approval, Task } from '../api/resource';
import { deriveIncidentView } from './incident-view';
import { initialWorkbenchState } from './types';

const digest = `sha256:${'a'.repeat(64)}`;

function incidentTask(status: Task['status'] = 'waiting_approval'): Task {
  return {
    id: 'task-1', kind: 'incident_remediation', sessionId: 'session-1', status, inputMessageId: 'message-1', latestSequence: 12, error: null,
    createdAt: '2026-07-16T10:00:00Z', startedAt: '2026-07-16T10:00:01Z', completedAt: null, updatedAt: '2026-07-16T10:00:02Z', version: 5,
    incidentPlan: {
      sourceId: 'demo-grafana', alertName: 'OrderQueueBacklog', alertFingerprint: 'fingerprint-1', serviceRef: 'order-demo', labels: { service_ref: 'order-demo' },
      mapping: { id: 'mapping', digest }, playbook: { id: 'order-queue-backlog', version: '1', digest }, phase: 'needs_approval',
      assetRefs: [{ kind: 'knowledge', id: 'order-service', version: '1', digest }, { kind: 'skill', id: 'worker-diagnosis', version: '1', digest }, { kind: 'playbook', id: 'order-queue-backlog', version: '1', digest }],
      diagnosis: { primaryHypothesis: 'worker_stopped', evidenceRefs: ['order_service.get_worker_state'], alternativeHypotheses: ['slow_processing', 'dependency_errors'], confidence: 0.99, candidateAction: 'restore_worker_concurrency' },
      intent: { id: 'intent-1', digest, capabilityId: 'order_service.restore_worker_concurrency', serviceRef: 'order-demo', instanceEpoch: 'epoch-1', expectedVersion: 2, beforeConcurrency: 0, afterConcurrency: 2, risk: 'low', createdAt: '2026-07-16T10:00:02Z' },
    },
  } as Task;
}

function pendingApproval(): Approval {
  return { id: 'approval-1', taskId: 'task-1', status: 'pending', intentDigest: digest, requestedAt: '2026-07-16T10:00:02Z', expiresAt: '2026-07-16T10:10:02Z', decidedAt: null, decidedBy: null, decisionReason: null, version: 1 };
}

describe('deriveIncidentView', () => {
  it('shows exact evidence, alternatives and immutable 0 → 2 diff to an Admin', () => {
    const view = deriveIncidentView(incidentTask(), { ...initialWorkbenchState, incidentTimeline: [{ sequence: 1, type: 'alert.received', title: '告警' }] }, pendingApproval(), true)!;
    expect(view).toMatchObject({ alertName: 'OrderQueueBacklog', serviceRef: 'order-demo', hypothesis: 'worker_stopped', confidence: '99%', alternatives: ['slow_processing', 'dependency_errors'], evidenceRefs: ['order_service.get_worker_state'], canDecide: true });
    expect(view.intent).toMatchObject({ beforeConcurrency: 0, afterConcurrency: 2, expectedVersion: 2, instanceEpoch: 'epoch-1', digest });
    expect(view.timeline).toHaveLength(1);
  });

  it('keeps Viewer and terminal Incident decisions read-only', () => {
    expect(deriveIncidentView(incidentTask(), initialWorkbenchState, pendingApproval(), false)?.canDecide).toBe(false);
    expect(deriveIncidentView(incidentTask('completed'), initialWorkbenchState, { ...pendingApproval(), status: 'approved', version: 2 }, true)?.canDecide).toBe(false);
  });
});
