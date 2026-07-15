PRAGMA defer_foreign_keys = ON;

ALTER TABLE sessions ADD COLUMN org_id TEXT NOT NULL DEFAULT 'legacy' CHECK (length(org_id) > 0);
ALTER TABLE sessions ADD COLUMN kind TEXT NOT NULL DEFAULT 'private' CHECK (kind IN ('private', 'org_incident'));
UPDATE sessions
SET org_id = CASE WHEN instr(tenant_id, ':') > 0 THEN substr(tenant_id, instr(tenant_id, ':') + 1) ELSE tenant_id END;

DROP TRIGGER messages_task_required_before_insert;
DROP TRIGGER messages_assistant_task_scope_before_insert;
DROP TRIGGER tasks_input_message_consistency_before_insert;
DROP TRIGGER tasks_input_message_consistency_before_update;

CREATE TABLE tasks_v7 (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('metric_analysis', 'incident_remediation')),
    session_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('created', 'planning', 'running_tools', 'waiting_approval', 'executing', 'reconciling', 'validating', 'completed', 'failed', 'cancelled')),
    input_message_id TEXT NOT NULL,
    datasource_uid TEXT,
    time_from TEXT,
    time_to TEXT,
    views_json TEXT CHECK (views_json IS NULL OR (json_valid(views_json) AND json_type(views_json) = 'array')),
    step_seconds INTEGER CHECK (step_seconds IS NULL OR step_seconds IN (5, 10, 15, 30, 60, 120, 300)),
    cpu_rate_window_seconds INTEGER CHECK (cpu_rate_window_seconds IS NULL OR cpu_rate_window_seconds IN (30, 60, 300)),
    incident_plan_json TEXT CHECK (incident_plan_json IS NULL OR (json_valid(incident_plan_json) AND json_type(incident_plan_json) = 'object')),
    latest_sequence INTEGER NOT NULL CHECK (latest_sequence >= 0),
    error_code TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    updated_at TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1),
    CHECK (
        (kind = 'metric_analysis' AND datasource_uid IS NOT NULL AND time_from IS NOT NULL AND time_to IS NOT NULL AND time_from < time_to AND views_json IS NOT NULL AND step_seconds IS NOT NULL AND cpu_rate_window_seconds IS NOT NULL AND incident_plan_json IS NULL)
        OR
        (kind = 'incident_remediation' AND datasource_uid IS NULL AND time_from IS NULL AND time_to IS NULL AND views_json IS NULL AND step_seconds IS NULL AND cpu_rate_window_seconds IS NULL AND incident_plan_json IS NOT NULL)
    ),
    FOREIGN KEY (session_id) REFERENCES sessions (id),
    FOREIGN KEY (input_message_id) REFERENCES messages_v7 (id) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE messages_v7 (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'trigger')),
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions (id),
    FOREIGN KEY (task_id) REFERENCES tasks_v7 (id) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE task_events_v7 (
    event_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence >= 1),
    type TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    UNIQUE (tenant_id, task_id, sequence),
    FOREIGN KEY (task_id) REFERENCES tasks_v7 (id)
);

CREATE TABLE tool_calls_v7 (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    tool_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('started', 'completed', 'failed')),
    input_summary_json TEXT NOT NULL CHECK (json_valid(input_summary_json)),
    output_summary_json TEXT CHECK (output_summary_json IS NULL OR json_valid(output_summary_json)),
    error_code TEXT,
    error_message TEXT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
    version INTEGER NOT NULL CHECK (version >= 1),
    source_call_id TEXT NOT NULL,
    FOREIGN KEY (task_id) REFERENCES tasks_v7 (id)
);

CREATE TABLE charts_v7 (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    title TEXT NOT NULL,
    visualization TEXT NOT NULL CHECK (visualization = 'timeseries'),
    unit TEXT NOT NULL,
    queries_json TEXT NOT NULL CHECK (json_valid(queries_json)),
    status TEXT NOT NULL CHECK (status IN ('proposed', 'ready')),
    latest_execution_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1),
    FOREIGN KEY (session_id) REFERENCES sessions (id),
    FOREIGN KEY (task_id) REFERENCES tasks_v7 (id)
);

