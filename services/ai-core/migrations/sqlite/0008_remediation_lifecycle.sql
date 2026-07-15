CREATE TABLE remediation_intents (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    digest TEXT NOT NULL CHECK (digest GLOB 'sha256:[0-9a-f]*' AND substr(digest, 8) NOT GLOB '*[^0-9a-f]*' AND length(digest) = 71),
    capability_id TEXT NOT NULL CHECK (capability_id = 'order_service.restore_worker_concurrency'),
    service_ref TEXT NOT NULL,
    instance_epoch TEXT NOT NULL,
    expected_version INTEGER NOT NULL CHECK (expected_version >= 1),
    before_concurrency INTEGER NOT NULL CHECK (before_concurrency = 0),
    after_concurrency INTEGER NOT NULL CHECK (after_concurrency = 2),
    risk TEXT NOT NULL CHECK (risk = 'low'),
    created_at TEXT NOT NULL,
    UNIQUE (tenant_id, task_id),
    FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE
);

CREATE TRIGGER remediation_intents_scope_before_insert
BEFORE INSERT ON remediation_intents
FOR EACH ROW WHEN NOT EXISTS (
    SELECT 1 FROM tasks t JOIN sessions s ON s.id = t.session_id
    WHERE t.id = NEW.task_id AND t.tenant_id = NEW.tenant_id AND t.kind = 'incident_remediation'
      AND json_extract(t.incident_plan_json, '$.serviceRef') = NEW.service_ref
      AND s.tenant_id = NEW.tenant_id AND s.org_id = NEW.org_id AND s.kind = 'org_incident'
)
BEGIN SELECT RAISE(ABORT, 'remediation intent scope is invalid'); END;

CREATE TABLE approvals (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    intent_id TEXT NOT NULL,
    intent_digest TEXT NOT NULL CHECK (intent_digest GLOB 'sha256:[0-9a-f]*' AND substr(intent_digest, 8) NOT GLOB '*[^0-9a-f]*' AND length(intent_digest) = 71),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    requested_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    decided_at TEXT,
    decided_by TEXT,
    decision_reason TEXT CHECK (decision_reason IS NULL OR length(decision_reason) <= 500),
    version INTEGER NOT NULL CHECK (version >= 1),
    CHECK (requested_at < expires_at),
    CHECK ((status = 'pending' AND version = 1 AND decided_at IS NULL AND decided_by IS NULL AND decision_reason IS NULL)
        OR (status IN ('approved', 'rejected') AND version = 2 AND decided_at IS NOT NULL AND decided_at < expires_at AND decided_by IS NOT NULL)
        OR (status = 'expired' AND version = 2 AND decided_at IS NOT NULL AND decided_at >= expires_at AND decided_by IS NOT NULL)),
    UNIQUE (tenant_id, task_id),
    FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE,
    FOREIGN KEY (intent_id) REFERENCES remediation_intents (id) ON DELETE CASCADE
);

CREATE TRIGGER approvals_scope_before_insert
BEFORE INSERT ON approvals
FOR EACH ROW WHEN NOT EXISTS (
    SELECT 1 FROM remediation_intents i WHERE i.id = NEW.intent_id AND i.tenant_id = NEW.tenant_id
      AND i.org_id = NEW.org_id AND i.task_id = NEW.task_id AND i.digest = NEW.intent_digest
)
BEGIN SELECT RAISE(ABORT, 'approval scope is invalid'); END;

CREATE TRIGGER approvals_scope_before_update
BEFORE UPDATE ON approvals
FOR EACH ROW WHEN NEW.tenant_id != OLD.tenant_id OR NEW.org_id != OLD.org_id OR NEW.task_id != OLD.task_id
    OR NEW.intent_id != OLD.intent_id OR NEW.intent_digest != OLD.intent_digest OR NEW.requested_at != OLD.requested_at
    OR NEW.expires_at != OLD.expires_at
BEGIN SELECT RAISE(ABORT, 'approval immutable scope changed'); END;

