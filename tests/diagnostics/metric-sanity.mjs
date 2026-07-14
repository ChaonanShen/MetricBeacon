const limits = {
  cpu: { min: 0, max: 100 },
  memory: { min: 0, max: 100 },
  load: { min: 0, max: Number.POSITIVE_INFINITY },
};

function timestampMillis(value, view) {
  const milliseconds = typeof value === 'number'
    ? value < 1_000_000_000_000 ? value * 1000 : value
    : Date.parse(value);
  if (!Number.isFinite(milliseconds)) {
    throw new Error(`${view}: metric point timestamp was invalid`);
  }
  return milliseconds;
}

export function summarizeMetricSeries(view, series, options = {}) {
  const range = limits[view];
  if (!range) throw new Error(`${view}: metric view was unsupported`);
  if (!Array.isArray(series) || series.length === 0) throw new Error(`${view}: metric result had no series`);
  const maxSeries = options.maxSeries ?? 20;
  const maxSamples = options.maxSamples ?? 5_000;
  if (series.length > maxSeries) throw new Error(`${view}: metric result exceeded ${maxSeries} series`);

  let samples = 0;
  let min = Number.POSITIVE_INFINITY;
  let max = Number.NEGATIVE_INFINITY;
  let latest = Number.NaN;
  let latestTimestamp = Number.NEGATIVE_INFINITY;
  for (const item of series) {
    if (typeof item?.labels?.instance !== 'string' || !item.labels.instance.trim()) {
      throw new Error(`${view}: metric series was missing the instance label`);
    }
    if (!Array.isArray(item.points) || item.points.length === 0) {
      throw new Error(`${view}: metric series had no points`);
    }
    let previousTimestamp = Number.NEGATIVE_INFINITY;
    for (const point of item.points) {
      const timestamp = timestampMillis(point?.timestamp, view);
      if (timestamp <= previousTimestamp) throw new Error(`${view}: metric timestamps were not strictly increasing`);
      previousTimestamp = timestamp;
      const value = Number(point?.value);
      if (!Number.isFinite(value)) throw new Error(`${view}: metric value was not finite`);
      if (value < range.min || value > range.max) {
        const expected = Number.isFinite(range.max) ? `${range.min}..${range.max}` : `>= ${range.min}`;
        throw new Error(`${view}: metric value ${value} was outside ${expected}`);
      }
      samples += 1;
      if (samples > maxSamples) throw new Error(`${view}: metric result exceeded ${maxSamples} samples`);
      min = Math.min(min, value);
      max = Math.max(max, value);
      if (timestamp > latestTimestamp) {
        latestTimestamp = timestamp;
        latest = value;
      }
    }
  }
  return { view, series: series.length, samples, min, max, latest };
}

export function formatMetricSummary(prefix, summary, resultType) {
  const number = (value) => Number(value.toFixed(4));
  const type = resultType ? ` resultType=${resultType}` : '';
  return `[${prefix}] view=${summary.view}${type} series=${summary.series} samples=${summary.samples} min=${number(summary.min)} max=${number(summary.max)} latest=${number(summary.latest)}`;
}
