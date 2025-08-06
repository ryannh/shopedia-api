package handler

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/smtp"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func RegisterHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type RegisterInput struct {
			Email    string `json:"email"`
			Fullname string `json:"fullname"`
			Password string `json:"password"`
		}

		var input RegisterInput
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		// Cek apakah user sudah ada
		var isActive bool
		err := db.QueryRow(ctx, "SELECT is_active FROM users WHERE email = $1", input.Email).Scan(&isActive)
		if err == nil {
			if isActive {
				return fiber.NewError(fiber.StatusBadRequest, "Email already registered")
			}
			// Sudah ada tapi belum aktif → lanjut
		} else {
			// Belum ada → insert user baru
			hash, error := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
			if error != nil {
				return fiber.ErrInternalServerError
			}
			_, err = db.Exec(ctx, "INSERT INTO users (email, full_name, password_hash, is_active) VALUES ($1, $2, $3, FALSE)",
				input.Email, input.Fullname, string(hash))
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		// Add roles
		var userID int
		_ = db.QueryRow(ctx, "SELECT id FROM users WHERE email=$1", input.Email).Scan(&userID)

		var roleID int
		_ = db.QueryRow(ctx, `SELECT id FROM roles WHERE name='end_user'`).Scan(&roleID)
		if roleID == 0 {
			return fiber.NewError(fiber.StatusInternalServerError, "Role 'end_user' not found")
		}

		_, err = db.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`,
			userID, roleID)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "insert user roles failed")
		}

		// Buat OTP
		var expiresAt time.Time
		err = db.QueryRow(ctx, `
			SELECT expires_at FROM otp_codes 
			WHERE user_id=$1 AND is_used=FALSE AND expires_at > NOW()`,
			userID).Scan(&expiresAt)
		if err == nil {
			// Sudah ada OTP aktif → return expired_otp_at
		} else {
			// Insert OTP baru
			otp := generateOTP()
			expiresAt = time.Now().Add(2 * time.Minute)
			_, err = db.Exec(ctx, `
				INSERT INTO otp_codes (user_id, otp_code, expires_at) VALUES ($1, $2, $3)`,
				userID, otp, expiresAt)
			if err != nil {
				return fiber.NewError(fiber.StatusInternalServerError, "Insert otp code failed")
			}
			sendOTP(input.Email, otp)
		}

		// Generate register_access_token
		var userUUID string
		_ = db.QueryRow(ctx, "SELECT uuid FROM users WHERE id=$1", userID).Scan(&userUUID)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   userID,
			"user_uuid": userUUID,
			"exp":       expiresAt.Unix(),
			"iat":       time.Now().Unix(), // issued at
			"nbf":       time.Now().Unix(), // not before
		})
		tokenString, _ := token.SignedString([]byte(os.Getenv("JWT_SECRET")))

		return c.JSON(fiber.Map{
			"register_access_token": tokenString,
			"expired_otp_at":        expiresAt.Format(time.RFC3339),
		})
	}
}

func VerifyOTPHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			RegisterAccessToken string `json:"register_access_token"`
			OtpCode             string `json:"otp_code"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		token, err := jwt.Parse(input.RegisterAccessToken, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !token.Valid {
			return fiber.ErrUnauthorized
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := int(claims["user_id"].(float64))

		ctx := context.Background()
		var otpID int
		err = db.QueryRow(ctx, `
			SELECT id FROM otp_codes 
			WHERE user_id=$1 AND otp_code=$2 AND is_used=FALSE AND expires_at > NOW()`,
			userID, input.OtpCode).Scan(&otpID)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "Invalid or expired OTP")
		}

		_, err = db.Exec(ctx, `UPDATE users SET is_active=TRUE WHERE id=$1`, userID)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "update user is_active failed")
		}
		_, err = db.Exec(ctx, `UPDATE otp_codes SET is_used=TRUE WHERE id=$1`, otpID)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Update otp code is_used failed")
		}

		return c.JSON(fiber.Map{"message": "OTP verified, account activated"})
	}
}

func RequestNewOTPHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			RegisterAccessToken string `json:"register_access_token"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		token, err := jwt.Parse(input.RegisterAccessToken, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		fmt.Println(err)
		if err != nil || !token.Valid {
			return fiber.ErrUnauthorized
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := int(claims["user_id"].(float64))

		ctx := context.Background()

		var expiresAt time.Time
		err = db.QueryRow(ctx, `
			SELECT expires_at FROM otp_codes 
			WHERE user_id=$1 AND is_used=FALSE AND expires_at > NOW()`,
			userID).Scan(&expiresAt)
		if err == nil {
			return c.JSON(fiber.Map{"expired_otp_at": expiresAt})
		}

		// Buat OTP baru
		otp := generateOTP()
		expiresAt = time.Now().Add(2 * time.Minute)
		_, err = db.Exec(ctx, `
			INSERT INTO otp_codes (user_id, otp_code, expires_at) VALUES ($1, $2, $3)`,
			userID, otp, expiresAt)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Insert otp code failed")
		}

		var email string
		_ = db.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, userID).Scan(&email)
		sendOTP(email, otp)

		return c.JSON(fiber.Map{"expired_otp_at": expiresAt})
	}
}

// Utility
func generateOTP() string {
	const chars = "0123456789"
	result := make([]byte, 6)
	rand.Read(result)
	for i, b := range result {
		result[i] = chars[b%byte(len(chars))]
	}
	return string(result)
}

func sendOTP(email, otp string) {
	auth := smtp.PlainAuth("", os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"), os.Getenv("SMTP_HOST"))
	to := []string{email}
	msg := []byte("Subject: OTP Code\n\nYour OTP code is: " + otp)
	smtp.SendMail(fmt.Sprintf("%s:%s", os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT")), auth,
		"noreply@myapp.com", to, msg)
}
