DROP TABLE api_keys;
ALTER TABLE otps DROP COLUMN IF EXISTS tenant_id;
DROP INDEX IF EXISTS idx_otps_tenant_id;
DROP INDEX IF EXISTS idx_api_keys_tenant_id;
DROP INDEX IF EXISTS idx_api_keys_key_hash;