CREATE TABLE remediation_executions (
    operation_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    approval_id TEXT NOT NULL,
    intent_digest TEXT NOT NULL CHECK (intent_digest GLOB 'sha256:[0-9a-f]*' AND substr(intent_digest, 8) NOT GLOB '*[^0-9a-f]*' AND length(intent_digest) = 71),
    instance_epoch TEXT NOT NULL,
    expected_version INTEGER NOT NULL CHECK (expected_version >= 1),
    state TEXT NOT NULL CHECK (state IN ('started', 'applied', 'already_applied', 'failed', 'unknown')),
    before_version INTEGER,
    after_version INTEGER,
    error_code TEXT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    version INTEGER NOT NULL CHECK (version >= 1),
    CHECK ((state = 'started' AND version = 1 AND before_version IS NULL AND after_version IS NULL AND error_code IS NULL AND completed_at IS NULL)
        OR (state IN ('applied', 'already_applied') AND version IN (2, 3) AND before_version = expected_version AND after_version = before_version + 1 AND error_code IS NULL AND completed_at IS NOT NULL)
        OR (state = 'unknown' AND version = 2 AND before_version IS NULL AND after_version IS NULL AND error_code IS NULL AND completed_at IS NOT NULL)
        OR (state = 'failed' AND version = 2 AND before_version IS NULL AND after_version IS NULL AND error_code IS NOT NULL AND completed_at IS NOT NULL)),
    UNIQUE (tenant_id, task_id),
    FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE,
    FOREIGN KEY (approval_id) REFERENCES approvals (id)
);

CREATE TRIGGER remediation_executions_scope_before_insert
BEFORE INSERT ON remediation_executions
FOR EACH ROW WHEN NOT EXISTS (
    SELECT 1 FROM approvals a JOIN remediation_intents i ON i.id = a.intent_id
    WHERE a.id = NEW.approval_id AND a.tenant_id = NEW.tenant_id AND a.org_id = NEW.org_id
      AND a.task_id = NEW.task_id AND a.intent_digest = NEW.intent_digest AND a.status = 'approved'
      AND i.instance_epoch = NEW.instance_epoch AND i.expected_version = NEW.expected_version
)
BEGIN SELECT RAISE(ABORT, 'remediation execution scope is invalid'); END;

CREATE TRIGGER remediation_executions_scope_before_update
BEFORE UPDATE ON remediation_executions
FOR EACH ROW WHEN NEW.tenant_id != OLD.tenant_id OR NEW.org_id != OLD.org_id OR NEW.task_id != OLD.task_id
    OR NEW.approval_id != OLD.approval_id OR NEW.intent_digest != OLD.intent_digest
    OR NEW.instance_epoch != OLD.instance_epoch OR NEW.expected_version != OLD.expected_version OR NEW.started_at != OLD.started_at
BEGIN SELECT RAISE(ABORT, 'remediation execution immutable scope changed'); END;

CREATE TABLE audit_records (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('approval_decision', 'remediation_execute', 'remediation_verify')),
    outcome TEXT NOT NULL CHECK (outcome IN ('accepted', 'rejected', 'succeeded', 'failed')),
    summary TEXT NOT NULL CHECK (length(summary) BETWEEN 1 AND 500),
    occurred_at TEXT NOT NULL,
    FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE
);

CREATE TRIGGER audit_records_scope_before_insert
BEFORE INSERT ON audit_records
FOR EACH ROW WHEN NOT EXISTS (
    SELECT 1 FROM tasks t JOIN sessions s ON s.id = t.session_id
    WHERE t.id = NEW.task_id AND t.tenant_id = NEW.tenant_id AND s.org_id = NEW.org_id
)
BEGIN SELECT RAISE(ABORT, 'audit record scope is invalid'); END;

CREATE INDEX remediation_intents_task_idx ON remediation_intents (tenant_id, task_id);
CREATE INDEX approvals_task_idx ON approvals (tenant_id, task_id);
CREATE INDEX remediation_executions_task_idx ON remediation_executions (tenant_id, task_id);
CREATE INDEX audit_records_task_idx ON audit_records (tenant_id, task_id, occurred_at, id);
