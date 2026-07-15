CREATE INDEX IF NOT EXISTS sessions_tenant_creator_updated_id_idx
    ON sessions (tenant_id, created_by, updated_at DESC, id DESC);
