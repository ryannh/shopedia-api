package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================
// Role Response Types
// ============================================

type RoleResponse struct {
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Scope       string    `json:"scope"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RoleDetailResponse struct {
	UUID        string               `json:"uuid"`
	Name        string               `json:"name"`
	Description *string              `json:"description"`
	Scope       string               `json:"scope"`
	IsSystem    bool                 `json:"is_system"`
	Permissions []PermissionResponse `json:"permissions"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// ============================================
// Role Handlers
// ============================================

// ListRolesHandler - GET /roles - list all roles with pagination
func ListRolesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		// Pagination params
		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "10"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 10
		}
		offset := (page - 1) * limit

		// Filter params
		search := c.Query("search", "")
		scopeFilter := c.Query("scope", "") // 'app' or 'dashboard'

		// Build query - exclude soft-deleted
		baseQuery := `FROM roles WHERE deleted_at IS NULL`
		args := []interface{}{}
		argCount := 0

		if search != "" {
			argCount++
			baseQuery += ` AND (name ILIKE $` + strconv.Itoa(argCount) + ` OR description ILIKE $` + strconv.Itoa(argCount) + `)`
			args = append(args, "%"+search+"%")
		}

		if scopeFilter != "" {
			argCount++
			baseQuery += ` AND scope = $` + strconv.Itoa(argCount)
			args = append(args, scopeFilter)
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
		dataQuery := `SELECT uuid, name, description, scope, is_system, created_at, updated_at ` +
			baseQuery + ` ORDER BY created_at ASC LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg)
		args = append(args, limit, offset)

		rows, err := db.Query(ctx, dataQuery, args...)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		roles := []RoleResponse{}
		for rows.Next() {
			var role RoleResponse
			err := rows.Scan(&role.UUID, &role.Name, &role.Description,
				&role.Scope, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
			if err != nil {
				continue
			}
			roles = append(roles, role)
		}

		totalPages := (totalItems + limit - 1) / limit

		return c.JSON(PaginatedResponse{
			Data:       roles,
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		})
	}
}

// GetRoleHandler - GET /roles/:uuid - get role by uuid with permissions
func GetRoleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleUUID := c.Params("uuid")
		ctx := context.Background()

		var role RoleDetailResponse
		var roleID int
		err := db.QueryRow(ctx, `
			SELECT id, uuid, name, description, scope, is_system, created_at, updated_at
			FROM roles WHERE uuid = $1 AND deleted_at IS NULL`,
			roleUUID).Scan(
			&roleID, &role.UUID, &role.Name, &role.Description,
			&role.Scope, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Get permissions for this role
		permRows, err := db.Query(ctx, `
			SELECT p.uuid, p.name, p.description, p.module
			FROM role_permissions rp
			JOIN permissions p ON rp.permission_id = p.id
			WHERE rp.role_id = $1 AND p.deleted_at IS NULL
			ORDER BY p.module, p.name`, roleID)
		if err == nil {
			defer permRows.Close()
			for permRows.Next() {
				var perm PermissionResponse
				if permRows.Scan(&perm.UUID, &perm.Name, &perm.Description, &perm.Module) == nil {
					role.Permissions = append(role.Permissions, perm)
				}
			}
		}

		if role.Permissions == nil {
			role.Permissions = []PermissionResponse{}
		}

		return c.JSON(role)
	}
}

// CreateRoleHandler - POST /roles - create new role (super_admin only)
func CreateRoleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			Name        string  `json:"name"`
			Description *string `json:"description"`
			Scope       string  `json:"scope"` // 'app' or 'dashboard'
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		if input.Name == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Name is required")
		}

		if input.Scope != "app" && input.Scope != "dashboard" {
			return fiber.NewError(fiber.StatusBadRequest, "Scope must be 'app' or 'dashboard'")
		}

		ctx := context.Background()

		// Check if role name already exists
		var exists bool
		err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1 AND deleted_at IS NULL)`,
			input.Name).Scan(&exists)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		if exists {
			return fiber.NewError(fiber.StatusConflict, "Role name already exists")
		}

		// Create role
		var roleUUID string
		err = db.QueryRow(ctx, `
			INSERT INTO roles (name, description, scope, is_system)
			VALUES ($1, $2, $3, FALSE)
			RETURNING uuid`,
			input.Name, input.Description, input.Scope).Scan(&roleUUID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "Role created successfully",
			"uuid":    roleUUID,
		})
	}
}

