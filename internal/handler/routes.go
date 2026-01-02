package handler

import (
	"shopedia-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRoutes(app *fiber.App, db *pgxpool.Pool) {
	api := app.Group("/api")

	// Auth routes
	SetupAppAuthRoutes(api, db)
	SetupAdminAuthRoutes(api, db)

	// User routes
	SetupAppUserRoutes(api, db)
	SetupAdminUserRoutes(api, db)
}

// ============================================
// App Auth Routes
// ============================================

func SetupAppAuthRoutes(api fiber.Router, db *pgxpool.Pool) {
	// Public endpoints
	api.Post("/app/register", RegisterHandler(db))
	api.Post("/app/verify-otp", VerifyOTPHandler(db))
	api.Post("/app/request-new-otp", RequestNewOTPHandler(db))
	api.Post("/app/login", LoginHandler(db, "app"))
	api.Post("/app/forgot-password", ForgotPasswordHandler(db))
	api.Post("/app/reset-password", ResetPasswordHandler(db))

	// Protected auth endpoints
	appAuth := api.Group("/app")
	appAuth.Use(middleware.JWTProtected(db))
	appAuth.Post("/logout", LogoutHandler(db))
	appAuth.Post("/logout-all", LogoutAllHandler(db))
	appAuth.Post("/change-password", ChangePasswordHandler(db))
}

// ============================================
// App User Routes
// ============================================

func SetupAppUserRoutes(api fiber.Router, db *pgxpool.Pool) {
	appUser := api.Group("/app")
	appUser.Use(middleware.JWTProtected(db))

	// Profile endpoints
	appUser.Get("/me", GetProfileHandler(db))
	appUser.Put("/me", UpdateProfileHandler(db))
}

// ============================================
// Admin Auth Routes
// ============================================

func SetupAdminAuthRoutes(api fiber.Router, db *pgxpool.Pool) {
	admin := api.Group("/admin")

	// Public admin endpoints (tanpa JWT)
	admin.Post("/register", AdminRegisterHandler(db))     // First time super_admin
	admin.Post("/accept-invite", AcceptInviteHandler(db)) // Admin accept invite
	admin.Post("/login", LoginHandler(db, "admin"))
	admin.Post("/forgot-password", ForgotPasswordHandler(db))
	admin.Post("/reset-password", ResetPasswordHandler(db))

	// Protected auth endpoints (semua admin/super_admin)
	adminAuth := admin.Group("")
	adminAuth.Use(middleware.JWTProtected(db))
	adminAuth.Use(middleware.RoleRequired(db, []string{"admin", "super_admin"}))
	adminAuth.Post("/logout", LogoutHandler(db))
	adminAuth.Post("/logout-all", LogoutAllHandler(db))
	adminAuth.Post("/change-password", ChangePasswordHandler(db))

	// Super admin only auth endpoints
	superAdminAuth := admin.Group("")
	superAdminAuth.Use(middleware.JWTProtected(db))
	superAdminAuth.Use(middleware.RoleRequired(db, []string{"super_admin"}))
	superAdminAuth.Post("/invite-user", InviteUserHandler(db))
}

// ============================================
// Admin User Routes
// ============================================

func SetupAdminUserRoutes(api fiber.Router, db *pgxpool.Pool) {
	admin := api.Group("/admin")
	admin.Use(middleware.JWTProtected(db))
	admin.Use(middleware.RoleRequired(db, []string{"admin", "super_admin"}))

	// Profile (self)
	admin.Get("/me", GetProfileHandler(db))
	admin.Put("/me", UpdateProfileHandler(db))

	// Internal user management (exclude end_user)
	admin.Get("/users", ListUsersHandler(db))
	admin.Get("/users/:uuid", GetUserHandler(db))
	admin.Put("/users/:uuid", UpdateUserHandler(db))
	admin.Delete("/users/:uuid", DeleteUserHandler(db))
	admin.Post("/users/:uuid/activate", ActivateUserHandler(db))
	admin.Post("/users/:uuid/deactivate", DeactivateUserHandler(db))

	// End user management (only end_user)
	admin.Get("/end-users", ListEndUsersHandler(db))
	admin.Get("/end-users/:uuid", GetEndUserHandler(db))
	admin.Post("/end-users/:uuid/ban", BanEndUserHandler(db))
	admin.Post("/end-users/:uuid/unban", UnbanEndUserHandler(db))
}
