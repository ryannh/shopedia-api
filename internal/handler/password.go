package handler

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	utils "shopedia-api/internal/util"
)

// ForgotPasswordHandler - request reset password, kirim email dengan token
func ForgotPasswordHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			Email string `json:"email"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		// Cek apakah email ada
		var userID int
		var isActive bool
		err := db.QueryRow(ctx,
			`SELECT id, is_active FROM users WHERE email = $1`,
			input.Email).Scan(&userID, &isActive)
		if err != nil {
			// Jangan reveal apakah email ada atau tidak (security)
			return c.JSON(fiber.Map{
				"message": "If the email exists, a reset link will be sent",
			})
		}

		if !isActive {
			return c.JSON(fiber.Map{
				"message": "If the email exists, a reset link will be sent",
			})
		}

		// Cek apakah sudah ada token yang belum expired
		var existingExpires time.Time
		err = db.QueryRow(ctx,
			`SELECT expires_at FROM password_reset_tokens
			 WHERE user_id = $1 AND is_used = FALSE AND expires_at > NOW()`,
			userID).Scan(&existingExpires)
		if err == nil {
			// Sudah ada token aktif
			return c.JSON(fiber.Map{
				"message":    "Reset link already sent",
				"expires_at": existingExpires.Format(time.RFC3339),
			})
		}

		// Buat token baru
		expiresAt := time.Now().Add(15 * time.Minute)
		var token string
		err = db.QueryRow(ctx,
			`INSERT INTO password_reset_tokens (user_id, expires_at)
			 VALUES ($1, $2) RETURNING token`,
			userID, expiresAt).Scan(&token)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Kirim email
		go sendPasswordResetEmail(input.Email, token)

		return c.JSON(fiber.Map{
			"message":    "If the email exists, a reset link will be sent",
			"expires_at": expiresAt.Format(time.RFC3339),
		})
	}
}

// ResetPasswordHandler - submit new password dengan token
func ResetPasswordHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			Token       string `json:"token"`
			NewPassword string `json:"new_password"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		if len(input.NewPassword) < 8 {
			return fiber.NewError(fiber.StatusBadRequest, "Password must be at least 8 characters")
		}

		ctx := context.Background()

		// Validasi token
		var tokenID, userID int
		err := db.QueryRow(ctx,
			`SELECT id, user_id FROM password_reset_tokens
			 WHERE token = $1 AND is_used = FALSE AND expires_at > NOW()`,
			input.Token).Scan(&tokenID, &userID)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid or expired token")
		}

		// Hash password baru
		hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Update password
		_, err = db.Exec(ctx,
			`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
			string(hash), userID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Mark token as used
		_, err = db.Exec(ctx,
			`UPDATE password_reset_tokens SET is_used = TRUE WHERE id = $1`,
			tokenID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Clear active session (force re-login)
		_ = utils.ClearActiveSession(ctx, db, userID, "")

		return c.JSON(fiber.Map{
			"message": "Password reset successfully, please login with new password",
		})
	}
}

// ChangePasswordHandler - change password untuk user yang sudah login
func ChangePasswordHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			OldPassword string `json:"old_password"`
			NewPassword string `json:"new_password"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		if len(input.NewPassword) < 8 {
			return fiber.NewError(fiber.StatusBadRequest, "Password must be at least 8 characters")
		}

		userID := c.Locals("userID").(int)
		ctx := context.Background()

		// Ambil password hash saat ini
		var currentHash string
		err := db.QueryRow(ctx,
			`SELECT password_hash FROM users WHERE id = $1`,
			userID).Scan(&currentHash)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Validasi old password
		if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(input.OldPassword)) != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Old password is incorrect")
		}

		// Hash password baru
		newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Update password
		_, err = db.Exec(ctx,
			`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
			string(newHash), userID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "Password changed successfully",
		})
	}
}

// Utility function untuk kirim email reset password
func sendPasswordResetEmail(email, token string) {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", os.Getenv("FRONTEND_URL"), token)

	auth := smtp.PlainAuth("", os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"), os.Getenv("SMTP_HOST"))
	to := []string{email}
	msg := []byte(fmt.Sprintf(
		"Subject: Reset Your Password\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n"+
			"\r\n"+
			"<h2>Reset Password</h2>"+
			"<p>Click the link below to reset your password:</p>"+
			"<p><a href=\"%s\">Reset Password</a></p>"+
			"<p>Or copy this link: %s</p>"+
			"<p>This link will expire in 15 minutes.</p>"+
			"<p>If you didn't request this, please ignore this email.</p>",
		resetURL, resetURL))

	smtp.SendMail(
		fmt.Sprintf("%s:%s", os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT")),
		auth,
		"noreply@myapp.com",
		to,
		msg,
	)
}
