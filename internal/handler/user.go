package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	utils "shopedia-api/internal/util"
)

// ============================================
// User Profile Response
// ============================================

type UserProfile struct {
	UUID      string     `json:"uuid"`
	Email     string     `json:"email"`
	FullName  *string    `json:"full_name"`
	IsActive  bool       `json:"is_active"`
	Roles     []string   `json:"roles"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// ============================================
// Profile Handlers (untuk user sendiri)
// ============================================

// GetProfileHandler - GET /me - get current user profile
func GetProfileHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID").(int)
		ctx := context.Background()

		var profile UserProfile
		err := db.QueryRow(ctx, `
			SELECT uuid, email, full_name, is_active, created_at, updated_at, last_login
			FROM users WHERE id = $1 AND deleted_at IS NULL`,
			userID).Scan(
			&profile.UUID, &profile.Email, &profile.FullName,
			&profile.IsActive, &profile.CreatedAt, &profile.UpdatedAt, &profile.LastLogin)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Get roles
		rows, err := db.Query(ctx, `
			SELECT r.name FROM user_roles ur
			JOIN roles r ON ur.role_id = r.id
			WHERE ur.user_id = $1`, userID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var role string
				if rows.Scan(&role) == nil {
					profile.Roles = append(profile.Roles, role)
				}
			}
		}

		return c.JSON(profile)
	}
}

// UpdateProfileHandler - PUT /me - update current user profile
func UpdateProfileHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			FullName string `json:"full_name"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		userID := c.Locals("userID").(int)
		ctx := context.Background()

		_, err := db.Exec(ctx, `
			UPDATE users SET full_name = $1, updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL`,
			input.FullName, userID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "Profile updated successfully",
		})
	}
}

// ============================================
// Admin User Management Handlers
// ============================================

type UserListItem struct {
	UUID      string     `json:"uuid"`
	Email     string     `json:"email"`
	FullName  *string    `json:"full_name"`
	IsActive  bool       `json:"is_active"`
	IsInvited bool       `json:"is_invited"`
	Roles     []string   `json:"roles"`
	CreatedAt time.Time  `json:"created_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalItems int         `json:"total_items"`
	TotalPages int         `json:"total_pages"`
}

// ListUsersHandler - GET /users - list internal users only (exclude end_user)
func ListUsersHandler(db *pgxpool.Pool) fiber.Handler {
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
		roleFilter := c.Query("role", "")
		activeFilter := c.Query("is_active", "")

		// Build query - exclude end_user role and soft-deleted users
		baseQuery := `FROM users u WHERE u.deleted_at IS NULL AND NOT EXISTS (
			SELECT 1 FROM user_roles ur
			JOIN roles r ON ur.role_id = r.id
			WHERE ur.user_id = u.id AND r.name = 'end_user'
		)`
		args := []interface{}{}
		argCount := 0

		if search != "" {
			argCount++
			baseQuery += ` AND (u.email ILIKE $` + strconv.Itoa(argCount) + ` OR u.full_name ILIKE $` + strconv.Itoa(argCount) + `)`
			args = append(args, "%"+search+"%")
		}

		if activeFilter != "" {
			argCount++
			baseQuery += ` AND u.is_active = $` + strconv.Itoa(argCount)
			isActive := activeFilter == "true"
			args = append(args, isActive)
		}

		if roleFilter != "" {
			argCount++
			baseQuery += ` AND EXISTS (
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = u.id AND r.name = $` + strconv.Itoa(argCount) + `)`
			args = append(args, roleFilter)
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
		dataQuery := `SELECT u.id, u.uuid, u.email, u.full_name, u.is_active, u.is_invited, u.created_at, u.last_login ` +
			baseQuery + ` ORDER BY u.created_at DESC LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg)
		args = append(args, limit, offset)

		rows, err := db.Query(ctx, dataQuery, args...)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		users := []UserListItem{}
		userIDs := []int{}
		userIDToIndex := make(map[int]int)

		idx := 0
		for rows.Next() {
			var user UserListItem
			var userID int
			err := rows.Scan(&userID, &user.UUID, &user.Email, &user.FullName,
				&user.IsActive, &user.IsInvited, &user.CreatedAt, &user.LastLogin)
			if err != nil {
				continue
			}
			users = append(users, user)
			userIDs = append(userIDs, userID)
			userIDToIndex[userID] = idx
			idx++
		}

		// Get roles for all users
		if len(userIDs) > 0 {
			roleRows, err := db.Query(ctx, `
				SELECT ur.user_id, r.name FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = ANY($1)`, userIDs)
			if err == nil {
				defer roleRows.Close()
				for roleRows.Next() {
					var userID int
					var role string
					if roleRows.Scan(&userID, &role) == nil {
						if idx, ok := userIDToIndex[userID]; ok {
							users[idx].Roles = append(users[idx].Roles, role)
						}
					}
				}
			}
		}

		totalPages := (totalItems + limit - 1) / limit

		return c.JSON(PaginatedResponse{
			Data:       users,
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		})
	}
}

