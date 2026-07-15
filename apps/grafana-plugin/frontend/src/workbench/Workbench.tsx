import { AppRootProps } from '@grafana/data';
import { config } from '@grafana/runtime';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useReducer, useRef, useState } from 'react';

import { resourceClient, type CreateTask, type GeneratedTaskEvent, type Task } from '../api/resource';
import { formatResourceError, isResourceNotFound } from '../api/resource-error';
import { ChartCanvas, type ChartCanvasHandle } from './ChartCanvas';
import { IncidentCanvas } from './IncidentCanvas';
import { autoFocusTask, deriveChartGroups } from './chart-groups';
import { createTaskInput } from './query-input';
import { clearWorkbenchRoute, readWorkbenchRoute, replaceWorkbenchRoute, replaceWorkbenchSessionRoute } from './route';
import { createInitialSessionWorkbenchState, isTerminal, sessionReducer } from './session-reducer';
import { deriveSessionTitle, flattenSessionPages } from './session-list';
import { subscribeTaskEvents } from './sse';
import { deriveIncidentView } from './incident-view';
import { deriveWorkbenchContext } from './workbench-view';
import { WorkbenchShell } from './WorkbenchShell';

type Event = Omit<GeneratedTaskEvent, 'payload'> & { payload: Record<string, unknown> };

