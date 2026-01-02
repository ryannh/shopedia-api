-- Migration: Roles & Permissions Enhancement
-- Add uuid, soft delete, scope, and seed functional roles with permissions

-- ================================
-- UPDATE ROLES TABLE
-- ================================
ALTER TABLE roles ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT gen_random_uuid();
ALTER TABLE roles ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS scope VARCHAR(20) DEFAULT 'dashboard'; -- 'app' or 'dashboard'
ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_system BOOLEAN DEFAULT FALSE;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP DEFAULT NULL;

CREATE INDEX IF NOT EXISTS idx_roles_uuid ON roles(uuid);
CREATE INDEX IF NOT EXISTS idx_roles_scope ON roles(scope);
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles(deleted_at);

-- ================================
-- UPDATE PERMISSIONS TABLE
-- ================================
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT gen_random_uuid();
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS module VARCHAR(50); -- e.g., 'finance', 'user', 'product'
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP DEFAULT NULL;

CREATE INDEX IF NOT EXISTS idx_permissions_uuid ON permissions(uuid);
CREATE INDEX IF NOT EXISTS idx_permissions_module ON permissions(module);
CREATE INDEX IF NOT EXISTS idx_permissions_deleted_at ON permissions(deleted_at);

-- ================================
-- UPDATE USER_ROLES TABLE
-- ================================
ALTER TABLE user_roles ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT gen_random_uuid();
ALTER TABLE user_roles ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE user_roles ADD COLUMN IF NOT EXISTS assigned_by INTEGER REFERENCES users(id);

CREATE INDEX IF NOT EXISTS idx_user_roles_uuid ON user_roles(uuid);

-- ================================
-- UPDATE ROLE_PERMISSIONS TABLE
-- ================================
ALTER TABLE role_permissions ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT gen_random_uuid();
ALTER TABLE role_permissions ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_role_permissions_uuid ON role_permissions(uuid);

-- ================================
-- UPDATE EXISTING ROLES
-- ================================
UPDATE roles SET
  description = 'Super Administrator with full system access',
  scope = 'dashboard',
  is_system = TRUE,
  updated_at = NOW()
WHERE name = 'super_admin';

UPDATE roles SET
  description = 'Internal staff with dashboard access',
  scope = 'dashboard',
  is_system = TRUE,
  updated_at = NOW()
WHERE name = 'admin';

UPDATE roles SET
  description = 'Regular application user',
  scope = 'app',
  is_system = TRUE,
  updated_at = NOW()
WHERE name = 'end_user';

-- ================================
-- SEED: New Roles
-- ================================
INSERT INTO roles (name, description, scope, is_system) VALUES
  ('seller', 'End user with store/seller capabilities', 'app', TRUE)
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  scope = EXCLUDED.scope,
  is_system = EXCLUDED.is_system,
  updated_at = NOW();

INSERT INTO roles (name, description, scope, is_system) VALUES
  ('finance', 'Finance team with financial data access', 'dashboard', TRUE)
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  scope = EXCLUDED.scope,
  is_system = EXCLUDED.is_system,
  updated_at = NOW();

INSERT INTO roles (name, description, scope, is_system) VALUES
  ('support', 'Customer support team', 'dashboard', TRUE)
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  scope = EXCLUDED.scope,
  is_system = EXCLUDED.is_system,
  updated_at = NOW();

INSERT INTO roles (name, description, scope, is_system) VALUES
  ('ops', 'Operations team for product moderation', 'dashboard', TRUE)
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  scope = EXCLUDED.scope,
  is_system = EXCLUDED.is_system,
  updated_at = NOW();

INSERT INTO roles (name, description, scope, is_system) VALUES
  ('marketing', 'Marketing team for promos and banners', 'dashboard', TRUE)
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  scope = EXCLUDED.scope,
  is_system = EXCLUDED.is_system,
  updated_at = NOW();

-- ================================
-- SEED: Permissions
-- ================================

-- Finance permissions
INSERT INTO permissions (name, description, module) VALUES
  ('finance.view', 'View financial data', 'finance'),
  ('finance.export', 'Export financial reports', 'finance'),
  ('finance.refund', 'Process refunds', 'finance'),
  ('finance.payout', 'Process seller payouts', 'finance')
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  module = EXCLUDED.module,
  updated_at = NOW();

-- Support permissions
INSERT INTO permissions (name, description, module) VALUES
  ('support.view', 'View support tickets', 'support'),
  ('support.respond', 'Respond to tickets', 'support'),
  ('support.escalate', 'Escalate tickets', 'support'),
  ('support.close', 'Close tickets', 'support')
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  module = EXCLUDED.module,
  updated_at = NOW();

-- Product/Ops permissions
INSERT INTO permissions (name, description, module) VALUES
  ('product.view', 'View products', 'product'),
  ('product.moderate', 'Moderate products (approve/reject)', 'product'),
  ('product.delete', 'Delete/hide products', 'product'),
  ('category.view', 'View categories', 'category'),
  ('category.create', 'Create categories', 'category'),
  ('category.update', 'Update categories', 'category'),
  ('category.delete', 'Delete categories', 'category')
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  module = EXCLUDED.module,
  updated_at = NOW();

-- Marketing permissions
INSERT INTO permissions (name, description, module) VALUES
  ('promo.view', 'View promotions', 'promo'),
  ('promo.create', 'Create promotions', 'promo'),
  ('promo.update', 'Update promotions', 'promo'),
  ('promo.delete', 'Delete promotions', 'promo'),
  ('banner.view', 'View banners', 'banner'),
  ('banner.create', 'Create banners', 'banner'),
  ('banner.update', 'Update banners', 'banner'),
  ('banner.delete', 'Delete banners', 'banner')
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  module = EXCLUDED.module,
  updated_at = NOW();

-- User management permissions
INSERT INTO permissions (name, description, module) VALUES
  ('user.view', 'View users', 'user'),
  ('user.create', 'Create users', 'user'),
  ('user.update', 'Update users', 'user'),
  ('user.delete', 'Delete users', 'user'),
  ('user.ban', 'Ban/unban users', 'user'),
  ('user.activate', 'Activate/deactivate users', 'user')
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  module = EXCLUDED.module,
  updated_at = NOW();

-- Role & Permission management (super_admin only)
INSERT INTO permissions (name, description, module) VALUES
  ('role.view', 'View roles', 'role'),
  ('role.create', 'Create roles', 'role'),
  ('role.update', 'Update roles', 'role'),
  ('role.delete', 'Delete roles', 'role'),
  ('role.assign', 'Assign roles to users', 'role'),
  ('permission.view', 'View permissions', 'permission'),
  ('permission.create', 'Create permissions', 'permission'),
  ('permission.update', 'Update permissions', 'permission'),
  ('permission.delete', 'Delete permissions', 'permission'),
  ('permission.assign', 'Assign permissions to roles', 'permission')
ON CONFLICT (name) DO UPDATE SET
  description = EXCLUDED.description,
  module = EXCLUDED.module,
  updated_at = NOW();

-- ================================
-- ASSIGN PERMISSIONS TO ROLES
-- ================================

-- Finance role permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'finance' AND p.name LIKE 'finance.%'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Support role permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'support' AND p.name LIKE 'support.%'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Ops role permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'ops' AND (p.name LIKE 'product.%' OR p.name LIKE 'category.%')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Marketing role permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'marketing' AND (p.name LIKE 'promo.%' OR p.name LIKE 'banner.%')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Admin role permissions (user management + view all)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin' AND (
  p.name LIKE 'user.%' OR
  p.name LIKE '%.view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;
