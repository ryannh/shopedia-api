-- Migration: Create revoked_tokens table for JWT revocation
-- Run this migration after users table exists

CREATE TABLE IF NOT EXISTS revoked_tokens (
    id SERIAL PRIMARY KEY,
    jti UUID NOT NULL UNIQUE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revoked_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Index untuk lookup cepat saat validasi token
CREATE INDEX IF NOT EXISTS idx_revoked_tokens_jti ON revoked_tokens(jti);

-- Index untuk cleanup expired tokens
CREATE INDEX IF NOT EXISTS idx_revoked_tokens_expires_at ON revoked_tokens(expires_at);

-- Index untuk revoke semua token user
CREATE INDEX IF NOT EXISTS idx_revoked_tokens_user_id ON revoked_tokens(user_id);

COMMENT ON TABLE revoked_tokens IS 'Menyimpan JTI dari token yang sudah di-revoke untuk logout/invalidation';
COMMENT ON COLUMN revoked_tokens.jti IS 'JWT ID - unique identifier dari token';
COMMENT ON COLUMN revoked_tokens.expires_at IS 'Waktu expired token, untuk auto-cleanup';
