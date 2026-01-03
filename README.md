# Shopedia API

REST API untuk platform e-commerce Shopedia, dibangun dengan Go dan Fiber framework.

## Tech Stack

- **Go** 1.21+
- **Fiber** v2 - Web framework
- **PostgreSQL** - Database
- **pgx** v5 - PostgreSQL driver
- **Redis** - Queue, Caching & Rate Limiting
- **Asynq** - Background job processing
- **JWT** - Authentication
- **Nginx** - Load Balancer (Production)

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

## Docker

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+

### Architecture

```
Production:
                      ┌→ api-1:3000
User :80 → nginx:80 → ├→ api-2:3000  (load balanced)
                      └→ api-n:3000
                            ↓
              ┌─────────────┴─────────────┐
              ↓                           ↓
         PostgreSQL                     Redis
              ↑                           ↑
              └─────────────┬─────────────┘
                            ↓
                    worker-1, worker-n
```

### Quick Start (Production)

Jalankan aplikasi lengkap dengan load balancer:

```bash
# Copy environment file
cp .env.example .env

# Edit .env sesuai kebutuhan (terutama JWT_SECRET dan SMTP)

# Build dan jalankan
docker-compose up -d

# Lihat logs
docker-compose logs -f api
```

Aplikasi akan berjalan di `http://localhost` (port 80 via nginx)

### Scaling (Production)

```bash
# Scale API ke 3 instances
docker-compose up -d --scale api=3

# Scale Worker ke 2 instances
docker-compose up -d --scale worker=2

# Scale keduanya
docker-compose up -d --scale api=3 --scale worker=2
```

### Development Mode

Untuk development, jalankan PostgreSQL + Redis via Docker, API & Worker lokal:

```bash
# Terminal 1: Jalankan PostgreSQL + Redis
docker-compose -f docker-compose.dev.yml up -d

# Terminal 2: Jalankan Worker
go run cmd/worker/main.go

# Terminal 3: Jalankan API
go run main.go
```

API akan berjalan di `http://localhost:3000`

### Docker Commands

```bash
# Build image saja
docker-compose build

# Start services
docker-compose up -d

# Stop services
docker-compose down

# Stop dan hapus volumes (reset database)
docker-compose down -v

# Lihat logs
docker-compose logs -f

# Restart service tertentu
docker-compose restart api

# Masuk ke container PostgreSQL
docker exec -it shopedia-db psql -U shopedia -d shopedia
```

### Environment Variables (Docker)

| Variable          | Default       | Deskripsi              |
| ----------------- | ------------- | ---------------------- |
| `POSTGRES_USER`   | shopedia      | PostgreSQL username    |
| `POSTGRES_PASSWORD` | shopedia123 | PostgreSQL password    |
| `POSTGRES_DB`     | shopedia      | PostgreSQL database    |
| `JWT_SECRET`      | -             | JWT secret key (wajib) |
| `SMTP_HOST`       | smtp.gmail.com| SMTP server            |
| `SMTP_PORT`       | 587           | SMTP port              |
| `SMTP_USER`       | -             | SMTP username          |
| `SMTP_PASS`       | -             | SMTP password          |
| `PORT`            | 3000          | Application port       |
| `REDIS_URL`       | redis://localhost:6379 | Redis connection URL |

---

## Queue & Worker

Shopedia API menggunakan **Asynq** dengan **Redis** untuk background job processing.

### Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   API       │────▶│   Redis     │◀────│   Worker    │
│  (Producer) │     │   (Queue)   │     │  (Consumer) │
└─────────────┘     └─────────────┘     └─────────────┘
```

### Available Task Types

| Task Type             | Queue    | Deskripsi                    |
| --------------------- | -------- | ---------------------------- |
| `email:send`          | default  | Send generic email           |
| `email:otp`           | critical | Send OTP verification        |
| `email:welcome`       | default  | Send welcome email           |
| `email:password_reset`| critical | Send password reset email    |
| `notification:send`   | default  | Send user notification       |
| `product:index`       | low      | Index product for search     |

### Running Worker

```bash
# Development (local)
go run cmd/worker/main.go

