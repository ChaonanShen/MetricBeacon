import { AppRootProps, type DataFrame, type TimeRange } from '@grafana/data';
import { Button, Card, Field, Input, Spinner, TimeSeries } from '@grafana/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useReducer, useRef, useState } from 'react';

import { resourceClient, type CreateTask } from '../api/resource';
import { ChartWireToDataFrame } from './mapper';
import { resetWorkbench, taskEventReducer } from './reducer';
import { readWorkbenchRoute, replaceWorkbenchRoute } from './route';
import { subscribeTaskEvents } from './sse';
import { chartTimeRange } from './time-range';
import { initialWorkbenchState } from './types';

export function Workbench(_props: AppRootProps) {
  const client = useQueryClient();
  const initialRoute = useMemo(() => readWorkbenchRoute(window.location.search), []);
  const [message, setMessage] = useState('');
  const [sessionId, setSessionId] = useState<string | undefined>(initialRoute.sessionId);
  const [taskId, setTaskId] = useState<string | undefined>(initialRoute.taskId);
  const [state, dispatch] = useReducer(taskEventReducer, initialWorkbenchState);
  const latestSequence = useRef(state.latestSequence);
  latestSequence.current = state.latestSequence;
  const session = useQuery({ queryKey: ['mini-torchbearing-session', sessionId], queryFn: () => resourceClient.getSession(sessionId!), enabled: Boolean(sessionId) });
  const task = useQuery({ queryKey: ['mini-torchbearing-task', taskId], queryFn: () => resourceClient.getTask(taskId!), enabled: Boolean(taskId), refetchInterval: (query) => query.state.data?.status === 'completed' || query.state.data?.status === 'failed' ? false : 1_000 });
  const create = useMutation({
    mutationFn: async () => {
      const activeSession = sessionId ? { id: sessionId } : await resourceClient.createSession('Node exporter mock analysis');
      const body: CreateTask = { sessionId: activeSession.id, message: message.trim(), analysisContext: { datasourceUid: 'mock-prometheus', timeRange: { relativeDuration: '30m' } } };
      const createdTask = await resourceClient.createTask(body, crypto.randomUUID());
      return { sessionId: activeSession.id, task: createdTask };
    },
    onSuccess: (created) => {
      dispatch(resetWorkbench());
      setSessionId(created.sessionId);
      setTaskId(created.task.id);
      replaceWorkbenchRoute(created.sessionId, created.task.id);
      client.setQueryData(['mini-torchbearing-task', created.task.id], created.task);
    },
  });
  useEffect(() => {
    if (!taskId) { return; }
    return subscribeTaskEvents((after) => resourceClient.eventURL(taskId, after), () => latestSequence.current, (event) => { dispatch(event); client.invalidateQueries({ queryKey: ['mini-torchbearing-task', taskId] }); }, () => undefined).close;
  }, [client, taskId]);
  const charts = useMemo(() => Object.values(state.charts), [state.charts]);
  const submit = () => { if (message.trim()) { create.mutate(); } };
  return <main style={{ padding: 16 }}>
    <h2>Mini Torchbearing Workbench</h2>
    <Field label="分析请求">
      <Input value={message} onChange={(event) => setMessage(event.currentTarget.value)} placeholder="例如：查看 node exporter" />
    </Field>
    <Button onClick={submit} disabled={!message.trim() || create.isPending}>开始分析</Button>
    {(create.isPending || session.isFetching || task.isFetching) && <Spinner />}
    <p>Task 状态：{state.taskStatus ?? task.data?.status ?? 'idle'}</p>
    {state.error && <p role="alert">{state.error.code}: {state.error.message}</p>}
    {state.assistantText && <section><h3>助手</h3><p>{state.assistantText}</p></section>}
    <section style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: 12 }}>
      {charts.map(({ chart, execution }) => <ChartCard key={chart.id} title={chart.title} status={String(chart.status)} expression={chart.queries?.[0]?.expression} frame={execution ? ChartWireToDataFrame(execution.series) : undefined} timeRange={chartTimeRange(chart, execution)} />)}
    </section>
  </main>;
}

function ChartCard({ title, status, expression, frame, timeRange }: { title: string; status: string; expression?: string; frame?: DataFrame; timeRange?: TimeRange }) {
  return <Card heading={title}><p>状态：{status}</p>{expression && <details><summary>PromQL</summary><code>{expression}</code></details>}{frame && timeRange && <TimeSeries width={420} height={220} timeRange={timeRange} timeZone="browser" frames={[frame]} legend={{ showLegend: true, placement: 'bottom', calcs: [] }} />}</Card>;
}
