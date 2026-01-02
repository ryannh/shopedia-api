package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	utils "shopedia-api/internal/util"
)

func JWTProtected(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return fiber.ErrUnauthorized
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return fiber.ErrUnauthorized
		}

		tokenString := parts[1]

		// Parse token menggunakan utils
		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			return fiber.ErrUnauthorized
		}

		// Validasi tipe token - hanya access token yang boleh
		if claims.Type != utils.TokenTypeAccess {
			return fiber.NewError(fiber.StatusUnauthorized, "Invalid token type")
		}

		// Cek apakah token sudah di-revoke
		jti := claims.ID
		if utils.IsTokenRevoked(c.Context(), db, jti) {
			return fiber.NewError(fiber.StatusUnauthorized, "Token has been revoked")
		}

		// Cek apakah token adalah active session (Single Active Session)
		if !utils.IsActiveSession(c.Context(), db, claims.UserID, jti) {
			return fiber.NewError(fiber.StatusUnauthorized, "Session expired, please login again")
		}

		// Cek status user di database (is_active, is_banned, deleted_at)
		var isActive bool
		var isBanned bool
		var isDeleted bool
		err = db.QueryRow(c.Context(), `
			SELECT is_active, COALESCE(is_banned, FALSE), (deleted_at IS NOT NULL)
			FROM users WHERE id = $1`,
			claims.UserID).Scan(&isActive, &isBanned, &isDeleted)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "User not found")
		}

		if isDeleted {
			return fiber.NewError(fiber.StatusUnauthorized, "Account has been deleted")
		}

		if isBanned {
			return fiber.NewError(fiber.StatusUnauthorized, "Account has been banned")
		}

		if !isActive {
			return fiber.NewError(fiber.StatusUnauthorized, "Account is not active")
		}

		// Inject ke ctx
		c.Locals("userID", claims.UserID)
		c.Locals("userUUID", claims.UserUUID)
		c.Locals("roles", claims.Roles)
		c.Locals("jti", jti)
		c.Locals("tokenExp", claims.ExpiresAt.Time)

		return c.Next()
	}
}

// RoleRequired middleware - check if user has any of the allowed roles
func RoleRequired(db *pgxpool.Pool, allowedRoles []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID").(int)

		// Get all user roles
		rows, err := db.Query(c.Context(),
			`SELECT r.name FROM user_roles ur
			 JOIN roles r ON ur.role_id = r.id
			 WHERE ur.user_id = $1 AND r.deleted_at IS NULL`,
			userID)
		if err != nil {
			return fiber.ErrForbidden
		}
		defer rows.Close()

		userRoles := []string{}
		for rows.Next() {
			var roleName string
			if rows.Scan(&roleName) == nil {
				userRoles = append(userRoles, roleName)
			}
		}

		// super_admin has access to everything
		for _, role := range userRoles {
			if role == "super_admin" {
				return c.Next()
			}
		}

		// Check if user has any of the allowed roles
		for _, userRole := range userRoles {
			for _, allowedRole := range allowedRoles {
				if userRole == allowedRole {
					return c.Next()
				}
			}
		}

		return fiber.ErrForbidden
	}
}

// PermissionRequired middleware - check if user has any of the required permissions
func PermissionRequired(db *pgxpool.Pool, requiredPermissions []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID").(int)

		// Check if user is super_admin (has all permissions)
		var isSuperAdmin bool
		err := db.QueryRow(c.Context(),
			`SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.name = 'super_admin' AND r.deleted_at IS NULL
			)`, userID).Scan(&isSuperAdmin)
		if err != nil {
			return fiber.ErrForbidden
		}

		if isSuperAdmin {
			return c.Next()
		}

		// Get user permissions through roles
		rows, err := db.Query(c.Context(),
			`SELECT DISTINCT p.name FROM user_roles ur
			 JOIN role_permissions rp ON ur.role_id = rp.role_id
			 JOIN permissions p ON rp.permission_id = p.id
			 WHERE ur.user_id = $1 AND p.deleted_at IS NULL`,
			userID)
		if err != nil {
			return fiber.ErrForbidden
		}
		defer rows.Close()

		userPermissions := make(map[string]bool)
		for rows.Next() {
			var permName string
			if rows.Scan(&permName) == nil {
				userPermissions[permName] = true
			}
		}

		// Check if user has any of the required permissions
		for _, reqPerm := range requiredPermissions {
			if userPermissions[reqPerm] {
				return c.Next()
			}

			// Support wildcard matching: if required is "finance.view",
			// user having "finance.*" should pass
			module := strings.Split(reqPerm, ".")[0]
			if userPermissions[module+".*"] {
				return c.Next()
			}
		}

		return fiber.NewError(fiber.StatusForbidden, "Insufficient permissions")
	}
}

// ScopeRequired middleware - check if user's role is in the required scope
func ScopeRequired(db *pgxpool.Pool, scope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID").(int)

		// Check if user has any role in the required scope
		var hasScope bool
		err := db.QueryRow(c.Context(),
			`SELECT EXISTS(
				SELECT 1 FROM user_roles ur
				JOIN roles r ON ur.role_id = r.id
				WHERE ur.user_id = $1 AND r.scope = $2 AND r.deleted_at IS NULL
			)`, userID, scope).Scan(&hasScope)
		if err != nil {
			return fiber.ErrForbidden
		}

		if !hasScope {
			return fiber.NewError(fiber.StatusForbidden, "Access denied for this scope")
		}

		return c.Next()
	}
}
