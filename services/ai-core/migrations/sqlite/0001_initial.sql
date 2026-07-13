CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active')),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1)
);

CREATE INDEX IF NOT EXISTS sessions_tenant_updated_at_idx
    ON sessions (tenant_id, updated_at);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions (id)
);

CREATE INDEX IF NOT EXISTS messages_tenant_session_created_at_idx
    ON messages (tenant_id, session_id, created_at);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('created', 'planning', 'running_tools', 'validating', 'completed', 'failed')),
    input_message_id TEXT NOT NULL,
    datasource_uid TEXT NOT NULL,
    time_from TEXT NOT NULL,
    time_to TEXT NOT NULL,
    latest_sequence INTEGER NOT NULL CHECK (latest_sequence >= 0),
    error_code TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    updated_at TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1),
    CHECK (time_from < time_to),
    FOREIGN KEY (session_id) REFERENCES sessions (id),
    FOREIGN KEY (input_message_id) REFERENCES messages (id)
);

CREATE INDEX IF NOT EXISTS tasks_tenant_session_created_at_idx
    ON tasks (tenant_id, session_id, created_at);

CREATE TABLE IF NOT EXISTS task_events (
    event_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence >= 1),
    type TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    UNIQUE (tenant_id, task_id, sequence),
    FOREIGN KEY (task_id) REFERENCES tasks (id)
);

CREATE INDEX IF NOT EXISTS task_events_replay_idx
    ON task_events (tenant_id, task_id, sequence);

CREATE TABLE IF NOT EXISTS tool_calls (
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
    FOREIGN KEY (task_id) REFERENCES tasks (id)
);

CREATE INDEX IF NOT EXISTS tool_calls_tenant_task_started_at_idx
    ON tool_calls (tenant_id, task_id, started_at);

CREATE TABLE IF NOT EXISTS charts (
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
    FOREIGN KEY (task_id) REFERENCES tasks (id)
);

CREATE INDEX IF NOT EXISTS charts_tenant_task_idx
    ON charts (tenant_id, task_id);

CREATE TABLE IF NOT EXISTS chart_executions (
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
    CHECK (sample_from < sample_to),
    FOREIGN KEY (chart_id) REFERENCES charts (id)
);

CREATE INDEX IF NOT EXISTS chart_executions_tenant_chart_created_at_idx
    ON chart_executions (tenant_id, chart_id, created_at);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    tenant_id TEXT NOT NULL,
    scope TEXT NOT NULL,
    key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('reserved', 'completed')),
    resource_id TEXT,
    response_json TEXT,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id, scope, key)
);
