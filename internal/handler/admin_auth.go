package handler

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func AdminRegisterHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fmt.Println("Hai")
		type Input struct {
			Email    string `json:"email"`
			Fullname string `json:"fullname"`
			Password string `json:"password"`
		}

		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		// Check if already exists
		var exists bool
		err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)`, input.Email).Scan(&exists)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		if exists {
			return fiber.NewError(fiber.StatusBadRequest, "Email already exists")
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Insert user
		_, err = db.Exec(ctx,
			`INSERT INTO users (email, full_name, password_hash, is_active, is_invited) VALUES ($1, $2, $3, TRUE, FALSE)`,
			input.Email, input.Fullname, string(hash))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Could not create user")
		}

		var userID int
		_ = db.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, input.Email).Scan(&userID)

		// Assign super_admin role
		var superAdminRoleID int
		_ = db.QueryRow(ctx, `SELECT id FROM roles WHERE name='super_admin'`).Scan(&superAdminRoleID)
		if superAdminRoleID == 0 {
			return fiber.NewError(fiber.StatusInternalServerError, "Role 'super_admin' not found")
		}

		_, err = db.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`,
			userID, superAdminRoleID)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "user_roles insert failed")
		}

		return c.JSON(fiber.Map{"message": "Super admin created successfully"})
	}
}

func InviteUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			Email  string `json:"email"`
			RoleID int    `json:"role_id"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		_, err := db.Exec(ctx,
			`INSERT INTO users (email, is_active, is_invited, invited_at) VALUES ($1, FALSE, TRUE, NOW())`,
			input.Email)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "User insert failed")
		}

		var userID int
		_ = db.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, input.Email).Scan(&userID)

		_, err = db.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, input.RoleID)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "user_roles insert failed")
		}

		inviteToken := uuid.New().String()
		expires := time.Now().Add(24 * time.Hour)
		_, err = db.Exec(ctx, `
			INSERT INTO invite_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
			userID, inviteToken, expires)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "invite_tokens insert failed")
		}

		sendInvite(input.Email, inviteToken)

		return c.JSON(fiber.Map{"message": "Invite sent"})
	}
}

func AcceptInviteHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			InviteToken string `json:"invite_token"`
			Password    string `json:"password"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()
		var userID int
		err := db.QueryRow(ctx, `
			SELECT user_id FROM invite_tokens 
			WHERE token=$1 AND is_used=FALSE AND expires_at > NOW()`,
			input.InviteToken).Scan(&userID)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "Invalid or expired invite token")
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		_, err = db.Exec(ctx, `UPDATE users SET password_hash=$1, is_active=TRUE WHERE id=$2`, string(hash), userID)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Update user password failed")
		}
		_, err = db.Exec(ctx, `UPDATE invite_tokens SET is_used=TRUE WHERE token=$1`, input.InviteToken)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Update invite_token failed")
		}

		return c.JSON(fiber.Map{"message": "Account activated, you can login now"})
	}
}

func sendInvite(email, token string) {
	auth := smtp.PlainAuth("", os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"), os.Getenv("SMTP_HOST"))
	to := []string{email}
	msg := []byte("Subject: Admin Invite\n\nAccept your invite: https://yourapp.com/accept-invite?token=" + token)
	smtp.SendMail(fmt.Sprintf("%s:%s", os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT")), auth,
		"noreply@myapp.com", to, msg)
}