export function Workbench(_props: AppRootProps) {
  const client = useQueryClient();
  const initialRoute = useMemo(() => readWorkbenchRoute(window.location.search), []);
  const [message, setMessage] = useState('');
  const [sessionId, setSessionId] = useState<string | undefined>(initialRoute.sessionId);
  const [recoveryNotice, setRecoveryNotice] = useState<string>();
  const [state, dispatch] = useReducer(sessionReducer, initialRoute.sessionId, createInitialSessionWorkbenchState);
  const sequenceByTask = useRef<Record<string, number>>({});
  const replayedTasks = useRef(new Set<string>());
  const idempotencyKey = useRef<string>();
  const pendingTask = useRef<CreateTask>();
  const chartCanvas = useRef<ChartCanvasHandle>(null);
  const autoFocusedTask = useRef<string>();
  const historyFocusedSession = useRef<string>();
  const session = useQuery({
    queryKey: ['mini-torchbearing-session', sessionId],
    queryFn: () => resourceClient.getSession(sessionId!),
    enabled: Boolean(sessionId),
    retry: (failureCount, error) => !isResourceNotFound(error) && failureCount < 3,
  });
  const history = useQuery({
    queryKey: ['mini-torchbearing-session-history', sessionId],
    queryFn: async () => {
      const requestedSessionId = sessionId!;
      const [messages, tasks] = await Promise.all([resourceClient.listMessages(requestedSessionId), resourceClient.listTasks(requestedSessionId)]);
      return { sessionId: requestedSessionId, messages, tasks };
    },
    enabled: Boolean(sessionId),
  });
  const sessionPages = useInfiniteQuery({
    queryKey: ['mini-torchbearing-sessions'],
    queryFn: ({ pageParam }) => resourceClient.listSessions(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextPageToken ?? undefined,
  });
  const incidentPages = useInfiniteQuery({
    queryKey: ['mini-torchbearing-incidents'],
    queryFn: ({ pageParam }) => resourceClient.listIncidents(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextPageToken ?? undefined,
  });

  const taskOrderKey = state.taskOrder.join(',');
  useEffect(() => {
    if (!sessionId || !session.isError || !isResourceNotFound(session.error)) return;
    setSessionId(undefined);
    setRecoveryNotice('已清除当前运行环境中不存在的旧会话，请重新提交分析。');
    sequenceByTask.current = {};
    replayedTasks.current = new Set<string>();
    idempotencyKey.current = undefined;
    pendingTask.current = undefined;
    autoFocusedTask.current = undefined;
    historyFocusedSession.current = undefined;
    dispatch({ type: 'session.selected' });
    clearWorkbenchRoute();
    void client.invalidateQueries({ queryKey: ['mini-torchbearing-sessions'] });
  }, [client, session.error, session.isError, sessionId]);

  useEffect(() => {
    if (!history.data) return;
    dispatch({ type: 'history.loaded', sessionId: history.data.sessionId, messages: history.data.messages.items, tasks: history.data.tasks.items, messageNextPageToken: history.data.messages.nextPageToken, taskNextPageToken: history.data.tasks.nextPageToken });
  }, [history.data]);

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
  const incidentTask = activeTask?.kind === 'incident_remediation' ? activeTask : state.taskOrder.map((id) => state.tasksById[id]).find((task) => task.kind === 'incident_remediation');
  const approval = useQuery({
    queryKey: ['mini-torchbearing-approval', incidentTask?.id],
    queryFn: () => resourceClient.getApproval(incidentTask!.id),
    enabled: Boolean(incidentTask?.incidentPlan?.intent),
    refetchInterval: incidentTask?.status === 'waiting_approval' ? 5000 : false,
  });
  const decideApproval = useMutation({
    mutationFn: (decision: 'approve' | 'reject') => resourceClient.decideApproval(incidentTask!.id, { decision, reason: decision === 'approve' ? 'Approved in Grafana Incident Workbench' : 'Rejected in Grafana Incident Workbench', expectedTaskVersion: incidentTask!.version, expectedApprovalVersion: approval.data!.version, intentDigest: approval.data!.intentDigest }, crypto.randomUUID()),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['mini-torchbearing-approval', incidentTask?.id] });
      void client.invalidateQueries({ queryKey: ['mini-torchbearing-session-history', sessionId] });
      void client.invalidateQueries({ queryKey: ['mini-torchbearing-incidents'] });
    },
  });
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
      (event) => event.type === 'task.completed' || event.type === 'task.failed' || (event.type === 'task.status_changed' && event.payload.status === 'cancelled'),
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
      void client.invalidateQueries({ queryKey: ['mini-torchbearing-session', created.sessionId] });
      void client.invalidateQueries({ queryKey: ['mini-torchbearing-session-history', created.sessionId] });
      void client.invalidateQueries({ queryKey: ['mini-torchbearing-sessions'] });
    },
  });
  const loadMore = useMutation({
    mutationFn: async () => {
      chartCanvas.current?.captureScrollSnapshot();
      const requestedSessionId = sessionId!;
      const [messages, tasks] = await Promise.all([
        state.messageNextPageToken ? resourceClient.listMessages(requestedSessionId, state.messageNextPageToken) : Promise.resolve({ items: [], nextPageToken: state.messageNextPageToken }),
        state.taskNextPageToken ? resourceClient.listTasks(requestedSessionId, state.taskNextPageToken) : Promise.resolve({ items: [], nextPageToken: state.taskNextPageToken }),
      ]);
      return { sessionId: requestedSessionId, messages, tasks };
    },
    onSuccess: ({ sessionId: loadedSessionId, messages, tasks }) => {
      dispatch({ type: 'history.loaded', sessionId: loadedSessionId, messages: messages.items, tasks: tasks.items, messageNextPageToken: messages.nextPageToken, taskNextPageToken: tasks.nextPageToken });
      requestAnimationFrame(() => requestAnimationFrame(() => chartCanvas.current?.restoreAfterPrepend()));
    },
  });
  const selectConversation = (nextSessionId?: string) => {
    if (create.isPending || loadMore.isPending) return;
    if (nextSessionId && nextSessionId === sessionId) return;
    if (sessionId) {
      void client.cancelQueries({ queryKey: ['mini-torchbearing-session', sessionId] });
      void client.cancelQueries({ queryKey: ['mini-torchbearing-session-history', sessionId] });
    }
    setMessage('');
    setSessionId(nextSessionId);
    setRecoveryNotice(undefined);
    sequenceByTask.current = {};
    replayedTasks.current = new Set<string>();
    idempotencyKey.current = undefined;
    pendingTask.current = undefined;
    autoFocusedTask.current = undefined;
    historyFocusedSession.current = undefined;
    create.reset();
    loadMore.reset();
    dispatch({ type: 'session.selected', sessionId: nextSessionId });
    if (nextSessionId) replaceWorkbenchSessionRoute(nextSessionId);
    else clearWorkbenchRoute();
  };
  const submit = () => { if (message.trim() && !activeTask) create.mutate(); };
  const staleSession = session.isError && isResourceNotFound(session.error);
  const requestError = [create.error, loadMore.error, decideApproval.error, staleSession ? undefined : session.error, staleSession ? undefined : history.error].find(Boolean);
  const messages = useMemo(() => state.messageOrder.map((id) => state.messagesById[id]), [state.messageOrder, state.messagesById]);
  const tasks = useMemo(() => state.taskOrder.map((id) => state.tasksById[id]), [state.taskOrder, state.tasksById]);
  const groups = deriveChartGroups(tasks, messages, state.runtimeByTaskId);
  const chartGroupKey = groups.map((group) => `${group.taskId}:${group.charts.map(({ chart }) => chart.id).join(',')}`).join(';');
  const allHistoryReplayed = state.taskOrder.length > 0 && state.taskOrder.every((taskId) => state.replayedTaskIds[taskId]);
  useEffect(() => {
    const target = autoFocusTask(groups, state.activeTaskId, autoFocusedTask.current, allHistoryReplayed, historyFocusedSession.current === sessionId);
    if (target) {
      if (target.behavior === 'smooth') autoFocusedTask.current = target.taskId;
      else historyFocusedSession.current = sessionId;
      requestAnimationFrame(() => chartCanvas.current?.scrollTaskIntoView(target.taskId, target.behavior));
    }
  }, [allHistoryReplayed, chartGroupKey, sessionId, state.activeTaskId]);
  const listedSessions = flattenSessionPages(sessionPages.data?.pages ?? []);
  const listedIncidents = (incidentPages.data?.pages ?? []).flatMap((page) => page.items).filter((task, index, values) => values.findIndex((candidate) => candidate.id === task.id) === index);
  const switchingDisabled = create.isPending || loadMore.isPending;
  const context = useMemo(() => deriveWorkbenchContext(session.data?.title, tasks, activeTask), [activeTask, session.data?.title, tasks]);
  const incidentView = useMemo(() => incidentTask ? deriveIncidentView(incidentTask, state.runtimeByTaskId[incidentTask.id], approval.data, config.bootData.user.orgRole === 'Admin') : undefined, [approval.data, incidentTask, state.runtimeByTaskId]);
  const decide = (decision: 'approve' | 'reject') => {
    const message = decision === 'approve' ? '确认按当前不可变 Intent/Diff 执行 worker concurrency 0 → 2？' : '确认拒绝当前修复 Intent？';
    if (window.confirm(message)) decideApproval.mutate(decision);
  };

  return <WorkbenchShell
    context={context}
    sessions={{ sessions: listedSessions, selectedSessionId: sessionId, loading: sessionPages.isLoading, loadingMore: sessionPages.isFetchingNextPage, hasMore: Boolean(sessionPages.hasNextPage), error: sessionPages.error ? formatResourceError(sessionPages.error) : undefined, switchingDisabled, onNewConversation: () => selectConversation(), onSelectSession: selectConversation, onLoadMore: () => { void sessionPages.fetchNextPage(); } }}
    incidents={{ incidents: listedIncidents, selectedTaskId: incidentTask?.id, loading: incidentPages.isLoading, loadingMore: incidentPages.isFetchingNextPage, hasMore: Boolean(incidentPages.hasNextPage), error: incidentPages.error ? formatResourceError(incidentPages.error) : undefined, switchingDisabled, onSelectIncident: (task) => selectConversation(task.sessionId), onLoadMore: () => { void incidentPages.fetchNextPage(); } }}
    chat={{ sessionTitle: session.data?.title, messages, tasks, runtimeByTaskId: state.runtimeByTaskId, activeTask, message, busy: create.isPending || session.isFetching || history.isFetching, canLoadMore: Boolean(state.messageNextPageToken || state.taskNextPageToken), loadingMore: loadMore.isPending, notice: recoveryNotice, requestError: requestError ? formatResourceError(requestError) : undefined, onMessageChange: setMessage, onSubmit: submit, onLoadMore: () => loadMore.mutate(), incident: Boolean(incidentTask) }}
    canvas={incidentTask ? <IncidentCanvas incident={incidentView} loading={approval.isLoading} deciding={decideApproval.isPending} error={approval.error && !isResourceNotFound(approval.error) ? formatResourceError(approval.error) : undefined} onApprove={() => decide('approve')} onReject={() => decide('reject')} /> : <ChartCanvas ref={chartCanvas} groups={groups} />}
  />;
}
