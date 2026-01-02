-- Migration: Active Sessions for Single Active Session feature
-- Hanya 1 session aktif per user, login baru akan invalidate session lama

CREATE TABLE IF NOT EXISTS active_sessions (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    jti UUID NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_active_sessions_user_id ON active_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_active_sessions_jti ON active_sessions(jti);

COMMENT ON TABLE active_sessions IS 'Menyimpan JTI session aktif per user untuk Single Active Session';
