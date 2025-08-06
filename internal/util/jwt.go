package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateJWT(userID int, userUUID string, expires time.Time, roles []string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":   userID,
		"user_uuid": userUUID,
		"roles":     roles,
		"exp":       expires.Unix(), // expired in 24 jam
		"iat":       now.Unix(),     // issued at
		"nbf":       now.Unix(),     // not before
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
