package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var ctx = context.Background()

// Cache key prefixes
const (
	PrefixCategory    = "cache:category:"
	PrefixCategories  = "cache:categories"
	PrefixSession     = "session:"
	PrefixOTP         = "otp:"
	PrefixRateLimit   = "ratelimit:"
	PrefixResetToken  = "reset_token:"
)

// Default TTLs
const (
	TTLCategories  = 1 * time.Hour
	TTLSession     = 24 * time.Hour
	TTLOTP         = 5 * time.Minute
	TTLResetToken  = 1 * time.Hour
	TTLRateLimit   = 1 * time.Minute
)

// InitRedis initializes the Redis client
func InitRedis() error {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	Client = redis.NewClient(opt)

	// Test connection
	_, err = Client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return nil
}

// CloseRedis closes the Redis client
func CloseRedis() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}

// ============================================
// Generic Cache Functions
// ============================================

// Set stores a value in cache with TTL
func Set(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return Client.Set(ctx, key, data, ttl).Err()
}

// Get retrieves a value from cache
func Get(key string, dest interface{}) error {
	data, err := Client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete removes a key from cache
func Delete(key string) error {
	return Client.Del(ctx, key).Err()
}

// DeletePattern removes all keys matching a pattern
func DeletePattern(pattern string) error {
	keys, err := Client.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return Client.Del(ctx, keys...).Err()
	}
	return nil
}

// Exists checks if a key exists
func Exists(key string) bool {
	result, err := Client.Exists(ctx, key).Result()
	return err == nil && result > 0
}

// ============================================
// Categories Cache
// ============================================

// SetCategories caches the categories list
func SetCategories(categories interface{}) error {
	return Set(PrefixCategories, categories, TTLCategories)
}

// GetCategories retrieves cached categories
func GetCategories(dest interface{}) error {
	return Get(PrefixCategories, dest)
}

// InvalidateCategories removes categories from cache
func InvalidateCategories() error {
	return Delete(PrefixCategories)
}

// ============================================
// Session Cache
// ============================================

type SessionData struct {
	UserID    int       `json:"user_id"`
	JTI       string    `json:"jti"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Scope     string    `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
}

// SetSession stores session data
func SetSession(jti string, session *SessionData) error {
	return Set(PrefixSession+jti, session, TTLSession)
}

// GetSession retrieves session data
func GetSession(jti string) (*SessionData, error) {
	var session SessionData
	err := Get(PrefixSession+jti, &session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// DeleteSession removes a session
func DeleteSession(jti string) error {
	return Delete(PrefixSession + jti)
}

// DeleteUserSessions removes all sessions for a user
func DeleteUserSessions(userID int) error {
	// Get all session keys
	keys, err := Client.Keys(ctx, PrefixSession+"*").Result()
	if err != nil {
		return err
	}

	// Check each session and delete if belongs to user
	for _, key := range keys {
		var session SessionData
		if err := Get(key[len(PrefixSession):], &session); err == nil {
			if session.UserID == userID {
				Client.Del(ctx, key)
			}
		}
	}
	return nil
}

// ============================================
// OTP Cache
// ============================================

type OTPData struct {
	Email     string    `json:"email"`
	OTP       string    `json:"otp"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
}

// SetOTP stores OTP data
func SetOTP(email string, otp string) error {
	data := &OTPData{
		Email:     email,
		OTP:       otp,
		Attempts:  0,
		CreatedAt: time.Now(),
	}
	return Set(PrefixOTP+email, data, TTLOTP)
}

// GetOTP retrieves OTP data
func GetOTP(email string) (*OTPData, error) {
	var data OTPData
	err := Get(PrefixOTP+email, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// IncrementOTPAttempts increases the attempt count
func IncrementOTPAttempts(email string) error {
	data, err := GetOTP(email)
	if err != nil {
		return err
	}
	data.Attempts++

	// Get remaining TTL
	ttl, err := Client.TTL(ctx, PrefixOTP+email).Result()
	if err != nil || ttl < 0 {
		ttl = TTLOTP
	}

	return Set(PrefixOTP+email, data, ttl)
}

// DeleteOTP removes OTP data
func DeleteOTP(email string) error {
	return Delete(PrefixOTP + email)
}

// ============================================
// Password Reset Token Cache
// ============================================

type ResetTokenData struct {
	UserID    int       `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

// SetResetToken stores password reset token
func SetResetToken(token string, data *ResetTokenData) error {
	return Set(PrefixResetToken+token, data, TTLResetToken)
}

// GetResetToken retrieves password reset token data
func GetResetToken(token string) (*ResetTokenData, error) {
	var data ResetTokenData
	err := Get(PrefixResetToken+token, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// DeleteResetToken removes password reset token
func DeleteResetToken(token string) error {
	return Delete(PrefixResetToken + token)
}

// ============================================
// Rate Limiting
// ============================================

// RateLimitResult contains rate limit check result
type RateLimitResult struct {
	Allowed   bool
	Remaining int
	ResetAt   time.Time
}

// CheckRateLimit checks if request is within rate limit
// key: unique identifier (e.g., IP, user ID)
// limit: max requests allowed
// window: time window for the limit
func CheckRateLimit(key string, limit int, window time.Duration) (*RateLimitResult, error) {
	fullKey := PrefixRateLimit + key

	// Get current count
	count, err := Client.Get(ctx, fullKey).Int()
	if err == redis.Nil {
		// First request, set count to 1
		err = Client.Set(ctx, fullKey, 1, window).Err()
		if err != nil {
			return nil, err
		}
		return &RateLimitResult{
			Allowed:   true,
			Remaining: limit - 1,
			ResetAt:   time.Now().Add(window),
		}, nil
	} else if err != nil {
		return nil, err
	}

	// Check if within limit
	if count >= limit {
		ttl, _ := Client.TTL(ctx, fullKey).Result()
		return &RateLimitResult{
			Allowed:   false,
			Remaining: 0,
			ResetAt:   time.Now().Add(ttl),
		}, nil
	}

	// Increment count
	newCount, err := Client.Incr(ctx, fullKey).Result()
	if err != nil {
		return nil, err
	}

	ttl, _ := Client.TTL(ctx, fullKey).Result()
	return &RateLimitResult{
		Allowed:   true,
		Remaining: limit - int(newCount),
		ResetAt:   time.Now().Add(ttl),
	}, nil
}

// ResetRateLimit resets the rate limit for a key
func ResetRateLimit(key string) error {
	return Delete(PrefixRateLimit + key)
}
