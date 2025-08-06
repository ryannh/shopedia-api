package handler

import (
	"fmt"
	"shopedia-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRoutes(app *fiber.App, db *pgxpool.Pool) {
	fmt.Printf("hai")
	api := app.Group("/api")

	SetupAppAuthRoutes(api, db)
	SetupAdminAuthRoutes(api, db)
}

func SetupAppAuthRoutes(api fiber.Router, db *pgxpool.Pool) {
	api.Post("/app/register", RegisterHandler(db))
	api.Post("/app/verify-otp", VerifyOTPHandler(db))
	api.Post("/app/request-new-otp", RequestNewOTPHandler(db))
	api.Post("/app/login", LoginHandler(db, "app"))
}

func SetupAdminAuthRoutes(api fiber.Router, db *pgxpool.Pool) {
	// Group /admin
	admin := api.Group("/admin")

	// 🔓 Public admin endpoints (tanpa JWT)
	admin.Post("/register", AdminRegisterHandler(db))     // First time super_admin
	admin.Post("/accept-invite", AcceptInviteHandler(db)) // Admin accept invite
	admin.Post("/login", LoginHandler(db, "admin"))       // Login endpoint

	// 🔐 Protected endpoints
	protected := admin.Group("")
	protected.Use(middleware.JWTProtected())
	protected.Use(middleware.RoleRequired(db, []string{"super_admin"}))

	protected.Post("/invite-user", InviteUserHandler(db))
}
