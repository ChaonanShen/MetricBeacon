import { Box, Grid, ScrollContainer, Stack, Text } from '@grafana/ui';

import { ChartCard } from './ChartCard';
import { Pane } from './ConversationPane';
import type { WorkbenchChart } from './types';

type Props = {
  charts: Array<WorkbenchChart & { taskID: string }>;
  selectedChartId?: string;
  onSelectChart: (chartId: string) => void;
};

export function ChartCanvas({ charts, selectedChartId, onSelectChart }: Props) {
  return <Pane aria-label="分析画布" data-testid="chart-canvas" grow={1} basis="0" minWidth={0}>
    <Stack direction="column" gap={2} height="100%" minHeight={0}>
      <Box padding={3} paddingBottom={0} display="flex" justifyContent="space-between" gap={2}>
        <Text element="h2" variant="h4">分析画布</Text>
        <Text color="secondary">{charts.length} 张图表</Text>
      </Box>
      <ScrollContainer grow={1} minHeight={0} padding={3} paddingTop={0} overflowY="auto">
        {charts.length === 0
          ? <Box paddingY={8}><Text color="secondary">分析生成的图表会显示在这里。</Text></Box>
          : <Grid minColumnWidth={34} gap={2} alignItems="stretch">
            {charts.map(({ taskID, chart, execution }) => <ChartCard key={`${taskID}:${chart.id}`} chart={chart} execution={execution} selected={chart.id === selectedChartId} onSelect={() => onSelectChart(chart.id)} />)}
          </Grid>}
      </ScrollContainer>
    </Stack>
  </Pane>;
}
