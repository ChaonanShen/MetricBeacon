import type { components } from '../api/generated/plugin-resource';

export type TaskStatus = components['schemas']['task.schema']['status'];
export type ChartWire = components['schemas']['chart'] & {
  unit?: string;
  queries?: Array<{ expression?: string; timeRange?: { from: string; to: string }; stepSeconds?: 5 | 10 | 15 | 30 | 60 | 120 | 300 }>;
};
export type ChartSeriesWire = {
  name: string;
  labels: Record<string, string>;
  points: Array<{ timestamp: string; value: number }>;
};
export type ExecutionWire = components['schemas']['execution'] & {
  series: ChartSeriesWire[];
  sampleRange?: { from: string; to: string };
  actualSampleRange?: { from: string; to: string } | null;
};
export type WorkbenchChart = { chart: ChartWire; execution?: ExecutionWire };
export type IncidentTimelineItem = {
  sequence: number;
  type: string;
  title: string;
  detail?: string;
};
export type WorkbenchState = {
  latestSequence: number;
  assistantText: string;
  taskStatus?: TaskStatus;
  error?: { code: string; message: string };
  charts: Record<string, WorkbenchChart>;
  incidentTimeline: IncidentTimelineItem[];
};

export const initialWorkbenchState: WorkbenchState = { latestSequence: 0, assistantText: '', charts: {}, incidentTimeline: [] };