// GetUserHandler - GET /users/:uuid - get internal user by uuid (exclude end_user)
func GetUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUUID := c.Params("uuid")
		ctx := context.Background()

		var profile UserProfile
		var targetUserID int
		err := db.QueryRow(ctx, `
			SELECT id, uuid, email, full_name, is_active, created_at, updated_at, last_login
			FROM users WHERE uuid = $1 AND deleted_at IS NULL`,
			targetUUID).Scan(
			&targetUserID, &profile.UUID, &profile.Email, &profile.FullName,
			&profile.IsActive, &profile.CreatedAt, &profile.UpdatedAt, &profile.LastLogin)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if user is end_user (not allowed)
		var isEndUser bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'end_user'
			)`, targetUserID).Scan(&isEndUser)
		if err != nil || isEndUser {
			return fiber.NewError(fiber.StatusForbidden, "Cannot access end_user via this endpoint")
		}

		// Get roles
		rows, err := db.Query(ctx, `
			SELECT r.name FROM user_roles ur
			JOIN roles r ON ur.role_id = r.id
			WHERE ur.user_id = $1`, targetUserID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var role string
				if rows.Scan(&role) == nil {
					profile.Roles = append(profile.Roles, role)
				}
			}
		}

		return c.JSON(profile)
	}
}

// UpdateUserHandler - PUT /users/:uuid - update internal user by admin (exclude end_user)
func UpdateUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUUID := c.Params("uuid")

		type Input struct {
			FullName *string `json:"full_name"`
			Email    *string `json:"email"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		// Check if user exists and get ID (exclude soft-deleted)
		var targetUserID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`, targetUUID).Scan(&targetUserID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if user is end_user (not allowed)
		var isEndUser bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'end_user'
			)`, targetUserID).Scan(&isEndUser)
		if err != nil || isEndUser {
			return fiber.NewError(fiber.StatusForbidden, "Cannot modify end_user via this endpoint")
		}

		// Build update query
		if input.FullName != nil {
			_, err = db.Exec(ctx, `UPDATE users SET full_name = $1, updated_at = NOW() WHERE id = $2`,
				*input.FullName, targetUserID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Email != nil {
			// Check if email already exists
			var emailExists bool
			err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND id != $2)`,
				*input.Email, targetUserID).Scan(&emailExists)
			if emailExists {
				return fiber.NewError(fiber.StatusBadRequest, "Email already in use")
			}

			_, err = db.Exec(ctx, `UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2`,
				*input.Email, targetUserID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		return c.JSON(fiber.Map{
			"message": "User updated successfully",
		})
	}
}

// DeleteUserHandler - DELETE /users/:uuid - soft delete internal user (exclude end_user)
func DeleteUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUUID := c.Params("uuid")
		currentUserUUID := c.Locals("userUUID").(string)

		// Prevent self-delete
		if targetUUID == currentUserUUID {
			return fiber.NewError(fiber.StatusBadRequest, "Cannot delete yourself")
		}

		ctx := context.Background()

		// Check if user exists and get ID (exclude already soft-deleted)
		var targetUserID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`, targetUUID).Scan(&targetUserID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if user is end_user (not allowed)
		var isEndUser bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'end_user'
			)`, targetUserID).Scan(&isEndUser)
		if err != nil || isEndUser {
			return fiber.NewError(fiber.StatusForbidden, "Cannot delete end_user via this endpoint")
		}

		// Clear active session first
		_ = utils.ClearActiveSession(ctx, db, targetUserID, "")

		// Soft delete user (set deleted_at timestamp)
		_, err = db.Exec(ctx, `UPDATE users SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, targetUserID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "User deleted successfully",
		})
	}
}

