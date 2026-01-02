-- Migration: Add is_banned column to users table
-- For banning end_user accounts

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_banned BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_banned ON users(is_banned);
