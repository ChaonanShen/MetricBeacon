import { Button, Box, Field, Input, ScrollContainer, Spinner, Stack, Text } from '@grafana/ui';

import type { Message, Task } from '../api/resource';
import { rangeOptions, resolutionOptions, type RangeOption, type ResolutionOption } from './query-options';
import type { WorkbenchState } from './types';

type Props = {
  sessionTitle?: string;
  messages: Message[];
  tasks: Task[];
  runtimeByTaskId: Record<string, WorkbenchState>;
  activeTask?: Task;
  message: string;
  range: RangeOption;
  resolution: ResolutionOption;
  busy: boolean;
  canLoadMore: boolean;
  loadingMore: boolean;
  notice?: string;
  requestError?: string;
  onMessageChange: (message: string) => void;
  onRangeChange: (range: RangeOption) => void;
  onResolutionChange: (resolution: ResolutionOption) => void;
  onSubmit: () => void;
  onLoadMore: () => void;
};

export function ConversationPane({ sessionTitle, messages, tasks, runtimeByTaskId, activeTask, message, range, resolution, busy, canLoadMore, loadingMore, notice, requestError, onMessageChange, onRangeChange, onResolutionChange, onSubmit, onLoadMore }: Props) {
  return <Pane aria-label="对话" data-testid="conversation-pane" minHeight={{ xs: '420px', xl: 0 }}>
    <Stack direction="column" gap={2} height="100%" minHeight={0}>
      <Box padding={3} paddingBottom={0}>
        <Text element="h2" variant="h4">对话</Text>
        <Text color="secondary">{sessionTitle ?? '当前会话'}</Text>
      </Box>
      <ScrollContainer grow={1} minHeight={0} paddingX={3} overflowY="auto">
        <Stack direction="column" gap={2}>
          {canLoadMore && <Button variant="secondary" onClick={onLoadMore} disabled={loadingMore}>加载更早记录</Button>}
          {messages.map((item) => <Text key={item.id}><strong>{item.role === 'user' ? '你' : '助手'}：</strong>{item.content}</Text>)}
          {tasks.map((task) => {
            const runtime = runtimeByTaskId[task.id];
            return runtime?.assistantText && !messages.some((item) => item.taskId === task.id && item.role === 'assistant')
              ? <Text key={`${task.id}-draft`}><strong>助手：</strong>{runtime.assistantText}</Text>
              : null;
          })}
          {tasks.map((task) => runtimeByTaskId[task.id]?.error && <Text role="alert" key={`${task.id}-error`} color="error">{runtimeByTaskId[task.id].error!.code}: {runtimeByTaskId[task.id].error!.message}</Text>)}
        </Stack>
      </ScrollContainer>
      <Box padding={3} paddingTop={0}>
        <Stack direction="column" gap={2}>
          {notice && <Text role="status">{notice}</Text>}
          {requestError && <Text role="alert" color="error">{requestError}</Text>}
          {activeTask && <Text>Task 状态：{runtimeByTaskId[activeTask.id]?.taskStatus ?? activeTask.status}</Text>}
          <Field label="分析请求">
            <Input value={message} onChange={(event) => onMessageChange(event.currentTarget.value)} placeholder="例如：查看 node exporter" />
          </Field>
          <Field label="默认时间范围" description="消息中的明确时间（如“最近 5 分钟”）优先。">
            <select aria-label="默认时间范围" value={range} onChange={(event) => onRangeChange(event.currentTarget.value as RangeOption)} disabled={busy || Boolean(activeTask)} style={{ width: '100%', minHeight: 32 }}>
              {rangeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </Field>
          <Field label="采样分辨率" description="消息中的明确间隔（如“每 5 秒一个点”）优先。">
            <select aria-label="采样分辨率" value={resolution} onChange={(event) => onResolutionChange(event.currentTarget.value as ResolutionOption)} disabled={busy || Boolean(activeTask)} style={{ width: '100%', minHeight: 32 }}>
              {resolutionOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </Field>
          <Box display="flex" alignItems="center" gap={2}>
            <Button onClick={onSubmit} disabled={!message.trim() || busy || Boolean(activeTask)}>开始分析</Button>
            {busy && <Spinner />}
          </Box>
        </Stack>
      </Box>
    </Stack>
  </Pane>;
}

export function Pane(props: React.ComponentProps<typeof Box>) {
  return <Box backgroundColor="secondary" borderColor="weak" borderStyle="solid" borderRadius="md" minWidth={0} {...props} />;
}