// ActivateUserHandler - POST /users/:uuid/activate - activate internal user (exclude end_user)
func ActivateUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUUID := c.Params("uuid")
		ctx := context.Background()

		// Check if user exists and get ID (exclude soft-deleted)
		var targetUserID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`, targetUUID).Scan(&targetUserID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if user is end_user (not allowed)
		var isEndUser bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'end_user'
			)`, targetUserID).Scan(&isEndUser)
		if err != nil || isEndUser {
			return fiber.NewError(fiber.StatusForbidden, "Cannot activate end_user via this endpoint")
		}

		result, err := db.Exec(ctx, `UPDATE users SET is_active = TRUE, updated_at = NOW() WHERE id = $1`, targetUserID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if result.RowsAffected() == 0 {
			return fiber.ErrNotFound
		}

		return c.JSON(fiber.Map{
			"message": "User activated successfully",
		})
	}
}

// DeactivateUserHandler - POST /users/:uuid/deactivate - deactivate internal user (exclude end_user)
func DeactivateUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUUID := c.Params("uuid")
		currentUserUUID := c.Locals("userUUID").(string)

		// Prevent self-deactivate
		if targetUUID == currentUserUUID {
			return fiber.NewError(fiber.StatusBadRequest, "Cannot deactivate yourself")
		}

		ctx := context.Background()

		// Get target user ID for session clearing (exclude soft-deleted)
		var targetUserID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`, targetUUID).Scan(&targetUserID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if user is end_user (not allowed)
		var isEndUser bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'end_user'
			)`, targetUserID).Scan(&isEndUser)
		if err != nil || isEndUser {
			return fiber.NewError(fiber.StatusForbidden, "Cannot deactivate end_user via this endpoint")
		}

		result, err := db.Exec(ctx, `UPDATE users SET is_active = FALSE, updated_at = NOW() WHERE id = $1`, targetUserID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if result.RowsAffected() == 0 {
			return fiber.ErrNotFound
		}

		// Clear active session (force logout)
		_ = utils.ClearActiveSession(ctx, db, targetUserID, "")

		return c.JSON(fiber.Map{
			"message": "User deactivated successfully",
		})
	}
}

// ============================================
// End User Management Handlers (only end_user role)
// ============================================

// EndUserListItem - response structure for end user list
type EndUserListItem struct {
	UUID      string     `json:"uuid"`
	Email     string     `json:"email"`
	FullName  *string    `json:"full_name"`
	IsActive  bool       `json:"is_active"`
	IsBanned  bool       `json:"is_banned"`
	CreatedAt time.Time  `json:"created_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// ListEndUsersHandler - GET /end-users - list end_users only with pagination
func ListEndUsersHandler(db *pgxpool.Pool) fiber.Handler {
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
		activeFilter := c.Query("is_active", "")
		bannedFilter := c.Query("is_banned", "")

		// Build query - only end_user role and exclude soft-deleted
		baseQuery := `FROM users u WHERE u.deleted_at IS NULL AND EXISTS (
			SELECT 1 FROM user_roles ur
			JOIN roles r ON ur.role_id = r.id
			WHERE ur.user_id = u.id AND r.name = 'end_user'
		)`
		args := []interface{}{}
		argCount := 0

		if search != "" {
			argCount++
			baseQuery += ` AND (u.email ILIKE $` + strconv.Itoa(argCount) + ` OR u.full_name ILIKE $` + strconv.Itoa(argCount) + `)`
			args = append(args, "%"+search+"%")
		}

		if activeFilter != "" {
			argCount++
			baseQuery += ` AND u.is_active = $` + strconv.Itoa(argCount)
			isActive := activeFilter == "true"
			args = append(args, isActive)
		}

		if bannedFilter != "" {
			argCount++
			baseQuery += ` AND u.is_banned = $` + strconv.Itoa(argCount)
			isBanned := bannedFilter == "true"
			args = append(args, isBanned)
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
		dataQuery := `SELECT u.uuid, u.email, u.full_name, u.is_active, u.is_banned, u.created_at, u.last_login ` +
			baseQuery + ` ORDER BY u.created_at DESC LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg)
		args = append(args, limit, offset)

		rows, err := db.Query(ctx, dataQuery, args...)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		users := []EndUserListItem{}
		for rows.Next() {
			var user EndUserListItem
			err := rows.Scan(&user.UUID, &user.Email, &user.FullName,
				&user.IsActive, &user.IsBanned, &user.CreatedAt, &user.LastLogin)
			if err != nil {
				continue
			}
			users = append(users, user)
		}

		totalPages := (totalItems + limit - 1) / limit

		return c.JSON(PaginatedResponse{
			Data:       users,
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		})
	}
}

