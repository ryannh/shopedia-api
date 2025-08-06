package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func JWTProtected() fiber.Handler {
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
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !token.Valid {
			return fiber.ErrUnauthorized
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := int(claims["user_id"].(float64))

		// Inject ke ctx
		c.Locals("userID", userID)

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
