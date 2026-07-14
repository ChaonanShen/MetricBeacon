import { FieldType, getDisplayProcessor, type DataFrame } from '@grafana/data';
import { Badge, Box, Button, LegendDisplayMode, Spinner, Stack, Text, TimeSeries, useTheme2, type BadgeColor } from '@grafana/ui';
import { useEffect, useMemo, useRef, useState } from 'react';

import { ChartWireToDataFrame } from './mapper';
import { chartTimeRange } from './time-range';
import type { ChartWire, ExecutionWire } from './types';

const plotHeight = 260;

type Props = {
  chart: ChartWire;
  execution?: ExecutionWire;
  selected?: boolean;
  onSelect?: () => void;
};

export function ChartCard({ chart, execution, selected = false, onSelect }: Props) {
  const theme = useTheme2();
  const { plotRef, width } = usePlotWidth();
  const timeRange = useMemo(() => chartTimeRange(chart, execution), [chart, execution]);
  const frame = useMemo(() => execution ? ChartWireToDataFrame(execution.series, chart.unit) : undefined, [chart.unit, execution]);
  const displayFrame = useMemo(() => frame ? withDisplayProcessors(frame, theme) : undefined, [frame, theme]);
  const status = chartStatus(execution);

  return <Box data-testid="timeseries-panel" element="article" backgroundColor="secondary" borderColor={selected ? 'strong' : 'weak'} borderStyle="solid" borderRadius="md" padding={3} minWidth={0}>
    <Stack direction="column" gap={2}>
      <Box display="flex" alignItems="center" justifyContent="space-between" gap={2} minWidth={0}>
        <Box grow={1} minWidth={0}>
          <Text element="h3" variant="h4" weight="medium">{chart.title}</Text>
        </Box>
        <Stack direction="row" gap={1} alignItems="center">
          <Badge color={status.color} text={status.text} />
          <Button size="sm" variant="secondary" aria-pressed={selected} onClick={onSelect}>详情</Button>
        </Stack>
      </Box>
      {chart.queries?.[0]?.expression && <details>
        <summary>PromQL</summary>
        <pre style={{ margin: '8px 0 0', overflowWrap: 'anywhere', whiteSpace: 'pre-wrap' }}>{chart.queries[0].expression}</pre>
      </details>}
      <div ref={plotRef} data-testid="timeseries-plot" style={{ height: plotHeight, minWidth: 0, overflow: 'hidden', width: '100%' }}>
        <ChartPlot execution={execution} frame={displayFrame} timeRange={timeRange} width={width} />
      </div>
    </Stack>
  </Box>;
}

function ChartPlot({ execution, frame, timeRange, width }: { execution?: ExecutionWire; frame?: DataFrame; timeRange?: ReturnType<typeof chartTimeRange>; width: number }) {
  if (!execution) {
    return <PlotMessage loading="等待查询结果" />;
  }
  if (execution.status === 'failed') {
    return <PlotMessage message="查询失败" />;
  }
  if (!timeRange) {
    return <PlotMessage message="缺少时间范围" />;
  }
  if (!frame || execution.series.length === 0) {
    return <PlotMessage message="暂无数据" />;
  }
  if (width === 0) {
    return <PlotMessage loading="正在准备图表" />;
  }
  return <TimeSeries width={width} height={plotHeight} timeRange={timeRange} timeZone="browser" frames={[frame]} legend={{ showLegend: true, placement: 'bottom', displayMode: LegendDisplayMode.List, calcs: [] }} />;
}

function PlotMessage({ loading, message }: { loading?: string; message?: string }) {
  return <Box display="flex" alignItems="center" justifyContent="center" gap={1} height={plotHeight}>
    {loading && <Spinner inline size="md" />}
    <Text color="secondary">{loading ?? message ?? '暂无数据'}</Text>
  </Box>;
}

function chartStatus(execution?: ExecutionWire): { color: BadgeColor; text: string } {
  if (!execution) {
    return { color: 'blue', text: '查询中' };
  }
  if (execution.status === 'success') {
    return { color: 'green', text: '已加载' };
  }
  if (execution.status === 'failed') {
    return { color: 'red', text: '加载失败' };
  }
  return { color: 'darkgrey', text: '状态未知' };
}

function withDisplayProcessors(frame: DataFrame, theme: ReturnType<typeof useTheme2>): DataFrame {
  return {
    ...frame,
    fields: frame.fields.map((field) => ({
      ...field,
      display: field.type === FieldType.number ? getDisplayProcessor({ field, theme }) : field.display,
    })),
  };
}

function usePlotWidth() {
  const plotRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);

  useEffect(() => {
    const element = plotRef.current;
    if (!element) {
      return;
    }
    const updateWidth = (nextWidth: number) => {
      const roundedWidth = Math.floor(nextWidth);
      setWidth((currentWidth) => currentWidth === roundedWidth ? currentWidth : roundedWidth);
    };
    updateWidth(element.getBoundingClientRect().width);
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) {
        updateWidth(entry.contentRect.width);
      }
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return { plotRef, width };
}
