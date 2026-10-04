CREATE TABLE tenants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tenants_name_not_blank CHECK (btrim(name) <> '')
);

-- Add tenant_id to otps (already added in migration 2, but add FK)
ALTER TABLE otps 
    ADD CONSTRAINT otps_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id);

-- Add tenant_id to api_keys (already added in migration 2, but add FK)
ALTER TABLE api_keys
    ADD CONSTRAINT api_keys_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id);