CREATE TABLE chart_executions_v7 (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    chart_id TEXT NOT NULL,
    query_ref_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('success', 'failed')),
    series_count INTEGER NOT NULL CHECK (series_count >= 0),
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0),
    sample_from TEXT NOT NULL,
    sample_to TEXT NOT NULL,
    series_json TEXT NOT NULL CHECK (json_valid(series_json)),
    warnings_json TEXT NOT NULL CHECK (json_valid(warnings_json)),
    created_at TEXT NOT NULL,
    actual_sample_from TEXT,
    actual_sample_to TEXT,
    CHECK (sample_from < sample_to),
    FOREIGN KEY (chart_id) REFERENCES charts_v7 (id)
);

INSERT INTO tasks_v7 (id, tenant_id, kind, session_id, status, input_message_id, datasource_uid, time_from, time_to, views_json, step_seconds, cpu_rate_window_seconds, incident_plan_json, latest_sequence, error_code, error_message, created_at, started_at, completed_at, updated_at, version)
SELECT id, tenant_id, 'metric_analysis', session_id, status, input_message_id, datasource_uid, time_from, time_to, views_json, step_seconds, cpu_rate_window_seconds, NULL, latest_sequence, error_code, error_message, created_at, started_at, completed_at, updated_at, version FROM tasks;
INSERT INTO messages_v7 SELECT id, tenant_id, session_id, task_id, role, content, created_at FROM messages;
INSERT INTO task_events_v7 SELECT event_id, tenant_id, task_id, session_id, sequence, type, timestamp, payload_json FROM task_events;
INSERT INTO tool_calls_v7 (id, tenant_id, task_id, tool_name, tool_version, status, input_summary_json, output_summary_json, error_code, error_message, started_at, completed_at, duration_ms, version, source_call_id)
SELECT id, tenant_id, task_id, tool_name, tool_version, status, input_summary_json, output_summary_json, error_code, error_message, started_at, completed_at, duration_ms, version, source_call_id FROM tool_calls;
INSERT INTO charts_v7 SELECT id, tenant_id, session_id, task_id, title, visualization, unit, queries_json, status, latest_execution_id, created_at, updated_at, version FROM charts;
INSERT INTO chart_executions_v7 (id, tenant_id, chart_id, query_ref_id, status, series_count, duration_ms, sample_from, sample_to, series_json, warnings_json, created_at, actual_sample_from, actual_sample_to)
SELECT id, tenant_id, chart_id, query_ref_id, status, series_count, duration_ms, sample_from, sample_to, series_json, warnings_json, created_at, actual_sample_from, actual_sample_to FROM chart_executions;

DROP TABLE chart_executions;
DROP TABLE charts;
DROP TABLE tool_calls;
DROP TABLE task_events;
DROP TABLE messages;
DROP TABLE tasks;

ALTER TABLE tasks_v7 RENAME TO tasks;
ALTER TABLE messages_v7 RENAME TO messages;
ALTER TABLE task_events_v7 RENAME TO task_events;
ALTER TABLE tool_calls_v7 RENAME TO tool_calls;
ALTER TABLE charts_v7 RENAME TO charts;
ALTER TABLE chart_executions_v7 RENAME TO chart_executions;

