package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================
// Permission Response Types
// ============================================

type PermissionResponse struct {
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Module      *string   `json:"module"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type PermissionDetailResponse struct {
	UUID        string         `json:"uuid"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	Module      *string        `json:"module"`
	Roles       []RoleResponse `json:"roles"` // Roles that have this permission
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ============================================
// Permission Handlers
// ============================================

// ListPermissionsHandler - GET /permissions - list all permissions with pagination
func ListPermissionsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		// Pagination params
		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		offset := (page - 1) * limit

		// Filter params
		search := c.Query("search", "")
		moduleFilter := c.Query("module", "")

		// Build query - exclude soft-deleted
		baseQuery := `FROM permissions WHERE deleted_at IS NULL`
		args := []interface{}{}
		argCount := 0

		if search != "" {
			argCount++
			baseQuery += ` AND (name ILIKE $` + strconv.Itoa(argCount) + ` OR description ILIKE $` + strconv.Itoa(argCount) + `)`
			args = append(args, "%"+search+"%")
		}

		if moduleFilter != "" {
			argCount++
			baseQuery += ` AND module = $` + strconv.Itoa(argCount)
			args = append(args, moduleFilter)
		}

		// Count total
		var totalItems int
		countQuery := `SELECT COUNT(*) ` + baseQuery
		err := db.QueryRow(ctx, countQuery, args...).Scan(&totalItems)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Get data with pagination
		argCount++
		limitArg := argCount
		argCount++
		offsetArg := argCount
		dataQuery := `SELECT uuid, name, description, module, created_at, updated_at ` +
			baseQuery + ` ORDER BY module, name LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg)
		args = append(args, limit, offset)

		rows, err := db.Query(ctx, dataQuery, args...)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		permissions := []PermissionResponse{}
		for rows.Next() {
			var perm PermissionResponse
			err := rows.Scan(&perm.UUID, &perm.Name, &perm.Description,
				&perm.Module, &perm.CreatedAt, &perm.UpdatedAt)
			if err != nil {
				continue
			}
			permissions = append(permissions, perm)
		}

		totalPages := (totalItems + limit - 1) / limit

		return c.JSON(PaginatedResponse{
			Data:       permissions,
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		})
	}
}

// ListPermissionModulesHandler - GET /permissions/modules - list all unique modules
func ListPermissionModulesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		rows, err := db.Query(ctx, `
			SELECT DISTINCT module FROM permissions
			WHERE deleted_at IS NULL AND module IS NOT NULL
			ORDER BY module`)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		modules := []string{}
		for rows.Next() {
			var module string
			if rows.Scan(&module) == nil {
				modules = append(modules, module)
			}
		}

		return c.JSON(fiber.Map{
			"modules": modules,
		})
	}
}

// GetPermissionHandler - GET /permissions/:uuid - get permission by uuid
func GetPermissionHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		permUUID := c.Params("uuid")
		ctx := context.Background()

		var perm PermissionDetailResponse
		var permID int
		err := db.QueryRow(ctx, `
			SELECT id, uuid, name, description, module, created_at, updated_at
			FROM permissions WHERE uuid = $1 AND deleted_at IS NULL`,
			permUUID).Scan(
			&permID, &perm.UUID, &perm.Name, &perm.Description,
			&perm.Module, &perm.CreatedAt, &perm.UpdatedAt)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Get roles that have this permission
		roleRows, err := db.Query(ctx, `
			SELECT r.uuid, r.name, r.description, r.scope, r.is_system, r.created_at, r.updated_at
			FROM role_permissions rp
			JOIN roles r ON rp.role_id = r.id
			WHERE rp.permission_id = $1 AND r.deleted_at IS NULL
			ORDER BY r.name`, permID)
		if err == nil {
			defer roleRows.Close()
			for roleRows.Next() {
				var role RoleResponse
				if roleRows.Scan(&role.UUID, &role.Name, &role.Description,
					&role.Scope, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt) == nil {
					perm.Roles = append(perm.Roles, role)
				}
			}
		}

		if perm.Roles == nil {
			perm.Roles = []RoleResponse{}
		}

		return c.JSON(perm)
	}
}

// CreatePermissionHandler - POST /permissions - create new permission (super_admin only)
func CreatePermissionHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			Name        string  `json:"name"`
			Description *string `json:"description"`
			Module      *string `json:"module"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		if input.Name == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Name is required")
		}

		ctx := context.Background()

		// Check if permission name already exists
		var exists bool
		err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM permissions WHERE name = $1 AND deleted_at IS NULL)`,
			input.Name).Scan(&exists)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		if exists {
			return fiber.NewError(fiber.StatusConflict, "Permission name already exists")
		}

		// Create permission
		var permUUID string
		err = db.QueryRow(ctx, `
			INSERT INTO permissions (name, description, module)
			VALUES ($1, $2, $3)
			RETURNING uuid`,
			input.Name, input.Description, input.Module).Scan(&permUUID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "Permission created successfully",
			"uuid":    permUUID,
		})
	}
}

// UpdatePermissionHandler - PUT /permissions/:uuid - update permission (super_admin only)
func UpdatePermissionHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		permUUID := c.Params("uuid")

		type Input struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
			Module      *string `json:"module"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		// Check if permission exists
		var permID int
		err := db.QueryRow(ctx, `
			SELECT id FROM permissions WHERE uuid = $1 AND deleted_at IS NULL`,
			permUUID).Scan(&permID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Update fields
		if input.Name != nil {
			// Check if new name already exists
			var exists bool
			err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM permissions WHERE name = $1 AND id != $2 AND deleted_at IS NULL)`,
				*input.Name, permID).Scan(&exists)
			if exists {
				return fiber.NewError(fiber.StatusConflict, "Permission name already exists")
			}

			_, err = db.Exec(ctx, `UPDATE permissions SET name = $1, updated_at = NOW() WHERE id = $2`,
				*input.Name, permID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Description != nil {
			_, err = db.Exec(ctx, `UPDATE permissions SET description = $1, updated_at = NOW() WHERE id = $2`,
				*input.Description, permID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Module != nil {
			_, err = db.Exec(ctx, `UPDATE permissions SET module = $1, updated_at = NOW() WHERE id = $2`,
				*input.Module, permID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		return c.JSON(fiber.Map{
			"message": "Permission updated successfully",
		})
	}
}

// DeletePermissionHandler - DELETE /permissions/:uuid - soft delete permission (super_admin only)
func DeletePermissionHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		permUUID := c.Params("uuid")
		ctx := context.Background()

		// Check if permission exists
		var permID int
		err := db.QueryRow(ctx, `
			SELECT id FROM permissions WHERE uuid = $1 AND deleted_at IS NULL`,
			permUUID).Scan(&permID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if permission is assigned to any roles
		var roleCount int
		err = db.QueryRow(ctx, `SELECT COUNT(*) FROM role_permissions WHERE permission_id = $1`, permID).Scan(&roleCount)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if roleCount > 0 {
			return fiber.NewError(fiber.StatusConflict, "Cannot delete permission that is assigned to roles")
		}

		// Soft delete
		_, err = db.Exec(ctx, `UPDATE permissions SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, permID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "Permission deleted successfully",
		})
	}
}
