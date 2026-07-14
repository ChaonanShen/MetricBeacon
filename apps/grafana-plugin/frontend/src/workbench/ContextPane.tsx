import { Box, ScrollContainer, Stack, Text } from '@grafana/ui';

import type { Task } from '../api/resource';
import { Pane } from './ConversationPane';
import type { WorkbenchChart, WorkbenchState } from './types';

type Props = {
  sessionTitle?: string;
  task?: Task;
  runtime?: WorkbenchState;
  selectedChart?: WorkbenchChart;
};

export function ContextPane({ sessionTitle, task, runtime, selectedChart }: Props) {
  const query = selectedChart?.chart.queries?.[0];
  const execution = selectedChart?.execution;
  return <Pane aria-label="分析上下文" data-testid="context-pane" minHeight={{ xs: 'auto', xl: 0 }}>
    <Stack direction="column" gap={2} height="100%" minHeight={0}>
      <Box padding={3} paddingBottom={0}>
        <Text element="h2" variant="h4">分析上下文</Text>
      </Box>
      <ScrollContainer grow={1} minHeight={0} padding={3} paddingTop={0} overflowY="auto">
        <Stack direction="column" gap={3}>
          <ContextItem label="Session" value={sessionTitle ?? '当前会话'} />
          <ContextItem label="数据源" value={task?.datasourceUid ?? 'prometheus-main'} />
          <ContextItem label="时间范围" value={task ? formatTimeRange(task) : '等待分析请求'} />
          <ContextItem label="有效采样间隔" value={task ? `${task.queryPlan.stepSeconds} 秒` : '等待分析请求'} />
          <ContextItem label="CPU rate window" value={task ? `${task.queryPlan.cpuRateWindowSeconds} 秒` : '等待分析请求'} />
          <ContextItem label="Task 状态" value={runtime?.taskStatus ?? task?.status ?? '尚未开始'} />
          {selectedChart
            ? <>
              <Text element="h3" variant="h5">当前图表</Text>
              <ContextItem label="标题" value={selectedChart.chart.title} />
              <ContextItem label="查询状态" value={execution?.status ?? '等待查询结果'} />
              <ContextItem label="图表采样间隔" value={query?.stepSeconds ? `${query.stepSeconds} 秒` : '等待查询结果'} />
              <ContextItem label="序列数" value={String(execution?.seriesCount ?? 0)} />
              <ContextItem label="实际样本范围" value={formatActualRange(execution?.actualSampleRange)} />
              <ContextItem label="单位" value={selectedChart.chart.unit || '未指定'} />
              <Box>
                <Text color="secondary">PromQL</Text>
                <pre style={{ margin: '4px 0 0', overflowWrap: 'anywhere', whiteSpace: 'pre-wrap' }}>{query?.expression ?? '暂无查询'}</pre>
              </Box>
            </>
            : <Text color="secondary">选择一张图表以查看只读详情。</Text>}
        </Stack>
      </ScrollContainer>
    </Stack>
  </Pane>;
}

function formatActualRange(range: { from: string; to: string } | null | undefined): string {
  if (!range) return '无可用样本';
  return `${new Date(range.from).toLocaleString()} 至 ${new Date(range.to).toLocaleString()}`;
}

function ContextItem({ label, value }: { label: string; value: string }) {
  return <Box>
    <Text color="secondary">{label}</Text>
    <Text>{value}</Text>
  </Box>;
}

function formatTimeRange(task: Task): string {
  return `${new Date(task.timeRange.from).toLocaleString()} 至 ${new Date(task.timeRange.to).toLocaleString()}`;
}