// UpdateRoleHandler - PUT /roles/:uuid - update role (super_admin only)
func UpdateRoleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleUUID := c.Params("uuid")

		type Input struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
			Scope       *string `json:"scope"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		// Check if role exists and is not system role
		var roleID int
		var isSystem bool
		err := db.QueryRow(ctx, `
			SELECT id, is_system FROM roles WHERE uuid = $1 AND deleted_at IS NULL`,
			roleUUID).Scan(&roleID, &isSystem)
		if err != nil {
			return fiber.ErrNotFound
		}

		// System roles can only update description
		if isSystem && (input.Name != nil || input.Scope != nil) {
			return fiber.NewError(fiber.StatusForbidden, "Cannot modify name or scope of system roles")
		}

		// Update fields
		if input.Name != nil {
			// Check if new name already exists
			var exists bool
			err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1 AND id != $2 AND deleted_at IS NULL)`,
				*input.Name, roleID).Scan(&exists)
			if exists {
				return fiber.NewError(fiber.StatusConflict, "Role name already exists")
			}

			_, err = db.Exec(ctx, `UPDATE roles SET name = $1, updated_at = NOW() WHERE id = $2`,
				*input.Name, roleID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Description != nil {
			_, err = db.Exec(ctx, `UPDATE roles SET description = $1, updated_at = NOW() WHERE id = $2`,
				*input.Description, roleID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Scope != nil {
			if *input.Scope != "app" && *input.Scope != "dashboard" {
				return fiber.NewError(fiber.StatusBadRequest, "Scope must be 'app' or 'dashboard'")
			}
			_, err = db.Exec(ctx, `UPDATE roles SET scope = $1, updated_at = NOW() WHERE id = $2`,
				*input.Scope, roleID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		return c.JSON(fiber.Map{
			"message": "Role updated successfully",
		})
	}
}

// DeleteRoleHandler - DELETE /roles/:uuid - soft delete role (super_admin only)
func DeleteRoleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleUUID := c.Params("uuid")
		ctx := context.Background()

		// Check if role exists and is not system role
		var roleID int
		var isSystem bool
		err := db.QueryRow(ctx, `
			SELECT id, is_system FROM roles WHERE uuid = $1 AND deleted_at IS NULL`,
			roleUUID).Scan(&roleID, &isSystem)
		if err != nil {
			return fiber.ErrNotFound
		}

		if isSystem {
			return fiber.NewError(fiber.StatusForbidden, "Cannot delete system roles")
		}

		// Check if role is assigned to any users
		var userCount int
		err = db.QueryRow(ctx, `SELECT COUNT(*) FROM user_roles WHERE role_id = $1`, roleID).Scan(&userCount)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if userCount > 0 {
			return fiber.NewError(fiber.StatusConflict, "Cannot delete role that is assigned to users")
		}

		// Soft delete
		_, err = db.Exec(ctx, `UPDATE roles SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, roleID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "Role deleted successfully",
		})
	}
}

// ============================================
// Role-Permission Assignment Handlers
// ============================================

// AssignPermissionsToRoleHandler - POST /roles/:uuid/permissions - assign permissions to role
func AssignPermissionsToRoleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleUUID := c.Params("uuid")

		type Input struct {
			PermissionUUIDs []string `json:"permission_uuids"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		if len(input.PermissionUUIDs) == 0 {
			return fiber.NewError(fiber.StatusBadRequest, "At least one permission UUID is required")
		}

		ctx := context.Background()

		// Get role ID
		var roleID int
		err := db.QueryRow(ctx, `SELECT id FROM roles WHERE uuid = $1 AND deleted_at IS NULL`,
			roleUUID).Scan(&roleID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Assign permissions
		assignedCount := 0
		for _, permUUID := range input.PermissionUUIDs {
			var permID int
			err := db.QueryRow(ctx, `SELECT id FROM permissions WHERE uuid = $1 AND deleted_at IS NULL`,
				permUUID).Scan(&permID)
			if err != nil {
				continue // Skip invalid permission
			}

			_, err = db.Exec(ctx, `
				INSERT INTO role_permissions (role_id, permission_id)
				VALUES ($1, $2) ON CONFLICT (role_id, permission_id) DO NOTHING`,
				roleID, permID)
			if err == nil {
				assignedCount++
			}
		}

		return c.JSON(fiber.Map{
			"message":        "Permissions assigned successfully",
			"assigned_count": assignedCount,
		})
	}
}

// RemovePermissionFromRoleHandler - DELETE /roles/:uuid/permissions/:perm_uuid - remove permission from role
func RemovePermissionFromRoleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleUUID := c.Params("uuid")
		permUUID := c.Params("perm_uuid")
		ctx := context.Background()

		// Get role ID
		var roleID int
		err := db.QueryRow(ctx, `SELECT id FROM roles WHERE uuid = $1 AND deleted_at IS NULL`,
			roleUUID).Scan(&roleID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Get permission ID
		var permID int
		err = db.QueryRow(ctx, `SELECT id FROM permissions WHERE uuid = $1 AND deleted_at IS NULL`,
			permUUID).Scan(&permID)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "Permission not found")
		}

		// Remove assignment
		result, err := db.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`,
			roleID, permID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if result.RowsAffected() == 0 {
			return fiber.NewError(fiber.StatusNotFound, "Permission not assigned to this role")
		}

		return c.JSON(fiber.Map{
			"message": "Permission removed from role successfully",
		})
	}
}

// ============================================
// User-Role Assignment Handlers
// ============================================

// AssignRoleToUserHandler - POST /users/:uuid/roles - assign role to user
func AssignRoleToUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userUUID := c.Params("uuid")
		assignedBy := c.Locals("userID").(int)

		type Input struct {
			RoleUUID string `json:"role_uuid"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		// Get user ID
		var userID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`,
			userUUID).Scan(&userID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Get role ID
		var roleID int
		err = db.QueryRow(ctx, `SELECT id FROM roles WHERE uuid = $1 AND deleted_at IS NULL`,
			input.RoleUUID).Scan(&roleID)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "Role not found")
		}

		// Assign role
		_, err = db.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id, assigned_by)
			VALUES ($1, $2, $3) ON CONFLICT (user_id, role_id) DO NOTHING`,
			userID, roleID, assignedBy)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "Role assigned to user successfully",
		})
	}
}

