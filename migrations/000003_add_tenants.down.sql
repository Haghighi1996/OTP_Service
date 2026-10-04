ALTER TABLE otps DROP CONSTRAINT IF EXISTS otps_tenant_id_fkey;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_tenant_id_fkey;
DROP TABLE tenants;