CREATE INDEX sessions_tenant_org_kind_updated_id_idx ON sessions (tenant_id, org_id, kind, updated_at DESC, id DESC);
CREATE INDEX messages_tenant_session_created_at_idx ON messages (tenant_id, session_id, created_at);
CREATE INDEX messages_tenant_session_created_id_idx ON messages (tenant_id, session_id, created_at, id);
CREATE UNIQUE INDEX messages_tenant_task_role_idx ON messages (tenant_id, task_id, role);
CREATE INDEX tasks_tenant_session_created_at_idx ON tasks (tenant_id, session_id, created_at);
CREATE INDEX tasks_tenant_session_created_id_idx ON tasks (tenant_id, session_id, created_at, id);
CREATE UNIQUE INDEX tasks_tenant_input_message_idx ON tasks (tenant_id, input_message_id);
CREATE UNIQUE INDEX tasks_one_active_per_session_idx ON tasks (tenant_id, session_id) WHERE status NOT IN ('completed', 'failed', 'cancelled');
CREATE UNIQUE INDEX tasks_active_incident_fingerprint_idx ON tasks (tenant_id, json_extract(incident_plan_json, '$.sourceId'), json_extract(incident_plan_json, '$.alertFingerprint')) WHERE kind = 'incident_remediation' AND status NOT IN ('completed', 'failed', 'cancelled');
CREATE INDEX task_events_replay_idx ON task_events (tenant_id, task_id, sequence);
CREATE INDEX tool_calls_tenant_task_started_at_idx ON tool_calls (tenant_id, task_id, started_at);
CREATE UNIQUE INDEX tool_calls_tenant_task_source_call_idx ON tool_calls (tenant_id, task_id, source_call_id);
CREATE INDEX charts_tenant_task_idx ON charts (tenant_id, task_id);
CREATE INDEX chart_executions_tenant_chart_created_at_idx ON chart_executions (tenant_id, chart_id, created_at);

CREATE TRIGGER messages_task_required_before_insert
BEFORE INSERT ON messages
FOR EACH ROW WHEN NEW.task_id IS NULL OR NEW.task_id = ''
BEGIN SELECT RAISE(ABORT, 'message task_id is required'); END;

CREATE TRIGGER messages_task_scope_before_insert
BEFORE INSERT ON messages
FOR EACH ROW WHEN NEW.role = 'assistant' AND NOT EXISTS (
    SELECT 1 FROM tasks t WHERE t.id = NEW.task_id AND t.tenant_id = NEW.tenant_id AND t.session_id = NEW.session_id
)
BEGIN SELECT RAISE(ABORT, 'message task scope is invalid'); END;

CREATE TRIGGER tasks_input_message_consistency_before_insert
BEFORE INSERT ON tasks
FOR EACH ROW WHEN NOT EXISTS (
    SELECT 1 FROM messages m WHERE m.id = NEW.input_message_id AND m.tenant_id = NEW.tenant_id AND m.session_id = NEW.session_id AND m.task_id = NEW.id
      AND ((NEW.kind = 'metric_analysis' AND m.role = 'user') OR (NEW.kind = 'incident_remediation' AND m.role = 'trigger'))
)
BEGIN SELECT RAISE(ABORT, 'task input message is invalid'); END;

CREATE TRIGGER tasks_input_message_consistency_before_update
BEFORE UPDATE OF input_message_id, tenant_id, session_id, kind ON tasks
FOR EACH ROW WHEN NOT EXISTS (
    SELECT 1 FROM messages m WHERE m.id = NEW.input_message_id AND m.tenant_id = NEW.tenant_id AND m.session_id = NEW.session_id AND m.task_id = NEW.id
      AND ((NEW.kind = 'metric_analysis' AND m.role = 'user') OR (NEW.kind = 'incident_remediation' AND m.role = 'trigger'))
)
BEGIN SELECT RAISE(ABORT, 'task input message is invalid'); END;

CREATE TABLE task_checkpoints (
    task_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    opaque_value TEXT NOT NULL CHECK (length(opaque_value) BETWEEN 1 AND 16384),
    updated_at TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1),
    FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE
);
CREATE INDEX task_checkpoints_tenant_task_idx ON task_checkpoints (tenant_id, task_id);

CREATE TABLE alert_events (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    starts_at TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('firing', 'resolved')),
    service_ref TEXT NOT NULL,
    alert_name TEXT NOT NULL,
    labels_json TEXT NOT NULL CHECK (json_valid(labels_json) AND json_type(labels_json) = 'object'),
    task_id TEXT,
    received_at TEXT NOT NULL,
    UNIQUE (tenant_id, org_id, source_id, fingerprint, starts_at, status),
    FOREIGN KEY (task_id) REFERENCES tasks (id)
);
CREATE INDEX alert_events_task_idx ON alert_events (tenant_id, task_id, received_at);
