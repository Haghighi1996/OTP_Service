-- OTP Service database schema
--
-- This file is the Phase 2.2 schema contract. Phase 2.3 will introduce a
-- migration tool and convert this DDL into versioned migrations.

CREATE TABLE otps (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    phone_number VARCHAR(32) NOT NULL,
    otp_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT otps_phone_number_not_blank CHECK (btrim(phone_number) <> ''),
    CONSTRAINT otps_otp_hash_not_blank CHECK (btrim(otp_hash) <> ''),
    CONSTRAINT otps_expiry_after_creation CHECK (expires_at > created_at),
    CONSTRAINT otps_used_after_creation CHECK (used_at IS NULL OR used_at >= created_at)
);

-- Supports the send and verify flows: locate the newest unused OTP for a
-- phone number, then let application queries reject expired records.
CREATE INDEX idx_otps_phone_number_active_created_at
    ON otps (phone_number, created_at DESC)
    WHERE used_at IS NULL;

-- Supports scheduled retention cleanup without slowing active OTP lookups.
CREATE INDEX idx_otps_expires_at ON otps (expires_at);
