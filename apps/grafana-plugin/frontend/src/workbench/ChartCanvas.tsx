import { Box, Stack, Text } from '@grafana/ui';
import { forwardRef, useImperativeHandle, useRef } from 'react';

import { ChartCard } from './ChartCard';
import { compensatedScrollTop, type ChartGroup } from './chart-groups';
import { Pane } from './ConversationPane';

type Props = {
  groups: ChartGroup[];
  selectedChartId?: string;
  onSelectChart: (chartId: string) => void;
};

export type ChartCanvasHandle = {
  captureScrollSnapshot: () => void;
  restoreAfterPrepend: () => void;
  scrollTaskIntoView: (taskId: string, behavior?: ScrollBehavior) => void;
};

export const ChartCanvas = forwardRef<ChartCanvasHandle, Props>(function ChartCanvas({ groups, selectedChartId, onSelectChart }, ref) {
  const chartCount = groups.reduce((total, group) => total + group.charts.length, 0);
  const scrollRef = useRef<HTMLDivElement>(null);
  const groupRefs = useRef<Record<string, HTMLElement | null>>({});
  const snapshot = useRef<{ top: number; height: number }>();

  useImperativeHandle(ref, () => ({
    captureScrollSnapshot: () => {
      const node = scrollRef.current;
      if (node) snapshot.current = { top: node.scrollTop, height: node.scrollHeight };
    },
    restoreAfterPrepend: () => {
      const node = scrollRef.current;
      const before = snapshot.current;
      if (node && before) node.scrollTop = compensatedScrollTop(before.top, before.height, node.scrollHeight);
      snapshot.current = undefined;
    },
    scrollTaskIntoView: (taskId, behavior = 'smooth') => groupRefs.current[taskId]?.scrollIntoView({ block: 'nearest', behavior }),
  }), []);

  return <Pane aria-label="分析画布" data-testid="chart-canvas" grow={3} basis="0" minWidth={0}>
    <style>{chartCanvasCSS}</style>
    <Stack direction="column" gap={2} height="100%" minHeight={0}>
      <Box padding={3} paddingBottom={0} display="flex" justifyContent="space-between" gap={2}>
        <Text element="h2" variant="h4">分析画布</Text>
        <Text color="secondary">{groups.length} 轮分析 · {chartCount} 张图表</Text>
      </Box>
      <div ref={scrollRef} data-testid="chart-scroll-container" className="mtb-chart-scroll">
        {groups.length === 0
          ? <Box paddingY={8}><Text color="secondary">分析生成的图表会显示在这里。</Text></Box>
          : <div className="mtb-chart-groups">
            {groups.map((group) => <section ref={(node) => { groupRefs.current[group.taskId] = node; }} key={group.taskId} className="mtb-chart-group" data-testid="chart-group" data-task-id={group.taskId}>
              <Box marginBottom={2} minWidth={0}>
                <Text element="h3" variant="h5" title={group.prompt}>{group.prompt}</Text>
                <Text color="secondary">分析请求 · {new Date(group.createdAt).toLocaleString()} · {group.charts.length} 张图</Text>
              </Box>
              <div className="mtb-chart-grid">
                {group.charts.map(({ chart, execution }) => <div className="mtb-chart-wrapper" key={`${group.taskId}:${chart.id}`}>
                  <ChartCard chart={chart} execution={execution} selected={chart.id === selectedChartId} onSelect={() => onSelectChart(chart.id)} />
                </div>)}
              </div>
            </section>)}
          </div>}
      </div>
    </Stack>
  </Pane>;
});

const chartCanvasCSS = `
.mtb-chart-scroll { flex: 1 1 auto; min-height: 0; overflow-y: auto; overflow-x: hidden; padding: 0 24px 24px; }
.mtb-chart-groups { container-type: inline-size; min-width: 0; }
.mtb-chart-group { min-width: 0; }
.mtb-chart-group + .mtb-chart-group { margin-top: 32px; padding-top: 32px; border-top: 1px solid rgba(128, 128, 128, 0.28); }
.mtb-chart-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 16px; min-width: 0; align-items: stretch; }
.mtb-chart-wrapper { min-width: 0; }
@media (min-width: 1200px) {
  @container (min-width: 736px) {
    .mtb-chart-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .mtb-chart-wrapper:last-child:nth-child(odd) { grid-column: 1 / -1; }
  }
}
`;
