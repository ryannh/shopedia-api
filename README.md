# Shopedia API

REST API untuk platform e-commerce Shopedia, dibangun dengan Go dan Fiber framework.

## Tech Stack

- **Go** 1.21+
- **Fiber** v2 - Web framework
- **PostgreSQL** - Database
- **pgx** v5 - PostgreSQL driver
- **JWT** - Authentication

## Getting Started

### Prerequisites

- Go 1.21 atau lebih baru
- PostgreSQL 14+

### Installation

```bash
# Clone repository
git clone https://github.com/ryannh/shopedia-api.git
cd shopedia-api

# Install dependencies
go mod download

# Copy environment file
cp .env.example .env
```

### Environment Variables

Edit file `.env` dan sesuaikan dengan konfigurasi lokal. Lihat `.env.example` untuk referensi.

### Run Application

```bash
go run main.go
```

Server akan berjalan di `http://localhost:3000`. Migration dijalankan otomatis saat startup.

---

## Authentication

API menggunakan JWT Bearer token untuk authentication.

```
Authorization: Bearer <access_token>
```

### Token Types

| Type       | Penggunaan     | Lifetime          |
| ---------- | -------------- | ----------------- |
| `access`   | Akses API      | 24 jam            |
| `register` | Registrasi/OTP | Sesuai OTP expiry |

---

## API Endpoints

### App Authentication

Base URL: `/api/app`

| Method | Endpoint           | Auth | Deskripsi              |
| ------ | ------------------ | :--: | ---------------------- |
| POST   | `/register`        |  -   | Register user baru     |
| POST   | `/verify-otp`      |  -   | Verifikasi OTP         |
| POST   | `/request-new-otp` |  -   | Request OTP baru       |
| POST   | `/login`           |  -   | Login                  |
| POST   | `/forgot-password` |  -   | Request reset password |
| POST   | `/reset-password`  |  -   | Reset password         |
| POST   | `/logout`          |  ✅  | Logout                 |
| POST   | `/logout-all`      |  ✅  | Logout semua device    |
| POST   | `/change-password` |  ✅  | Ganti password         |
| GET    | `/me`              |  ✅  | Get profile            |
| PUT    | `/me`              |  ✅  | Update profile         |

### Admin Authentication

Base URL: `/api/admin`

| Method | Endpoint           | Auth | Deskripsi                    |
| ------ | ------------------ | :--: | ---------------------------- |
| POST   | `/register`        |  -   | Register super_admin pertama |
| POST   | `/accept-invite`   |  -   | Accept invite                |
| POST   | `/login`           |  -   | Login                        |
| POST   | `/forgot-password` |  -   | Request reset password       |
| POST   | `/reset-password`  |  -   | Reset password               |
| POST   | `/logout`          |  ✅  | Logout                       |
| POST   | `/logout-all`      |  ✅  | Logout semua device          |
| POST   | `/change-password` |  ✅  | Ganti password               |
| POST   | `/invite-user`     |  ✅  | Invite admin (super_admin)   |
| GET    | `/me`              |  ✅  | Get profile                  |
| PUT    | `/me`              |  ✅  | Update profile               |

### User Management

Base URL: `/api/admin`

#### Internal Users (Dashboard)

| Method | Endpoint                        | Permission    | Deskripsi           |
| ------ | ------------------------------- | ------------- | ------------------- |
| GET    | `/users`                        | user.view     | List internal users |
| GET    | `/users/:uuid`                  | user.view     | Get user detail     |
| PUT    | `/users/:uuid`                  | user.update   | Update user         |
| DELETE | `/users/:uuid`                  | user.delete   | Soft delete user    |
| POST   | `/users/:uuid/activate`         | user.activate | Activate user       |
| POST   | `/users/:uuid/deactivate`       | user.activate | Deactivate user     |
| GET    | `/users/:uuid/roles`            | user.view     | Get user roles      |
| POST   | `/users/:uuid/roles`            | role.assign   | Assign role         |
| DELETE | `/users/:uuid/roles/:role_uuid` | role.assign   | Remove role         |

#### End Users (App)

| Method | Endpoint                 | Permission | Deskripsi           |
| ------ | ------------------------ | ---------- | ------------------- |
| GET    | `/end-users`             | user.view  | List end users      |
| GET    | `/end-users/:uuid`       | user.view  | Get end user detail |
| POST   | `/end-users/:uuid/ban`   | user.ban   | Ban user            |
| POST   | `/end-users/:uuid/unban` | user.ban   | Unban user          |

#### Query Parameters

| Parameter   | Type   | Default | Deskripsi                 |
| ----------- | ------ | ------- | ------------------------- |
| `page`      | int    | 1       | Halaman                   |
| `limit`     | int    | 10      | Items per page (max: 100) |
| `search`    | string | -       | Search by email/name      |
| `is_active` | bool   | -       | Filter by status          |
| `role`      | string | -       | Filter by role            |
| `is_banned` | bool   | -       | Filter banned (end users) |

### Role Management

Base URL: `/api/admin`

