ALTER TABLE messages
    ADD COLUMN task_id TEXT REFERENCES tasks(id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE tool_calls ADD COLUMN source_call_id TEXT;

UPDATE messages
SET task_id = (
    SELECT t.id FROM tasks t
    WHERE t.tenant_id = messages.tenant_id
      AND t.session_id = messages.session_id
      AND t.input_message_id = messages.id
)
WHERE role = 'user';

UPDATE messages
SET task_id = (
    SELECT e.task_id FROM task_events e
    WHERE e.tenant_id = messages.tenant_id
      AND e.type = 'assistant.message.completed'
      AND json_extract(e.payload_json, '$.message.id') = messages.id
)
WHERE role = 'assistant';

UPDATE task_events
SET payload_json = json_set(payload_json, '$.message.taskId', (
    SELECT m.task_id FROM messages m
    WHERE m.tenant_id = task_events.tenant_id
      AND m.id = json_extract(task_events.payload_json, '$.message.id')
))
WHERE type = 'assistant.message.completed';

UPDATE tool_calls SET source_call_id = 'legacy:' || id;

CREATE UNIQUE INDEX messages_tenant_task_role_idx
    ON messages (tenant_id, task_id, role);
CREATE UNIQUE INDEX tasks_tenant_input_message_idx
    ON tasks (tenant_id, input_message_id);
CREATE UNIQUE INDEX tool_calls_tenant_task_source_call_idx
    ON tool_calls (tenant_id, task_id, source_call_id);
CREATE UNIQUE INDEX tasks_one_active_per_session_idx
    ON tasks (tenant_id, session_id)
    WHERE status NOT IN ('completed', 'failed');
CREATE INDEX messages_tenant_session_created_id_idx
    ON messages (tenant_id, session_id, created_at, id);
CREATE INDEX tasks_tenant_session_created_id_idx
    ON tasks (tenant_id, session_id, created_at, id);

CREATE TRIGGER messages_task_required_before_insert
BEFORE INSERT ON messages
FOR EACH ROW WHEN NEW.task_id IS NULL OR NEW.task_id = ''
BEGIN
    SELECT RAISE(ABORT, 'message task_id is required');
END;

CREATE TRIGGER messages_assistant_task_scope_before_insert
BEFORE INSERT ON messages
FOR EACH ROW WHEN NEW.role = 'assistant' AND NOT EXISTS (
    SELECT 1 FROM tasks t
    WHERE t.id = NEW.task_id AND t.tenant_id = NEW.tenant_id AND t.session_id = NEW.session_id
)
BEGIN
    SELECT RAISE(ABORT, 'assistant message task scope is invalid');
END;

CREATE TRIGGER tasks_input_message_consistency_before_insert
BEFORE INSERT ON tasks
FOR EACH ROW WHEN NOT EXISTS (
    SELECT 1 FROM messages m
    WHERE m.id = NEW.input_message_id
      AND m.tenant_id = NEW.tenant_id
      AND m.session_id = NEW.session_id
      AND m.task_id = NEW.id
      AND m.role = 'user'
)
BEGIN
    SELECT RAISE(ABORT, 'task input message is invalid');
END;

CREATE TRIGGER tasks_input_message_consistency_before_update
BEFORE UPDATE OF input_message_id, tenant_id, session_id ON tasks
FOR EACH ROW WHEN NOT EXISTS (
    SELECT 1 FROM messages m
    WHERE m.id = NEW.input_message_id
      AND m.tenant_id = NEW.tenant_id
      AND m.session_id = NEW.session_id
      AND m.task_id = NEW.id
      AND m.role = 'user'
)
BEGIN
    SELECT RAISE(ABORT, 'task input message is invalid');
END;
