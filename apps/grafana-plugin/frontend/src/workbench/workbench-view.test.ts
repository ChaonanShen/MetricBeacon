import { describe, expect, it } from 'vitest';

import type { Task } from '../api/resource';
import { deriveWorkbenchContext, examplePrompts, productNavigation } from './workbench-view';

describe('deriveWorkbenchContext', () => {
  it('returns truthful fresh-conversation fallbacks without inventing task context', () => {
    expect(deriveWorkbenchContext(undefined, [])).toEqual({
      sessionTitle: '新对话',
      datasource: 'Prometheus',
      timeRange: '提交后确定',
      views: '提交后确定',
      step: '提交后确定',
      status: '等待分析',
      statusTone: 'neutral',
      hasTask: false,
    });
  });

  it('prefers the active task over a newer completed task', () => {
    const active = task({ id: 'active', status: 'running_tools', createdAt: '2026-07-15T01:00:00Z', views: ['cpu'] });
    const newer = task({ id: 'newer', status: 'completed', createdAt: '2026-07-15T02:00:00Z', views: ['memory'] });

    expect(deriveWorkbenchContext('CPU 会话', [newer, active], active)).toMatchObject({
      sessionTitle: 'CPU 会话',
      views: 'CPU',
      status: '查询中',
      statusTone: 'warning',
      hasTask: true,
    });
  });

  it('uses the newest task and formats durable query-plan values', () => {
    const older = task({ id: 'older', createdAt: '2026-07-15T01:00:00Z' });
    const newer = task({ id: 'newer', createdAt: '2026-07-15T02:00:00Z', status: 'completed', views: ['cpu', 'memory', 'load'] });

    expect(deriveWorkbenchContext('  ', [older, newer])).toEqual({
      sessionTitle: '未命名会话',
      datasource: 'Prometheus',
      timeRange: '2026-07-15 00:00:00 UTC — 2026-07-15 00:30:00 UTC',
      views: 'CPU、内存、系统负载',
      step: '30 秒',
      status: '已完成',
      statusTone: 'success',
      hasTask: true,
    });
  });

  it('uses stable fallbacks for malformed historical timestamps and empty views', () => {
    const malformed = task({ id: 'bad', views: [] });
    malformed.timeRange = { from: 'invalid', to: 'invalid' };

    expect(deriveWorkbenchContext('历史', [malformed])).toMatchObject({ timeRange: '—', views: '—' });
  });
});

describe('examplePrompts', () => {
  it('only describes the three currently registered metric views', () => {
    expect(examplePrompts).toEqual([
      '查看最近 30 分钟 CPU 使用率',
      '分析最近 30 分钟内存可用率',
      '查看最近 30 分钟系统负载',
    ]);
  });
});

describe('productNavigation', () => {
  it('only enables capabilities backed by the current workbench', () => {
    expect(productNavigation.filter((item) => item.enabled).map((item) => item.id)).toEqual(['workbench', 'sessions', 'incidents']);
    expect(productNavigation.filter((item) => !item.enabled).map((item) => item.id)).toEqual(['knowledge', 'playbook', 'skill', 'promotion']);
  });
});

function task(overrides: { id: string; status?: Task['status']; createdAt?: string; views?: NonNullable<Task['queryPlan']>['views'] }): Task {
  return {
    id: overrides.id,
    kind: 'metric_analysis',
    sessionId: 'session-1',
    status: overrides.status ?? 'planning',
    inputMessageId: 'message-1',
    datasourceUid: 'prometheus-main',
    timeRange: { from: '2026-07-15T00:00:00Z', to: '2026-07-15T00:30:00Z' },
    queryPlan: { views: overrides.views ?? ['cpu'], stepSeconds: 30, cpuRateWindowSeconds: 60 },
    latestSequence: 0,
    error: null,
    createdAt: overrides.createdAt ?? '2026-07-15T00:00:00Z',
    startedAt: null,
    completedAt: null,
    updatedAt: overrides.createdAt ?? '2026-07-15T00:00:00Z',
    version: 1,
  };
}
