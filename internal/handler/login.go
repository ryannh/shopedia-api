package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	utils "shopedia-api/internal/util"
)

// LoginHandler - Generic Login Handler untuk "app" atau "admin"
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

		// Verify password terlebih dahulu sebelum cek role
		if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
			return fiber.ErrUnauthorized
		}

		// Ambil role user
		var role string
		err = db.QueryRow(ctx,
			`SELECT r.name FROM user_roles ur
			 JOIN roles r ON ur.role_id = r.id
			 WHERE ur.user_id = $1 LIMIT 1`, userID).Scan(&role)
		if err != nil {
			return fiber.ErrForbidden
		}

		// Validasi role berdasarkan mode
		if mode == "app" && role != "end_user" {
			return fiber.ErrForbidden
		} else if mode == "admin" && (role != "admin" && role != "super_admin") {
			return fiber.ErrForbidden
		}

		// Ambil user UUID
		var userUUID string
		err = db.QueryRow(ctx, "SELECT uuid FROM users WHERE id=$1", userID).Scan(&userUUID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Generate Access Token dengan JTI
		expires := time.Now().Add(24 * time.Hour)
		tokenString, jti, err := utils.GenerateAccessToken(userID, userUUID, []string{role}, expires)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Set active session (revoke token lama jika ada)
		err = utils.SetActiveSession(ctx, db, userID, jti, expires)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"access_token": tokenString,
			"expires_at":   expires.Format(time.RFC3339),
		})
	}
}
