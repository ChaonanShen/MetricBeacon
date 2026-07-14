import { AppRootProps } from '@grafana/data';
import { Button, Field, Grid, Input, Spinner } from '@grafana/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useReducer, useRef, useState } from 'react';

import { resourceClient, type CreateTask, type GeneratedTaskEvent, type Task } from '../api/resource';
import { ChartCard } from './ChartCard';
import { readWorkbenchRoute, replaceWorkbenchRoute } from './route';
import { initialSessionWorkbenchState, isTerminal, sessionReducer } from './session-reducer';
import { subscribeTaskEvents } from './sse';

type Event = Omit<GeneratedTaskEvent, 'payload'> & { payload: Record<string, unknown> };

export function Workbench(_props: AppRootProps) {
  const client = useQueryClient();
  const initialRoute = useMemo(() => readWorkbenchRoute(window.location.search), []);
  const [message, setMessage] = useState('');
  const [sessionId, setSessionId] = useState<string | undefined>(initialRoute.sessionId);
  const [state, dispatch] = useReducer(sessionReducer, initialSessionWorkbenchState);
  const sequenceByTask = useRef<Record<string, number>>({});
  const replayedTasks = useRef(new Set<string>());
  const idempotencyKey = useRef<string>();
  const pendingTask = useRef<CreateTask>();
  const session = useQuery({ queryKey: ['mini-torchbearing-session', sessionId], queryFn: () => resourceClient.getSession(sessionId!), enabled: Boolean(sessionId) });
  const history = useQuery({
    queryKey: ['mini-torchbearing-session-history', sessionId],
    queryFn: async () => Promise.all([resourceClient.listMessages(sessionId!), resourceClient.listTasks(sessionId!)]),
    enabled: Boolean(sessionId),
  });

  useEffect(() => {
    if (!history.data) return;
    const [messages, tasks] = history.data;
    dispatch({ type: 'history.loaded', messages: messages.items, tasks: tasks.items, messageNextPageToken: messages.nextPageToken, taskNextPageToken: tasks.nextPageToken });
  }, [history.data]);

  useEffect(() => {
    let cancelled = false;
    const replay = async (task: Task) => {
      if (replayedTasks.current.has(task.id)) return;
      let pageToken: string | undefined;
      let targetSequence = 0;
      do {
        const page = await resourceClient.replayEvents(task.id, pageToken);
        targetSequence = page.targetSequence;
        for (const event of page.items as Event[]) {
          sequenceByTask.current[task.id] = event.sequence;
          dispatch({ type: 'task.event', event });
        }
        pageToken = page.nextPageToken ?? undefined;
      } while (!cancelled && pageToken);
      if (!cancelled) {
        sequenceByTask.current[task.id] = targetSequence;
        replayedTasks.current.add(task.id);
        dispatch({ type: 'task.replayed', taskId: task.id, targetSequence });
      }
    };
    void Promise.all(state.taskOrder.map((id) => replay(state.tasksById[id])));
    return () => { cancelled = true; };
  }, [state.taskOrder, state.tasksById]);

  const activeTask = state.activeTaskId ? state.tasksById[state.activeTaskId] : undefined;
  useEffect(() => {
    if (!activeTask || !replayedTasks.current.has(activeTask.id) || isTerminal(activeTask.status)) return;
    return subscribeTaskEvents(
      (after) => resourceClient.eventURL(activeTask.id, after),
      () => sequenceByTask.current[activeTask.id] ?? 0,
      (event) => {
        sequenceByTask.current[activeTask.id] = event.sequence;
        dispatch({ type: 'task.event', event });
        client.invalidateQueries({ queryKey: ['mini-torchbearing-session-history', sessionId] });
      },
      () => undefined,
      (event) => event.type === 'task.completed' || event.type === 'task.failed',
    ).close;
  }, [activeTask, client, sessionId]);

  const create = useMutation({
    mutationFn: async () => {
      if (!pendingTask.current) {
        const activeSession = sessionId ? { id: sessionId } : await resourceClient.createSession('Node exporter mock analysis');
        pendingTask.current = { sessionId: activeSession.id, message: message.trim(), analysisContext: { datasourceUid: 'mock-prometheus', timeRange: { relativeDuration: '30m' } } };
        if (!sessionId) setSessionId(activeSession.id);
      }
      idempotencyKey.current ??= crypto.randomUUID();
      const task = await resourceClient.createTask(pendingTask.current, idempotencyKey.current);
      return { sessionId: pendingTask.current.sessionId, task };
    },
    onSuccess: (created) => {
      idempotencyKey.current = undefined;
      pendingTask.current = undefined;
      setSessionId(created.sessionId);
      dispatch({ type: 'task.created', task: created.task });
      replaceWorkbenchRoute(created.sessionId, created.task.id);
      client.setQueryData(['mini-torchbearing-session', created.sessionId], session.data);
      client.invalidateQueries({ queryKey: ['mini-torchbearing-session-history', created.sessionId] });
    },
  });
  const loadMore = useMutation({
    mutationFn: async () => {
      const [messages, tasks] = await Promise.all([
        state.messageNextPageToken ? resourceClient.listMessages(sessionId!, state.messageNextPageToken) : Promise.resolve({ items: [], nextPageToken: state.messageNextPageToken }),
        state.taskNextPageToken ? resourceClient.listTasks(sessionId!, state.taskNextPageToken) : Promise.resolve({ items: [], nextPageToken: state.taskNextPageToken }),
      ]);
      return { messages, tasks };
    },
    onSuccess: ({ messages, tasks }) => dispatch({ type: 'history.loaded', messages: messages.items, tasks: tasks.items, messageNextPageToken: messages.nextPageToken, taskNextPageToken: tasks.nextPageToken }),
  });
  const submit = () => { if (message.trim() && !activeTask) create.mutate(); };
  const messages = state.messageOrder.map((id) => state.messagesById[id]);
  const charts = state.taskOrder.flatMap((taskID) => Object.values(state.runtimeByTaskId[taskID]?.charts ?? {}).map((chart) => ({ taskID, ...chart })));

  return <main style={{ padding: 16 }}>
    <h2>Mini Torchbearing Workbench</h2>
    <section aria-label="对话历史">
      {messages.map((item) => <p key={item.id}><strong>{item.role === 'user' ? '你' : '助手'}：</strong>{item.content}</p>)}
      {state.taskOrder.map((id) => {
        const runtime = state.runtimeByTaskId[id];
        return runtime?.assistantText && !messages.some((item) => item.taskId === id && item.role === 'assistant') ? <p key={`${id}-draft`}><strong>助手：</strong>{runtime.assistantText}</p> : null;
      })}
    </section>
    <Field label="分析请求">
      <Input value={message} onChange={(event) => setMessage(event.currentTarget.value)} placeholder="例如：查看 node exporter" />
    </Field>
    <Button onClick={submit} disabled={!message.trim() || create.isPending || Boolean(activeTask)}>开始分析</Button>
    {(state.messageNextPageToken || state.taskNextPageToken) && <Button variant="secondary" onClick={() => loadMore.mutate()} disabled={loadMore.isPending}>加载更早记录</Button>}
    {(create.isPending || session.isFetching || history.isFetching) && <Spinner />}
    {activeTask && <p>Task 状态：{state.runtimeByTaskId[activeTask.id]?.taskStatus ?? activeTask.status}</p>}
    {state.taskOrder.map((id) => state.runtimeByTaskId[id]?.error && <p role="alert" key={`${id}-error`}>{state.runtimeByTaskId[id].error!.code}: {state.runtimeByTaskId[id].error!.message}</p>)}
    <section aria-label="分析图表">
      <Grid minColumnWidth={44} gap={2} alignItems="stretch">
        {charts.map(({ taskID, chart, execution }) => <ChartCard key={`${taskID}:${chart.id}`} chart={chart} execution={execution} />)}
      </Grid>
    </section>
  </main>;
}