| Method | Endpoint                              | Permission  | Deskripsi              |
| ------ | ------------------------------------- | ----------- | ---------------------- |
| GET    | `/roles`                              | role.view   | List roles             |
| GET    | `/roles/:uuid`                        | role.view   | Get role + permissions |
| POST   | `/roles`                              | super_admin | Create role            |
| PUT    | `/roles/:uuid`                        | super_admin | Update role            |
| DELETE | `/roles/:uuid`                        | super_admin | Delete role            |
| POST   | `/roles/:uuid/permissions`            | super_admin | Assign permissions     |
| DELETE | `/roles/:uuid/permissions/:perm_uuid` | super_admin | Remove permission      |

### Permission Management

Base URL: `/api/admin`

| Method | Endpoint               | Permission      | Deskripsi             |
| ------ | ---------------------- | --------------- | --------------------- |
| GET    | `/permissions`         | permission.view | List permissions      |
| GET    | `/permissions/modules` | permission.view | List modules          |
| GET    | `/permissions/:uuid`   | permission.view | Get permission detail |
| POST   | `/permissions`         | super_admin     | Create permission     |
| PUT    | `/permissions/:uuid`   | super_admin     | Update permission     |
| DELETE | `/permissions/:uuid`   | super_admin     | Delete permission     |

---

## Roles & Permissions

### Roles

| Role          | Scope     | Deskripsi        |
| ------------- | --------- | ---------------- |
| `super_admin` | dashboard | Full access      |
| `admin`       | dashboard | Internal staff   |
| `finance`     | dashboard | Tim finance      |
| `support`     | dashboard | Customer support |
| `ops`         | dashboard | Moderasi produk  |
| `marketing`   | dashboard | Promo & banner   |
| `end_user`    | app       | User aplikasi    |
| `seller`      | app       | User + toko      |

### Permission Modules

| Module       | Permissions                                 |
| ------------ | ------------------------------------------- |
| `finance`    | view, export, refund, payout                |
| `support`    | view, respond, escalate, close              |
| `product`    | view, moderate, delete                      |
| `category`   | view, create, update, delete                |
| `promo`      | view, create, update, delete                |
| `banner`     | view, create, update, delete                |
| `user`       | view, create, update, delete, ban, activate |
| `role`       | view, create, update, delete, assign        |
| `permission` | view, create, update, delete, assign        |

### Default Permissions

| Role          | Permissions           |
| ------------- | --------------------- |
| `super_admin` | All (bypass check)    |
| `admin`       | user._, _.view        |
| `finance`     | finance.\*            |
| `support`     | support.\*            |
| `ops`         | product._, category._ |
| `marketing`   | promo._, banner._     |

---

## Response Format

### Success Response

```json
{
  "message": "Success message",
  "data": { ... }
}
```

### Paginated Response

```json
{
  "data": [ ... ],
  "page": 1,
  "limit": 10,
  "total_items": 100,
  "total_pages": 10
}
```

### Error Response

```json
{
  "error": "Error message"
}
```

### HTTP Status Codes

| Code | Deskripsi             |
| ---- | --------------------- |
| 200  | OK                    |
| 201  | Created               |
| 400  | Bad Request           |
| 401  | Unauthorized          |
| 403  | Forbidden             |
| 404  | Not Found             |
| 409  | Conflict              |
| 500  | Internal Server Error |

---

## Database Migrations

Migration dijalankan otomatis saat startup. File migration ada di folder `migration/`.

| File                                | Deskripsi                      |
| ----------------------------------- | ------------------------------ |
| `001_initial_schema.sql`            | Users, roles, permissions, OTP |
| `002_products_and_transactions.sql` | Products & transactions        |
| `003_create_revoked_tokens.sql`     | Token revocation               |
| `004_active_sessions.sql`           | Single active session          |
| `005_password_reset_tokens.sql`     | Password reset                 |
| `006_add_is_banned.sql`             | Ban feature                    |
| `007_add_soft_delete.sql`           | Soft delete                    |
| `008_roles_permissions_update.sql`  | Enhanced RBAC                  |

### Manual Migration

```bash
psql -d shopedia -f migration/001_initial_schema.sql
```

---

## Project Structure

```
shopedia-api/
├── internal/
│   ├── handler/          # HTTP handlers
│   │   ├── app_auth.go
│   │   ├── login.go
│   │   ├── logout.go
│   │   ├── password.go
│   │   ├── user.go
│   │   ├── role.go
│   │   ├── permission.go
│   │   └── routes.go
│   ├── middleware/       # Middleware
│   │   └── jwt.go
│   ├── repository/       # Database
│   │   └── db.go
│   └── util/             # Utilities
│       └── jwt.go
├── migration/            # SQL migrations
├── main.go
├── go.mod
├── go.sum
├── .env
├── README.md
└── changes-history.md
```

---

## Security Features

- JWT dengan JTI untuk token tracking
- Single active session per user
- Token revocation
- Password hashing dengan bcrypt
- Soft delete (data tidak dihapus permanen)
- Role-based access control (RBAC)
- Permission-based authorization
- Auto session clear saat ban/deactivate/delete

---

## License

MIT License
