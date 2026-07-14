ALTER TABLE tasks
ADD COLUMN views_json TEXT NOT NULL DEFAULT '[]'
CHECK (json_valid(views_json) AND json_type(views_json) = 'array');

UPDATE tasks
SET views_json = json_array(
    CASE WHEN EXISTS (
        SELECT 1 FROM charts c, json_each(c.queries_json) q
        WHERE c.task_id = tasks.id AND c.tenant_id = tasks.tenant_id
          AND coalesce(json_extract(q.value, '$.Expression'), json_extract(q.value, '$.expression')) LIKE '%node_cpu_seconds_total%'
    ) THEN 'cpu' END,
    CASE WHEN EXISTS (
        SELECT 1 FROM charts c, json_each(c.queries_json) q
        WHERE c.task_id = tasks.id AND c.tenant_id = tasks.tenant_id
          AND coalesce(json_extract(q.value, '$.Expression'), json_extract(q.value, '$.expression')) LIKE '%node_memory_MemAvailable_bytes%'
    ) THEN 'memory' END,
    CASE WHEN EXISTS (
        SELECT 1 FROM charts c, json_each(c.queries_json) q
        WHERE c.task_id = tasks.id AND c.tenant_id = tasks.tenant_id
          AND coalesce(json_extract(q.value, '$.Expression'), json_extract(q.value, '$.expression')) LIKE '%node_load1%'
    ) THEN 'load' END
);

UPDATE tasks
SET views_json = (
    SELECT json_group_array(value)
    FROM json_each(tasks.views_json)
    WHERE value IS NOT NULL
);
