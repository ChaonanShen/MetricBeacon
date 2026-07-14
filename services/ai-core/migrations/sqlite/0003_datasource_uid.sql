UPDATE tasks
SET datasource_uid = 'prometheus-main'
WHERE datasource_uid = 'mock-prometheus';

UPDATE charts
SET queries_json = (
    SELECT json_group_array(json_set(value, '$.datasourceUid', 'prometheus-main'))
    FROM json_each(charts.queries_json)
)
WHERE EXISTS (
    SELECT 1
    FROM json_each(charts.queries_json)
    WHERE json_extract(value, '$.datasourceUid') = 'mock-prometheus'
);
