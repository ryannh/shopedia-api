package utils

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenType string

const (
	TokenTypeAccess   TokenType = "access"
	TokenTypeRegister TokenType = "register"
)

type TokenClaims struct {
	jwt.RegisteredClaims
	UserID   int       `json:"user_id"`
	UserUUID string    `json:"user_uuid"`
	Roles    []string  `json:"roles,omitempty"`
	Type     TokenType `json:"type"`
}

// GenerateAccessToken - untuk login, dipakai di semua endpoint yang butuh authorization
func GenerateAccessToken(userID int, userUUID string, roles []string, expires time.Time) (string, string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", "", errors.New("JWT_SECRET not set")
	}

	now := time.Now()
	jti := uuid.New().String()

	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID:   userID,
		UserUUID: userUUID,
		Roles:    roles,
		Type:     TokenTypeAccess,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	return tokenString, jti, nil
}

// GenerateRegisterToken - untuk register, hanya dipakai untuk validasi/request OTP
func GenerateRegisterToken(userID int, userUUID string, expires time.Time) (string, string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", "", errors.New("JWT_SECRET not set")
	}

	now := time.Now()
	jti := uuid.New().String()

	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID:   userID,
		UserUUID: userUUID,
		Type:     TokenTypeRegister,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	return tokenString, jti, nil
}

// ParseToken - parse dan validasi token
func ParseToken(tokenString string) (*TokenClaims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET not set")
	}

	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// IsTokenRevoked - cek apakah token sudah di-revoke
func IsTokenRevoked(ctx context.Context, db *pgxpool.Pool, jti string) bool {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM revoked_tokens WHERE jti = $1)`,
		jti).Scan(&exists)
	if err != nil {
		return true // fail-safe: anggap revoked jika error
	}
	return exists
}

// RevokeToken - revoke token dengan menyimpan jti ke database
func RevokeToken(ctx context.Context, db *pgxpool.Pool, jti string, userID int, expiresAt time.Time) error {
	_, err := db.Exec(ctx,
		`INSERT INTO revoked_tokens (jti, user_id, expires_at) VALUES ($1, $2, $3)
		 ON CONFLICT (jti) DO NOTHING`,
		jti, userID, expiresAt)
	return err
}

// RevokeAllUserTokens - revoke semua token user (untuk force logout)
func RevokeAllUserTokens(ctx context.Context, db *pgxpool.Pool, userID int) error {
	_, err := db.Exec(ctx,
		`UPDATE revoked_tokens SET revoked_at = NOW() WHERE user_id = $1`,
		userID)
	return err
}

// CleanupExpiredTokens - hapus token yang sudah expired dari tabel revoked_tokens
func CleanupExpiredTokens(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx,
		`DELETE FROM revoked_tokens WHERE expires_at < NOW()`)
	return err
}

// ============================================
// Single Active Session Functions
// ============================================

// SetActiveSession - simpan session aktif baru dan revoke session lama
func SetActiveSession(ctx context.Context, db *pgxpool.Pool, userID int, jti string, expiresAt time.Time) error {
	// Ambil JTI lama jika ada
	var oldJTI string
	var oldExpiresAt time.Time
	err := db.QueryRow(ctx,
		`SELECT jti, expires_at FROM active_sessions WHERE user_id = $1`,
		userID).Scan(&oldJTI, &oldExpiresAt)

	// Jika ada session lama, revoke tokennya
	if err == nil && oldJTI != "" {
		_ = RevokeToken(ctx, db, oldJTI, userID, oldExpiresAt)
	}

	// Insert atau update session baru
	_, err = db.Exec(ctx,
		`INSERT INTO active_sessions (user_id, jti, expires_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE SET jti = $2, expires_at = $3, created_at = NOW()`,
		userID, jti, expiresAt)
	return err
}

// IsActiveSession - cek apakah JTI adalah session yang aktif
func IsActiveSession(ctx context.Context, db *pgxpool.Pool, userID int, jti string) bool {
	var activeJTI string
	err := db.QueryRow(ctx,
		`SELECT jti FROM active_sessions WHERE user_id = $1`,
		userID).Scan(&activeJTI)
	if err != nil {
		return false
	}
	return activeJTI == jti
}

// ClearActiveSession - hapus session aktif (untuk logout)
func ClearActiveSession(ctx context.Context, db *pgxpool.Pool, userID int) error {
	_, err := db.Exec(ctx,
		`DELETE FROM active_sessions WHERE user_id = $1`,
		userID)
	return err
}
