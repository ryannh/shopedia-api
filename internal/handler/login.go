package handler

import (
	"context"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Generic Login Handler → pakai param: `mode` → "app" atau "admin"
func LoginHandler(db *pgxpool.Pool, mode string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		var userID int
		var passwordHash string
		var isActive bool

		err := db.QueryRow(ctx,
			`SELECT id, password_hash, is_active FROM users WHERE email=$1`,
			input.Email).Scan(&userID, &passwordHash, &isActive)
		if err != nil {
			return fiber.ErrUnauthorized
		}

		if !isActive {
			return fiber.NewError(fiber.StatusUnauthorized, "Account not active")
		}

		// Mode: Apps → harus role end_user
		// Mode: Admin → harus ada role admin atau super_admin

		if mode == "app" {
			var role string
			err := db.QueryRow(ctx,
				`SELECT r.name FROM user_roles ur 
			 JOIN roles r ON ur.role_id = r.id
			 WHERE ur.user_id = $1 LIMIT 1`, userID).Scan(&role)
			if err != nil || role != "end_user" {
				return fiber.ErrForbidden
			}
		} else if mode == "admin" {
			var role string
			err := db.QueryRow(ctx,
				`SELECT r.name FROM user_roles ur 
			 JOIN roles r ON ur.role_id = r.id
			 WHERE ur.user_id = $1 LIMIT 1`, userID).Scan(&role)
			if err != nil || (role != "admin" && role != "super_admin") {
				return fiber.ErrForbidden
			}
		}

		// Verify password
		if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
			return fiber.ErrUnauthorized
		}

		// Generate JWT
		var userUUID string
		_ = db.QueryRow(ctx, "SELECT uuid FROM users WHERE id=$1", userID).Scan(&userUUID)
		expires := time.Now().Add(24 * time.Hour)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   userID,
			"user_uuid": userUUID,
			"exp":       expires.Unix(),
			"iat":       time.Now().Unix(), // issued at
			"nbf":       time.Now().Unix(), // not before
		})
		tokenString, _ := token.SignedString([]byte(os.Getenv("JWT_SECRET")))

		return c.JSON(fiber.Map{
			"access_token": tokenString,
			"expires_at":   expires.Format(time.RFC3339),
		})
	}
}
