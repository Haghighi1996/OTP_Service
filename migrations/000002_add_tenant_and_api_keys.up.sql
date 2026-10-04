ALTER TABLE otps ADD COLUMN tenant_id BIGINT NOT NULL DEFAULT 1;

CREATE INDEX idx_otps_tenant_id ON otps (tenant_id);

CREATE TABLE api_keys (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    key_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT api_keys_key_hash_not_blank CHECK (btrim(key_hash) <> ''),
    CONSTRAINT api_keys_expires_valid CHECK (expires_at IS NULL OR expires_at > created_at)
);

CREATE INDEX idx_api_keys_tenant_id ON api_keys (tenant_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys (key_hash);
