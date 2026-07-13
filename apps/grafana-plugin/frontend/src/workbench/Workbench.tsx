import { AppRootProps, dateTime, type DataFrame, type TimeRange } from '@grafana/data';
import { Button, Card, Field, Input, Spinner, TimeSeries } from '@grafana/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useReducer, useState } from 'react';

import { resourceClient, type CreateTask } from '../api/resource';
import { ChartWireToDataFrame } from './mapper';
import { resetWorkbench, taskEventReducer } from './reducer';
import { subscribeTaskEvents } from './sse';
import { initialWorkbenchState } from './types';

const chartRange: TimeRange = { from: dateTime(Date.now() - 30 * 60 * 1000), to: dateTime(), raw: { from: 'now-30m', to: 'now' } };

export function Workbench(_props: AppRootProps) {
  const client = useQueryClient();
  const [message, setMessage] = useState('');
  const [sessionId, setSessionId] = useState<string>();
  const [taskId, setTaskId] = useState<string>();
  const [state, dispatch] = useReducer(taskEventReducer, initialWorkbenchState);
  const task = useQuery({ queryKey: ['mini-torchbearing-task', taskId], queryFn: () => resourceClient.getTask(taskId!), enabled: Boolean(taskId), refetchInterval: (query) => query.state.data?.status === 'completed' || query.state.data?.status === 'failed' ? false : 1_000 });
  const create = useMutation({
    mutationFn: async () => {
      const session = sessionId ? { id: sessionId } : await resourceClient.createSession('Node exporter overview');
      setSessionId(session.id);
      const body: CreateTask = { sessionId: session.id, message: message.trim(), analysisContext: { datasourceUid: 'mock-prometheus', timeRange: { relativeDuration: '30m' } } };
      return resourceClient.createTask(body, crypto.randomUUID());
    },
    onSuccess: (created) => { dispatch(resetWorkbench()); setTaskId(created.id); client.setQueryData(['mini-torchbearing-task', created.id], created); },
  });
  useEffect(() => {
    if (!taskId) { return; }
    return subscribeTaskEvents((after) => resourceClient.eventURL(taskId, after), () => state.latestSequence, (event) => { dispatch(event); client.invalidateQueries({ queryKey: ['mini-torchbearing-task', taskId] }); }, () => undefined).close;
  }, [client, state.latestSequence, taskId]);
  const charts = useMemo(() => Object.values(state.charts), [state.charts]);
  const submit = () => { if (message.trim()) { create.mutate(); } };
  return <main style={{ padding: 16 }}>
    <h2>Mini Torchbearing Workbench</h2>
    <Field label="分析请求">
      <Input value={message} onChange={(event) => setMessage(event.currentTarget.value)} placeholder="例如：查看 node exporter" />
    </Field>
    <Button onClick={submit} disabled={!message.trim() || create.isPending}>开始分析</Button>
    {(create.isPending || task.isFetching) && <Spinner />}
    <p>Task 状态：{state.taskStatus ?? task.data?.status ?? 'idle'}</p>
    {state.error && <p role="alert">{state.error.code}: {state.error.message}</p>}
    {state.assistantText && <section><h3>助手</h3><p>{state.assistantText}</p></section>}
    <section style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: 12 }}>
      {charts.map(({ chart, execution }) => <ChartCard key={chart.id} title={chart.title} status={String(chart.status)} expression={chart.queries?.[0]?.expression} frame={execution ? ChartWireToDataFrame(execution.series) : undefined} />)}
    </section>
  </main>;
}

function ChartCard({ title, status, expression, frame }: { title: string; status: string; expression?: string; frame?: DataFrame }) {
  return <Card heading={title}><p>状态：{status}</p>{expression && <details><summary>PromQL</summary><code>{expression}</code></details>}{frame && <TimeSeries width={420} height={220} timeRange={chartRange} timeZone="browser" frames={[frame]} legend={{ showLegend: true, placement: 'bottom', calcs: [] }} />}</Card>;
}
