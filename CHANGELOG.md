# Changes History

Catatan perubahan dan implementasi fitur pada project Shopedia API.

---

## [Unreleased] - 2026-01-02

### Security & Authentication

#### JWT Token Refactoring
- Refactor JWT dengan `TokenClaims` struct
- Tambah JTI (JWT ID) untuk tracking token
- Pisahkan token type: `access` dan `register`
- Implementasi token revocation (`revoked_tokens` table)
- Implementasi single active session per user
- Tambah goroutine auto cleanup expired tokens (setiap 1 jam)

#### JWT Middleware Enhancement
- Validasi token type (hanya `access` token)
- Cek token revocation status
- Cek active session
- Cek user status dari database:
  - `deleted_at IS NULL` (not soft deleted)
  - `is_banned = FALSE` (not banned)
  - `is_active = TRUE` (active)

#### Auth Handlers
- Tambah `LogoutHandler` - revoke token + clear session
- Tambah `LogoutAllHandler` - force logout semua device
- Tambah `ForgotPasswordHandler` - request reset password via email
- Tambah `ResetPasswordHandler` - reset password dengan token
- Tambah `ChangePasswordHandler` - ganti password
- Update `RegisterHandler` dan `LoginHandler` untuk JWT baru

### User Management

#### Internal User Management
- Tambah `ListUsersHandler` - list internal users (exclude end_user)
- Tambah `GetUserHandler` - get internal user detail
- Tambah `UpdateUserHandler` - update internal user
- Tambah `DeleteUserHandler` - soft delete internal user
- Tambah `ActivateUserHandler` - activate internal user
- Tambah `DeactivateUserHandler` - deactivate + clear session

#### End User Management
- Tambah `ListEndUsersHandler` - list end_users only
- Tambah `GetEndUserHandler` - get end_user detail
- Tambah `BanEndUserHandler` - ban + clear session
- Tambah `UnbanEndUserHandler` - unban end_user

#### Session Clearing
- Ban, Deactivate, dan Delete otomatis clear active session
- Middleware block akses meskipun session clearing gagal

### Soft Delete
- Implementasi soft delete untuk users (`deleted_at` column)
- Semua query exclude soft-deleted records
- Update `DeleteUserHandler` dari hard delete ke soft delete

### Roles Management

#### Role System
- Definisi 8 roles:
  - Dashboard: `super_admin`, `admin`, `finance`, `support`, `ops`, `marketing`
  - App: `end_user`, `seller`
- Tambah `scope` field (`app` / `dashboard`)
- Tambah `is_system` flag untuk system roles

#### Role CRUD
- Tambah `ListRolesHandler` - list roles dengan filter scope
- Tambah `GetRoleHandler` - get role detail + permissions
- Tambah `CreateRoleHandler` - create role (super_admin only)
- Tambah `UpdateRoleHandler` - update role (super_admin only)
- Tambah `DeleteRoleHandler` - soft delete role (super_admin only)

#### User-Role Assignment
- Tambah `AssignRoleToUserHandler` - assign role ke user
- Tambah `RemoveRoleFromUserHandler` - remove role dari user
- Tambah `GetUserRolesHandler` - get roles user

### Permissions Management

#### Permission Modules
- `finance`: view, export, refund, payout
- `support`: view, respond, escalate, close
- `product`: view, moderate, delete
- `category`: view, create, update, delete
- `promo`: view, create, update, delete
- `banner`: view, create, update, delete
- `user`: view, create, update, delete, ban, activate
- `role`: view, create, update, delete, assign
- `permission`: view, create, update, delete, assign

#### Permission CRUD
- Tambah `ListPermissionsHandler` - list permissions dengan filter module
- Tambah `ListPermissionModulesHandler` - list unique modules
- Tambah `GetPermissionHandler` - get permission detail + roles
- Tambah `CreatePermissionHandler` - create permission (super_admin only)
- Tambah `UpdatePermissionHandler` - update permission (super_admin only)
- Tambah `DeletePermissionHandler` - soft delete permission (super_admin only)

#### Role-Permission Assignment
- Tambah `AssignPermissionsToRoleHandler` - assign permissions ke role
- Tambah `RemovePermissionFromRoleHandler` - remove permission dari role

### Middleware
- Update `RoleRequired` - support multiple roles per user
- Tambah `PermissionRequired` - cek permission user
- Tambah `ScopeRequired` - cek role scope
- `super_admin` bypass semua permission check

### Database & Migrations

#### Auto Migration
- Implementasi auto migration runner saat startup
- Tracking migration history di `migration_history` table

#### New Migrations
- `003_create_revoked_tokens.sql` - revoked tokens table
- `004_active_sessions.sql` - single active session table
- `005_password_reset_tokens.sql` - password reset tokens
- `006_add_is_banned.sql` - add `is_banned` column
- `007_add_soft_delete.sql` - add `deleted_at` column
- `008_roles_permissions_update.sql` - enhanced roles & permissions

### Files Changed/Added
```
internal/handler/
├── app_auth.go      (modified)
├── login.go         (modified)
├── logout.go        (new)
├── password.go      (new)
├── user.go          (new)
├── role.go          (new)
├── permission.go    (new)
└── routes.go        (modified)

internal/middleware/
└── jwt.go           (modified)

internal/repository/
└── db.go            (modified - auto migration)

internal/util/
└── jwt.go           (modified)

migration/
├── 003_create_revoked_tokens.sql    (new)
├── 004_active_sessions.sql          (new)
├── 005_password_reset_tokens.sql    (new)
├── 006_add_is_banned.sql            (new)
├── 007_add_soft_delete.sql          (new)
└── 008_roles_permissions_update.sql (new)

main.go              (modified - auto migration + cleanup goroutine)
```

---

## [0.1.0] - 2025-12-30

### Initial Release
- Basic project setup dengan Fiber framework
- Database connection dengan pgx
- Initial schema: users, roles, permissions, OTP, invite tokens
- Basic registration dan login
- Products dan transactions schema