# Production (Docker)
docker-compose up -d worker

# Lihat worker logs
docker-compose logs -f worker
```

### Enqueueing Tasks

```go
import "shopedia-api/internal/queue"

// Send OTP email
task, _ := queue.NewSendOTPTask("user@email.com", "123456", "John")
queue.Enqueue(task, asynq.Queue("critical"))

// Send welcome email
task, _ := queue.NewSendWelcomeTask("user@email.com", "John")
queue.Enqueue(task)

// Send notification
task, _ := queue.NewNotificationTask(userID, "Title", "Message", "info")
queue.Enqueue(task)
```

### Queue Priority

| Queue    | Weight | Use Case                    |
| -------- | ------ | --------------------------- |
| critical | 6      | OTP, password reset, urgent |
| default  | 3      | Welcome email, notifications|
| low      | 1      | Indexing, cleanup, reports  |

---

## Redis Caching

Shopedia API menggunakan Redis untuk caching data yang sering diakses.

### Cache Layers

| Data | TTL | Key Pattern | Deskripsi |
| ---- | --- | ----------- | --------- |
| Categories | 1 jam | `cache:categories` | List semua kategori aktif |
| Session | 24 jam | `session:{jti}` | Active session data |
| OTP | 5 menit | `otp:{email}` | OTP verification dengan attempt tracking |
| Rate Limit | 1-5 menit | `ratelimit:{key}` | Request rate limiting |

### Cache Strategy

- **Write-through**: Session dan OTP disimpan ke Redis + PostgreSQL
- **Cache-aside**: Categories di-cache saat pertama kali diakses
- **Auto-invalidation**: Categories cache di-invalidate saat create/update/delete
- **Graceful fallback**: Jika Redis down, fallback ke PostgreSQL

### Cache Headers

Response dari cached endpoints menyertakan header:
```
X-Cache: HIT   # Data dari cache
X-Cache: MISS  # Data dari database
```

---

## Rate Limiting

API menggunakan Redis-based rate limiting untuk mencegah abuse.

### Rate Limit Rules

| Endpoint | Limit | Window | Deskripsi |
| -------- | ----- | ------ | --------- |
| `/app/register` | 5 req | 1 menit | Strict limit |
| `/app/login` | 5 req | 1 menit | Strict limit |
| `/app/verify-otp` | 5 req | 1 menit | Strict limit |
| `/app/request-new-otp` | 3 req | 5 menit | OTP limit |
| `/app/forgot-password` | 5 req | 1 menit | Strict limit |
| `/app/reset-password` | 5 req | 1 menit | Strict limit |
| Other endpoints | 100 req | 1 menit | Default limit |

### Rate Limit Headers

```
X-RateLimit-Limit: 100        # Max requests
X-RateLimit-Remaining: 95     # Remaining requests
X-RateLimit-Reset: 1704067200 # Reset timestamp (Unix)
Retry-After: 60               # Seconds to wait (saat limit exceeded)
```

### OTP Attempt Tracking

OTP verification memiliki max 3 attempts. Setelah 3x salah, harus request OTP baru.

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

### Public Products & Categories

Base URL: `/api`

| Method | Endpoint         | Auth | Deskripsi                    |
| ------ | ---------------- | :--: | ---------------------------- |
| GET    | `/categories`    |  -   | List active categories       |
| GET    | `/products`      |  -   | List active products         |
| GET    | `/products/:uuid`|  -   | Get product detail           |

#### Query Parameters (Products)

| Parameter  | Type   | Default | Deskripsi              |
| ---------- | ------ | ------- | ---------------------- |
| `page`     | int    | 1       | Halaman                |
| `limit`    | int    | 20      | Items per page (max: 100) |
| `search`   | string | -       | Search by title/description |
| `category` | string | -       | Filter by category UUID |

### Seller Products (App)

Base URL: `/api/app/my`

| Method | Endpoint         | Auth | Deskripsi              |
| ------ | ---------------- | :--: | ---------------------- |
| GET    | `/products`      |  ✅  | List seller's products |
| GET    | `/products/:uuid`|  ✅  | Get product detail     |
| POST   | `/products`      |  ✅  | Create new product     |
| PUT    | `/products/:uuid`|  ✅  | Update product         |
| DELETE | `/products/:uuid`|  ✅  | Soft delete product    |

#### Create/Update Product Body

```json
{
  "title": "Product Name",
  "description": "Product description",
  "images": ["https://example.com/img1.jpg", "https://example.com/img2.jpg"],
  "price": 100000,
  "stock": 10,
  "category_uuid": "uuid-of-category",
  "slug": "optional-custom-slug",
  "is_active": true
}
```

### Admin Category Management (Dashboard)

Base URL: `/api/admin`

| Method | Endpoint            | Permission      | Deskripsi         |
| ------ | ------------------- | --------------- | ----------------- |
| GET    | `/categories`       | category.view   | List categories   |
| GET    | `/categories/:uuid` | category.view   | Get category      |
| POST   | `/categories`       | category.create | Create category   |
| PUT    | `/categories/:uuid` | category.update | Update category   |
| DELETE | `/categories/:uuid` | category.delete | Delete category   |

### Admin Product Management (Dashboard)

Base URL: `/api/admin`

| Method | Endpoint                 | Permission       | Deskripsi        |
| ------ | ------------------------ | ---------------- | ---------------- |
| GET    | `/products`              | product.view     | List all products|
| GET    | `/products/:uuid`        | product.view     | Get product      |
| POST   | `/products/:uuid/block`  | product.moderate | Block product    |
| POST   | `/products/:uuid/unblock`| product.moderate | Unblock product  |

#### Block Product Body

```json
{
  "reason": "Alasan pemblokiran produk"
}
```

#### Query Parameters (Admin Products)

| Parameter  | Type   | Default | Deskripsi               |
| ---------- | ------ | ------- | ----------------------- |
| `page`     | int    | 1       | Halaman                 |
| `limit`    | int    | 20      | Items per page (max: 100) |
| `search`   | string | -       | Search by title/description |
| `status`   | string | -       | Filter: active, blocked |
| `category` | string | -       | Filter by category UUID |
| `owner`    | string | -       | Filter by owner UUID    |

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
| `009_products_enhancement.sql`      | Products & categories enhancement |

### Manual Migration

```bash
psql -d shopedia -f migration/001_initial_schema.sql
```

---

## Project Structure

```
shopedia-api/
├── cmd/
│   └── worker/           # Worker entry point
│       └── main.go
├── internal/
│   ├── cache/            # Redis caching
│   │   └── redis.go
│   ├── handler/          # HTTP handlers
│   │   ├── app_auth.go
│   │   ├── login.go
│   │   ├── logout.go
│   │   ├── password.go
│   │   ├── user.go
│   │   ├── role.go
│   │   ├── permission.go
│   │   ├── category.go
│   │   ├── product.go
│   │   └── routes.go
│   ├── middleware/       # Middleware
│   │   ├── jwt.go
│   │   └── ratelimit.go
│   ├── queue/            # Queue & Tasks
│   │   ├── client.go
│   │   ├── tasks.go
│   │   └── handlers.go
│   ├── repository/       # Database
│   │   └── db.go
│   └── util/             # Utilities
│       └── jwt.go
├── migration/            # SQL migrations
├── main.go               # API entry point
├── Dockerfile            # API Dockerfile
├── Dockerfile.worker     # Worker Dockerfile
├── docker-compose.yml    # Production compose (with nginx)
├── docker-compose.dev.yml # Development compose
├── nginx.conf            # Nginx load balancer config
├── go.mod
├── go.sum
├── .env
└── README.md
```

---

## Security Features

- JWT dengan JTI untuk token tracking
- Single active session per user (Redis-cached)
- Token revocation
- Password hashing dengan bcrypt
- Soft delete (data tidak dihapus permanen)
- Role-based access control (RBAC)
- Permission-based authorization
- Auto session clear saat ban/deactivate/delete
- Redis-based rate limiting
- OTP attempt tracking (max 3 attempts)
- Graceful fallback saat Redis unavailable

---

## License

MIT License
