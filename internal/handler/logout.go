package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	utils "shopedia-api/internal/util"
)

// LogoutHandler - revoke current access token dan clear active session
func LogoutHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID").(int)
		jti := c.Locals("jti").(string)
		tokenExp := c.Locals("tokenExp").(time.Time)

		// Revoke token
		err := utils.RevokeToken(c.Context(), db, jti, userID, tokenExp)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Clear active session
		err = utils.ClearActiveSession(c.Context(), db, userID, jti)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "Logged out successfully",
		})
	}
}

// LogoutAllHandler - revoke semua token user (force logout dari semua device)
// Dengan Single Active Session, ini sama saja dengan LogoutHandler
func LogoutAllHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID").(int)
		jti := c.Locals("jti").(string)
		tokenExp := c.Locals("tokenExp").(time.Time)

		// Revoke current token
		err := utils.RevokeToken(c.Context(), db, jti, userID, tokenExp)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Clear active session
		err = utils.ClearActiveSession(c.Context(), db, userID, jti)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "Logged out from all devices",
		})
	}
}
