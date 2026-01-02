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

// ACL middleware
func RoleRequired(db *pgxpool.Pool, allowedRoles []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID").(int)

		// Query role name user
		var roleName string
		err := db.QueryRow(c.Context(),
			`SELECT r.name FROM user_roles ur
			 JOIN roles r ON ur.role_id = r.id
			 WHERE ur.user_id = $1 LIMIT 1`,
			userID).Scan(&roleName)
		if err != nil {
			return fiber.ErrForbidden
		}

		for _, r := range allowedRoles {
			if r == roleName {
				return c.Next()
			}
		}

		return fiber.ErrForbidden
	}
}
