import { AppRootProps } from '@grafana/data';
import { Box, Stack } from '@grafana/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useReducer, useRef, useState } from 'react';

import { resourceClient, type CreateTask, type GeneratedTaskEvent, type Task } from '../api/resource';
import { formatResourceError, isResourceNotFound } from '../api/resource-error';
import { ChartCanvas, type ChartCanvasHandle } from './ChartCanvas';
import { autoFocusTarget, deriveChartGroups } from './chart-groups';
import { ContextPane } from './ContextPane';
import { ConversationPane } from './ConversationPane';
import { createTaskInput } from './query-input';
import { clearWorkbenchRoute, readWorkbenchRoute, replaceWorkbenchRoute } from './route';
import { createInitialSessionWorkbenchState, isTerminal, sessionReducer } from './session-reducer';
import { deriveSessionTitle } from './session-list';
import { subscribeTaskEvents } from './sse';

type Event = Omit<GeneratedTaskEvent, 'payload'> & { payload: Record<string, unknown> };

export function Workbench(_props: AppRootProps) {
  const client = useQueryClient();
  const initialRoute = useMemo(() => readWorkbenchRoute(window.location.search), []);
  const [message, setMessage] = useState('');
  const [sessionId, setSessionId] = useState<string | undefined>(initialRoute.sessionId);
  const [selectedChartId, setSelectedChartId] = useState<string>();
  const [recoveryNotice, setRecoveryNotice] = useState<string>();
  const [state, dispatch] = useReducer(sessionReducer, initialRoute.sessionId, createInitialSessionWorkbenchState);
  const sequenceByTask = useRef<Record<string, number>>({});
  const replayedTasks = useRef(new Set<string>());
  const idempotencyKey = useRef<string>();
  const pendingTask = useRef<CreateTask>();
  const chartCanvas = useRef<ChartCanvasHandle>(null);
  const autoFocusedTask = useRef<string>();
  const session = useQuery({
    queryKey: ['mini-torchbearing-session', sessionId],
    queryFn: () => resourceClient.getSession(sessionId!),
    enabled: Boolean(sessionId),
    retry: (failureCount, error) => !isResourceNotFound(error) && failureCount < 3,
  });
  const history = useQuery({
    queryKey: ['mini-torchbearing-session-history', sessionId],
    queryFn: async () => Promise.all([resourceClient.listMessages(sessionId!), resourceClient.listTasks(sessionId!)]),
    enabled: Boolean(sessionId),
  });

  const taskOrderKey = state.taskOrder.join(',');
  useEffect(() => {
    if (!sessionId || !session.isError || !isResourceNotFound(session.error)) return;
    setSessionId(undefined);
    setSelectedChartId(undefined);
    setRecoveryNotice('已清除当前运行环境中不存在的旧会话，请重新提交分析。');
    sequenceByTask.current = {};
    replayedTasks.current = new Set<string>();
    idempotencyKey.current = undefined;
    pendingTask.current = undefined;
    dispatch({ type: 'session.selected' });
    clearWorkbenchRoute();
  }, [session.error, session.isError, sessionId]);

  useEffect(() => {
    if (!history.data) return;
    const [messages, tasks] = history.data;
    if (sessionId) dispatch({ type: 'history.loaded', sessionId, messages: messages.items, tasks: tasks.items, messageNextPageToken: messages.nextPageToken, taskNextPageToken: tasks.nextPageToken });
  }, [history.data, sessionId]);

  useEffect(() => {
    let cancelled = false;
    const replay = async (task: Task) => {
      if (replayedTasks.current.has(task.id)) return;
      let pageToken: string | undefined;
      let targetSequence = 0;
      do {
        const page = await resourceClient.replayEvents(task.id, pageToken);
        if (cancelled) return;
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
        dispatch({ type: 'task.replayed', sessionId: task.sessionId, taskId: task.id, targetSequence });
      }
    };
    void Promise.all(state.taskOrder.map((id) => replay(state.tasksById[id])));
    return () => { cancelled = true; };
  }, [taskOrderKey]);

  const activeTask = state.activeTaskId ? state.tasksById[state.activeTaskId] : undefined;
  const activeTaskReplayed = activeTask ? Boolean(state.replayedTaskIds[activeTask.id]) : false;
  useEffect(() => {
    if (!activeTask || !activeTaskReplayed || isTerminal(activeTask.status)) return;
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
  }, [activeTask?.id, activeTask?.status, activeTaskReplayed, client, sessionId]);

  const create = useMutation({
    mutationFn: async () => {
      if (!pendingTask.current) {
        const activeSession = sessionId ? { id: sessionId } : await resourceClient.createSession(deriveSessionTitle(message));
        pendingTask.current = createTaskInput(activeSession.id, message);
        if (!sessionId) {
          dispatch({ type: 'session.selected', sessionId: activeSession.id });
          setSessionId(activeSession.id);
        }
      }
      idempotencyKey.current ??= crypto.randomUUID();
      const task = await resourceClient.createTask(pendingTask.current, idempotencyKey.current);
      return { sessionId: pendingTask.current.sessionId, task };
    },
    onSuccess: (created) => {
      idempotencyKey.current = undefined;
      pendingTask.current = undefined;
      setSessionId(created.sessionId);
      setRecoveryNotice(undefined);
      dispatch({ type: 'task.created', sessionId: created.sessionId, task: created.task });
      replaceWorkbenchRoute(created.sessionId, created.task.id);
      client.setQueryData(['mini-torchbearing-session', created.sessionId], session.data);
      client.invalidateQueries({ queryKey: ['mini-torchbearing-session-history', created.sessionId] });
    },
  });
  const loadMore = useMutation({
    mutationFn: async () => {
      chartCanvas.current?.captureScrollSnapshot();
      const [messages, tasks] = await Promise.all([
        state.messageNextPageToken ? resourceClient.listMessages(sessionId!, state.messageNextPageToken) : Promise.resolve({ items: [], nextPageToken: state.messageNextPageToken }),
        state.taskNextPageToken ? resourceClient.listTasks(sessionId!, state.taskNextPageToken) : Promise.resolve({ items: [], nextPageToken: state.taskNextPageToken }),
      ]);
      return { messages, tasks };
    },
    onSuccess: ({ messages, tasks }) => {
      dispatch({ type: 'history.loaded', sessionId: sessionId!, messages: messages.items, tasks: tasks.items, messageNextPageToken: messages.nextPageToken, taskNextPageToken: tasks.nextPageToken });
      requestAnimationFrame(() => requestAnimationFrame(() => chartCanvas.current?.restoreAfterPrepend()));
    },
  });
  const startNewConversation = () => {
    if (create.isPending || loadMore.isPending) return;
    if (sessionId) {
      void client.cancelQueries({ queryKey: ['mini-torchbearing-session', sessionId] });
      void client.cancelQueries({ queryKey: ['mini-torchbearing-session-history', sessionId] });
    }
    setMessage('');
    setSessionId(undefined);
    setSelectedChartId(undefined);
    setRecoveryNotice(undefined);
    sequenceByTask.current = {};
    replayedTasks.current = new Set<string>();
    idempotencyKey.current = undefined;
    pendingTask.current = undefined;
    autoFocusedTask.current = undefined;
    create.reset();
    loadMore.reset();
    dispatch({ type: 'session.selected' });
    clearWorkbenchRoute();
  };
  const submit = () => { if (message.trim() && !activeTask) create.mutate(); };
  const staleSession = session.isError && isResourceNotFound(session.error);
  const requestError = [create.error, loadMore.error, staleSession ? undefined : session.error, staleSession ? undefined : history.error].find(Boolean);
  const messages = state.messageOrder.map((id) => state.messagesById[id]);
  const groups = deriveChartGroups(state.taskOrder.map((taskID) => state.tasksById[taskID]), messages, state.runtimeByTaskId);
  const charts = groups.flatMap((group) => group.charts.map((chart) => ({ taskID: group.taskId, ...chart })));
  const chartIDs = charts.map(({ chart }) => chart.id).join(',');
  const allHistoryReplayed = state.taskOrder.length > 0 && state.taskOrder.every((taskId) => state.replayedTaskIds[taskId]);
  useEffect(() => {
    setSelectedChartId((current) => current && charts.some(({ chart }) => chart.id === current) ? current : undefined);
    const target = autoFocusTarget(groups, state.activeTaskId, autoFocusedTask.current, allHistoryReplayed, selectedChartId);
    if (target) {
      if (target.behavior === 'smooth') autoFocusedTask.current = target.taskId;
      setSelectedChartId(target.chartId);
      requestAnimationFrame(() => chartCanvas.current?.scrollTaskIntoView(target.taskId, target.behavior));
    }
  }, [allHistoryReplayed, chartIDs, state.activeTaskId]);
  const selectedChart = charts.find(({ chart }) => chart.id === selectedChartId);
  const contextTask = selectedChart ? state.tasksById[selectedChart.taskID] : activeTask ?? state.tasksById[state.taskOrder[0]];

  return <main style={{ padding: 16 }}>
    <h2>Mini Torchbearing Workbench</h2>
    <Stack direction={{ xs: 'column', xl: 'row' }} gap={2} height={{ xs: 'auto', xl: 'calc(100dvh - 112px)' }} alignItems="stretch">
      <Box width={{ xs: '100%', xl: 'auto' }} minWidth={{ xs: 0, xl: '320px' }} maxWidth={{ xs: 'none', xl: '420px' }} height={{ xs: 'auto', xl: '100%' }} grow={{ xs: 0, xl: 1 }} shrink={{ xs: 1, xl: 1 }} basis={{ xs: 'auto', xl: '0' }}>
        <ConversationPane sessionTitle={session.data?.title} messages={messages} tasks={state.taskOrder.map((id) => state.tasksById[id])} runtimeByTaskId={state.runtimeByTaskId} activeTask={activeTask} message={message} busy={create.isPending || session.isFetching || history.isFetching} canLoadMore={Boolean(state.messageNextPageToken || state.taskNextPageToken)} loadingMore={loadMore.isPending} notice={recoveryNotice} requestError={requestError ? formatResourceError(requestError) : undefined} newConversationDisabled={create.isPending || loadMore.isPending} onMessageChange={setMessage} onSubmit={submit} onLoadMore={() => loadMore.mutate()} onNewConversation={startNewConversation} />
      </Box>
      <ChartCanvas ref={chartCanvas} groups={groups} selectedChartId={selectedChartId} onSelectChart={setSelectedChartId} />
      <Box width={{ xs: '100%', xl: '280px' }} shrink={{ xs: 1, xl: 0 }}>
        <ContextPane sessionTitle={session.data?.title} task={contextTask} runtime={contextTask ? state.runtimeByTaskId[contextTask.id] : undefined} selectedChart={selectedChart} />
      </Box>
    </Stack>
  </main>;
}
