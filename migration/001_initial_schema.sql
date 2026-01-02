-- Migration: Initial Schema
-- Create all base tables

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ================================
-- USERS
-- ================================
CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  uuid UUID DEFAULT gen_random_uuid(),
  email VARCHAR(255) NOT NULL UNIQUE,
  password_hash TEXT,
  full_name VARCHAR(255),
  is_active BOOLEAN DEFAULT FALSE,
  is_invited BOOLEAN DEFAULT FALSE,
  invited_at TIMESTAMP,
  last_login TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ================================
-- ROLES
-- ================================
CREATE TABLE IF NOT EXISTS roles (
  id SERIAL PRIMARY KEY,
  uuid UUID DEFAULT gen_random_uuid(),
  name VARCHAR(50) UNIQUE NOT NULL,
  description TEXT
);

-- ================================
-- USER_ROLES (M2M)
-- ================================
CREATE TABLE IF NOT EXISTS user_roles (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  UNIQUE(user_id, role_id)
);

-- ================================
-- PERMISSIONS
-- ================================
CREATE TABLE IF NOT EXISTS permissions (
  id SERIAL PRIMARY KEY,
  name VARCHAR(100) UNIQUE NOT NULL,
  description TEXT
);

-- ================================
-- ROLE_PERMISSIONS (M2M)
-- ================================
CREATE TABLE IF NOT EXISTS role_permissions (
  id SERIAL PRIMARY KEY,
  role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  UNIQUE(role_id, permission_id)
);

-- ================================
-- OTP_CODES
-- ================================
CREATE TABLE IF NOT EXISTS otp_codes (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  otp_code VARCHAR(10) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  is_used BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_otp_user_id ON otp_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_otp_code ON otp_codes(otp_code);

-- ================================
-- INVITE_TOKENS
-- ================================
CREATE TABLE IF NOT EXISTS invite_tokens (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token UUID NOT NULL UNIQUE,
  expires_at TIMESTAMP NOT NULL,
  is_used BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ================================
-- SEED: Default Roles
-- ================================
INSERT INTO roles (name) VALUES ('end_user') ON CONFLICT (name) DO NOTHING;
INSERT INTO roles (name) VALUES ('admin') ON CONFLICT (name) DO NOTHING;
INSERT INTO roles (name) VALUES ('super_admin') ON CONFLICT (name) DO NOTHING;
