package middleware

import (
	"fmt"
	"strconv"
	"time"

	"shopedia-api/internal/cache"

	"github.com/gofiber/fiber/v2"
)

// RateLimitConfig holds rate limit configuration
type RateLimitConfig struct {
	// Max requests per window
	Max int
	// Time window
	Window time.Duration
	// Key generator function (default: IP address)
	KeyGenerator func(c *fiber.Ctx) string
	// Skip function to bypass rate limiting
	Skip func(c *fiber.Ctx) bool
}

// DefaultRateLimitConfig returns default config
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Max:    100,
		Window: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		Skip: nil,
	}
}

// RateLimit creates a rate limiting middleware
func RateLimit(config ...RateLimitConfig) fiber.Handler {
	cfg := DefaultRateLimitConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *fiber.Ctx) error {
		// Skip if configured
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		// Generate key
		key := cfg.KeyGenerator(c)

		// Check rate limit
		result, err := cache.CheckRateLimit(key, cfg.Max, cfg.Window)
		if err != nil {
			// If Redis error, allow request but log
			return c.Next()
		}

		// Set rate limit headers
		c.Set("X-RateLimit-Limit", strconv.Itoa(cfg.Max))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed {
			c.Set("Retry-After", strconv.FormatInt(int64(time.Until(result.ResetAt).Seconds()), 10))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":      "Rate limit exceeded",
				"retry_after": int(time.Until(result.ResetAt).Seconds()),
			})
		}

		return c.Next()
	}
}

// RateLimitByIP creates IP-based rate limiting
func RateLimitByIP(max int, window time.Duration) fiber.Handler {
	return RateLimit(RateLimitConfig{
		Max:    max,
		Window: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			return "ip:" + c.IP()
		},
	})
}

// RateLimitByUser creates user-based rate limiting (requires JWT auth)
func RateLimitByUser(max int, window time.Duration) fiber.Handler {
	return RateLimit(RateLimitConfig{
		Max:    max,
		Window: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			userID := c.Locals("user_id")
			if userID != nil {
				return fmt.Sprintf("user:%d", userID.(int))
			}
			return "ip:" + c.IP()
		},
	})
}

// RateLimitByEndpoint creates endpoint-specific rate limiting
func RateLimitByEndpoint(max int, window time.Duration) fiber.Handler {
	return RateLimit(RateLimitConfig{
		Max:    max,
		Window: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			return fmt.Sprintf("endpoint:%s:%s:%s", c.Method(), c.Path(), c.IP())
		},
	})
}

// StrictRateLimit creates strict rate limiting for sensitive endpoints
// (e.g., login, register, forgot password)
func StrictRateLimit() fiber.Handler {
	return RateLimit(RateLimitConfig{
		Max:    5,
		Window: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return fmt.Sprintf("strict:%s:%s", c.Path(), c.IP())
		},
	})
}

// OTPRateLimit creates rate limiting for OTP requests
func OTPRateLimit() fiber.Handler {
	return RateLimit(RateLimitConfig{
		Max:    3,
		Window: 5 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return fmt.Sprintf("otp:%s", c.IP())
		},
	})
}