// RemoveRoleFromUserHandler - DELETE /users/:uuid/roles/:role_uuid - remove role from user
func RemoveRoleFromUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userUUID := c.Params("uuid")
		roleUUID := c.Params("role_uuid")
		ctx := context.Background()

		// Get user ID
		var userID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`,
			userUUID).Scan(&userID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Get role ID
		var roleID int
		err = db.QueryRow(ctx, `SELECT id FROM roles WHERE uuid = $1 AND deleted_at IS NULL`,
			roleUUID).Scan(&roleID)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "Role not found")
		}

		// Remove assignment
		result, err := db.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`,
			userID, roleID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if result.RowsAffected() == 0 {
			return fiber.NewError(fiber.StatusNotFound, "Role not assigned to this user")
		}

		return c.JSON(fiber.Map{
			"message": "Role removed from user successfully",
		})
	}
}

// GetUserRolesHandler - GET /users/:uuid/roles - get user's roles
func GetUserRolesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userUUID := c.Params("uuid")
		ctx := context.Background()

		// Get user ID
		var userID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`,
			userUUID).Scan(&userID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Get roles
		rows, err := db.Query(ctx, `
			SELECT r.uuid, r.name, r.description, r.scope, r.is_system, r.created_at, r.updated_at
			FROM user_roles ur
			JOIN roles r ON ur.role_id = r.id
			WHERE ur.user_id = $1 AND r.deleted_at IS NULL
			ORDER BY r.name`, userID)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		roles := []RoleResponse{}
		for rows.Next() {
			var role RoleResponse
			if rows.Scan(&role.UUID, &role.Name, &role.Description,
				&role.Scope, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt) == nil {
				roles = append(roles, role)
			}
		}

		return c.JSON(fiber.Map{
			"roles": roles,
		})
	}
}
