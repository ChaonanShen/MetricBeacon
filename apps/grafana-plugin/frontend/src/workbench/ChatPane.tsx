import { Button, Field, Input, Spinner } from '@grafana/ui';
import type { FormEvent, RefObject } from 'react';

import type { Message, Task } from '../api/resource';
import type { WorkbenchState } from './types';
import { examplePrompts } from './workbench-view';
import { SessionMenu, type IncidentControls, type SessionControls } from './SessionMenu';

export type ChatControls = {
  sessionTitle?: string;
  messages: Message[];
  tasks: Task[];
  runtimeByTaskId: Record<string, WorkbenchState>;
  activeTask?: Task;
  message: string;
  busy: boolean;
  canLoadMore: boolean;
  loadingMore: boolean;
  notice?: string;
  requestError?: string;
  onMessageChange: (message: string) => void;
  onSubmit: () => void;
  onLoadMore: () => void;
  incident: boolean;
};

type Props = {
  chat: ChatControls;
  sessions: SessionControls;
  incidents: IncidentControls;
  selectorMode: 'sessions' | 'incidents';
  onSelectorModeChange: (mode: 'sessions' | 'incidents') => void;
  sessionMenuOpen: boolean;
  onSessionMenuOpenChange: (open: boolean) => void;
  sessionMenuToggleRef: RefObject<HTMLButtonElement>;
};

export function ChatPane({ chat, sessions, incidents, selectorMode, onSelectorModeChange, sessionMenuOpen, onSessionMenuOpenChange, sessionMenuToggleRef }: Props) {
  const { sessionTitle, messages, tasks, runtimeByTaskId, activeTask, message, busy, canLoadMore, loadingMore, notice, requestError, onMessageChange, onSubmit, onLoadMore, incident } = chat;
  const hasConversation = messages.length > 0 || tasks.length > 0;
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit();
  };

  return <section className="mtb-chat-pane" aria-label="聊天" data-testid="chat-pane">
    <div className="mtb-chat-header">
      <div>
        <span className="mtb-pane-kicker">Chat</span>
        <h2>{incident ? '事件处置' : '指标分析'}</h2>
        <p>{sessionTitle ?? '新对话'}</p>
      </div>
    </div>
    <SessionMenu {...sessions} incidents={incidents} mode={selectorMode} onModeChange={onSelectorModeChange} open={sessionMenuOpen} onOpenChange={onSessionMenuOpenChange} toggleRef={sessionMenuToggleRef} />
    <div className="mtb-chat-timeline" data-testid="conversation-scroll-container" aria-live="polite">
      {canLoadMore && <Button variant="secondary" size="sm" onClick={onLoadMore} disabled={loadingMore}>{loadingMore ? '加载中…' : '加载更早记录'}</Button>}
      {!hasConversation && !incident && <div className="mtb-chat-empty">
        <strong>开始一次指标分析</strong>
        <p>描述你关心的 CPU、内存或系统负载。你也可以选择一个示例填入输入框。</p>
        <div className="mtb-example-prompts">
          {examplePrompts.map((prompt) => <button type="button" key={prompt} onClick={() => onMessageChange(prompt)}>{prompt}</button>)}
        </div>
      </div>}
      {messages.map((item) => <article key={item.id} className={`mtb-message is-${item.role}`} data-testid="chat-message">
        <strong>{item.role === 'user' ? '你' : item.role === 'trigger' ? '告警' : '助手'}：</strong>{item.content}
      </article>)}
      {tasks.map((task) => {
        const runtime = runtimeByTaskId[task.id];
        return runtime?.assistantText && !messages.some((item) => item.taskId === task.id && item.role === 'assistant')
          ? <article key={`${task.id}-draft`} className="mtb-message is-assistant" data-testid="assistant-draft"><strong>助手：</strong>{runtime.assistantText}</article>
          : null;
      })}
      {tasks.map((task) => runtimeByTaskId[task.id]?.error
        ? <p role="alert" key={`${task.id}-error`} className="mtb-inline-error">{runtimeByTaskId[task.id].error!.code}: {runtimeByTaskId[task.id].error!.message}</p>
        : null)}
    </div>
    {incident ? <div className="mtb-chat-composer"><p className="mtb-muted">事件由 Prometheus 告警触发；诊断与修复只通过版本化 Playbook 和受控工具推进。</p>{requestError && <p role="alert" className="mtb-inline-error">{requestError}</p>}{activeTask && <p role="status" className="mtb-task-status">Task 状态：{runtimeByTaskId[activeTask.id]?.taskStatus ?? activeTask.status}</p>}</div> : <form className="mtb-chat-composer" onSubmit={submit}>
      {notice && <p role="status" className="mtb-inline-notice">{notice}</p>}
      {requestError && <p role="alert" className="mtb-inline-error">{requestError}</p>}
      {activeTask && <p role="status" className="mtb-task-status">Task 状态：{runtimeByTaskId[activeTask.id]?.taskStatus ?? activeTask.status}</p>}
      <Field label="分析请求">
        <Input value={message} onChange={(event) => onMessageChange(event.currentTarget.value)} placeholder="描述想分析的指标、服务或现象" />
      </Field>
      <div className="mtb-composer-actions">
        <Button type="submit" disabled={!message.trim() || busy || Boolean(activeTask)}>开始分析</Button>
        {busy && <Spinner inline size="sm" />}
      </div>
    </form>}
  </section>;
}
