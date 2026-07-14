ALTER TABLE tasks
ADD COLUMN step_seconds INTEGER NOT NULL DEFAULT 300
CHECK (step_seconds IN (5, 10, 15, 30, 60, 120, 300));

ALTER TABLE tasks
ADD COLUMN cpu_rate_window_seconds INTEGER NOT NULL DEFAULT 300
CHECK (cpu_rate_window_seconds IN (30, 60, 300));

ALTER TABLE chart_executions
ADD COLUMN actual_sample_from TEXT;

ALTER TABLE chart_executions
ADD COLUMN actual_sample_to TEXT;

UPDATE charts
SET queries_json = (
    SELECT json_group_array(json_set(value, '$.StepSeconds', 300))
    FROM json_each(charts.queries_json)
);