// EndUserProfile - response structure for end user detail
type EndUserProfile struct {
	UUID      string     `json:"uuid"`
	Email     string     `json:"email"`
	FullName  *string    `json:"full_name"`
	IsActive  bool       `json:"is_active"`
	IsBanned  bool       `json:"is_banned"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// GetEndUserHandler - GET /end-users/:uuid - get end_user by uuid
func GetEndUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUUID := c.Params("uuid")
		ctx := context.Background()

		var profile EndUserProfile
		var targetUserID int
		err := db.QueryRow(ctx, `
			SELECT id, uuid, email, full_name, is_active, is_banned, created_at, updated_at, last_login
			FROM users WHERE uuid = $1 AND deleted_at IS NULL`,
			targetUUID).Scan(
			&targetUserID, &profile.UUID, &profile.Email, &profile.FullName,
			&profile.IsActive, &profile.IsBanned, &profile.CreatedAt, &profile.UpdatedAt, &profile.LastLogin)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if user is end_user
		var isEndUser bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'end_user'
			)`, targetUserID).Scan(&isEndUser)
		if err != nil || !isEndUser {
			return fiber.NewError(fiber.StatusForbidden, "User is not an end_user")
		}

		return c.JSON(profile)
	}
}

// BanEndUserHandler - POST /end-users/:uuid/ban - ban end_user
func BanEndUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUUID := c.Params("uuid")
		ctx := context.Background()

		// Check if user exists and get ID (exclude soft-deleted)
		var targetUserID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`, targetUUID).Scan(&targetUserID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if user is end_user
		var isEndUser bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'end_user'
			)`, targetUserID).Scan(&isEndUser)
		if err != nil || !isEndUser {
			return fiber.NewError(fiber.StatusForbidden, "User is not an end_user")
		}

		result, err := db.Exec(ctx, `UPDATE users SET is_banned = TRUE, updated_at = NOW() WHERE id = $1`, targetUserID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if result.RowsAffected() == 0 {
			return fiber.ErrNotFound
		}

		// Clear active session (force logout)
		_ = utils.ClearActiveSession(ctx, db, targetUserID, "")

		return c.JSON(fiber.Map{
			"message": "User banned successfully",
		})
	}
}

// UnbanEndUserHandler - POST /end-users/:uuid/unban - unban end_user
func UnbanEndUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUUID := c.Params("uuid")
		ctx := context.Background()

		// Check if user exists and get ID (exclude soft-deleted)
		var targetUserID int
		err := db.QueryRow(ctx, `SELECT id FROM users WHERE uuid = $1 AND deleted_at IS NULL`, targetUUID).Scan(&targetUserID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if user is end_user
		var isEndUser bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'end_user'
			)`, targetUserID).Scan(&isEndUser)
		if err != nil || !isEndUser {
			return fiber.NewError(fiber.StatusForbidden, "User is not an end_user")
		}

		result, err := db.Exec(ctx, `UPDATE users SET is_banned = FALSE, updated_at = NOW() WHERE id = $1`, targetUserID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if result.RowsAffected() == 0 {
			return fiber.ErrNotFound
		}

		return c.JSON(fiber.Map{
			"message": "User unbanned successfully",
		})
	}
}
