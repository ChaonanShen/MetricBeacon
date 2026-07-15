import { forwardRef, useImperativeHandle, useRef } from 'react';

import { ChartCard } from './ChartCard';
import { compensatedScrollTop, type ChartGroup } from './chart-groups';

type Props = {
  groups: ChartGroup[];
};

export type ChartCanvasHandle = {
  captureScrollSnapshot: () => void;
  restoreAfterPrepend: () => void;
  scrollTaskIntoView: (taskId: string, behavior?: ScrollBehavior) => void;
};

export const ChartCanvas = forwardRef<ChartCanvasHandle, Props>(function ChartCanvas({ groups }, ref) {
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

  return <section aria-label="Canvas 图表画布" data-testid="chart-canvas" className="mtb-canvas-pane">
    <header className="mtb-canvas-header">
      <div>
        <span className="mtb-pane-kicker">Canvas</span>
        <h2>分析画布</h2>
      </div>
      <span className="mtb-canvas-count">{groups.length} 轮分析 · {chartCount} 张图表</span>
    </header>
    <div ref={scrollRef} data-testid="chart-scroll-container" className="mtb-chart-scroll">
      {groups.length === 0
        ? <div className="mtb-canvas-empty"><strong>空 Canvas</strong><p>提交指标分析后，真实查询生成的 Grafana 时序图会显示在这里。</p></div>
        : <div className="mtb-chart-groups">
          {groups.map((group) => <section ref={(node) => { groupRefs.current[group.taskId] = node; }} key={group.taskId} className="mtb-chart-group" data-testid="chart-group" data-task-id={group.taskId}>
            <header className="mtb-chart-group-header">
              <h3 title={group.prompt}>{group.prompt}</h3>
              <p>分析请求 · {new Date(group.createdAt).toLocaleString()} · {group.charts.length} 张图</p>
            </header>
            <div className="mtb-chart-grid">
              {group.charts.map(({ chart, execution }) => <div className="mtb-chart-wrapper" key={`${group.taskId}:${chart.id}`}>
                <ChartCard chart={chart} execution={execution} />
              </div>)}
            </div>
          </section>)}
        </div>}
    </div>
  </section>;
});
