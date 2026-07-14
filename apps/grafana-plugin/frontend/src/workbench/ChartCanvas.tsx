import { Box, Grid, ScrollContainer, Stack, Text } from '@grafana/ui';

import { ChartCard } from './ChartCard';
import { Pane } from './ConversationPane';
import type { ChartGroup } from './chart-groups';

type Props = {
  groups: ChartGroup[];
  selectedChartId?: string;
  onSelectChart: (chartId: string) => void;
};

export function ChartCanvas({ groups, selectedChartId, onSelectChart }: Props) {
  const chartCount = groups.reduce((total, group) => total + group.charts.length, 0);
  return <Pane aria-label="分析画布" data-testid="chart-canvas" grow={1} basis="0" minWidth={0}>
    <Stack direction="column" gap={2} height="100%" minHeight={0}>
      <Box padding={3} paddingBottom={0} display="flex" justifyContent="space-between" gap={2}>
        <Text element="h2" variant="h4">分析画布</Text>
        <Text color="secondary">{groups.length} 轮分析 · {chartCount} 张图表</Text>
      </Box>
      <ScrollContainer grow={1} minHeight={0} padding={3} paddingTop={0} overflowY="auto">
        {groups.length === 0
          ? <Box paddingY={8}><Text color="secondary">分析生成的图表会显示在这里。</Text></Box>
          : <Stack direction="column" gap={4}>
            {groups.map((group) => <section key={group.taskId} data-testid="chart-group" data-task-id={group.taskId}>
              <Box marginBottom={2} minWidth={0}>
                <Text element="h3" variant="h5" title={group.prompt}>{group.prompt}</Text>
                <Text color="secondary">分析请求 · {new Date(group.createdAt).toLocaleString()} · {group.charts.length} 张图</Text>
              </Box>
              <Grid minColumnWidth={{ xs: 34, xl: 21 }} gap={2} alignItems="stretch">
                {group.charts.map(({ chart, execution }) => <ChartCard key={`${group.taskId}:${chart.id}`} chart={chart} execution={execution} selected={chart.id === selectedChartId} onSelect={() => onSelectChart(chart.id)} />)}
              </Grid>
            </section>)}
          </Stack>}
      </ScrollContainer>
    </Stack>
  </Pane>;
}
