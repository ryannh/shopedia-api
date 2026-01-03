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

	// Role & Permission routes
	SetupAdminRoleRoutes(api, db)

	// Product & Category routes
	SetupPublicRoutes(api, db)
	SetupSellerRoutes(api, db)
	SetupAdminProductRoutes(api, db)
}

// ============================================
// App Auth Routes
// ============================================

func SetupAppAuthRoutes(api fiber.Router, db *pgxpool.Pool) {
	// Public endpoints with rate limiting
	api.Post("/app/register", middleware.StrictRateLimit(), RegisterHandler(db))
	api.Post("/app/verify-otp", middleware.StrictRateLimit(), VerifyOTPHandler(db))
	api.Post("/app/request-new-otp", middleware.OTPRateLimit(), RequestNewOTPHandler(db))
	api.Post("/app/login", middleware.StrictRateLimit(), LoginHandler(db, "app"))
	api.Post("/app/forgot-password", middleware.StrictRateLimit(), ForgotPasswordHandler(db))
	api.Post("/app/reset-password", middleware.StrictRateLimit(), ResetPasswordHandler(db))

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
	admin.Use(middleware.ScopeRequired(db, "dashboard"))

	// Profile (self) - all dashboard users
	admin.Get("/me", GetProfileHandler(db))
	admin.Put("/me", UpdateProfileHandler(db))

	// User management - requires user.* permissions
	userMgmt := admin.Group("")
	userMgmt.Use(middleware.PermissionRequired(db, []string{"user.view"}))

	// Internal user management (exclude end_user)
	userMgmt.Get("/users", ListUsersHandler(db))
	userMgmt.Get("/users/:uuid", GetUserHandler(db))
	userMgmt.Get("/users/:uuid/roles", GetUserRolesHandler(db))

	// User write operations
	userWrite := admin.Group("")
	userWrite.Use(middleware.PermissionRequired(db, []string{"user.update"}))
	userWrite.Put("/users/:uuid", UpdateUserHandler(db))

	userDelete := admin.Group("")
	userDelete.Use(middleware.PermissionRequired(db, []string{"user.delete"}))
	userDelete.Delete("/users/:uuid", DeleteUserHandler(db))

	userActivate := admin.Group("")
	userActivate.Use(middleware.PermissionRequired(db, []string{"user.activate"}))
	userActivate.Post("/users/:uuid/activate", ActivateUserHandler(db))
	userActivate.Post("/users/:uuid/deactivate", DeactivateUserHandler(db))

	// End user management (only end_user)
	endUserMgmt := admin.Group("")
	endUserMgmt.Use(middleware.PermissionRequired(db, []string{"user.view"}))
	endUserMgmt.Get("/end-users", ListEndUsersHandler(db))
	endUserMgmt.Get("/end-users/:uuid", GetEndUserHandler(db))

	endUserBan := admin.Group("")
	endUserBan.Use(middleware.PermissionRequired(db, []string{"user.ban"}))
	endUserBan.Post("/end-users/:uuid/ban", BanEndUserHandler(db))
	endUserBan.Post("/end-users/:uuid/unban", UnbanEndUserHandler(db))

	// Role assignment to user (super_admin only)
	roleAssign := admin.Group("")
	roleAssign.Use(middleware.PermissionRequired(db, []string{"role.assign"}))
	roleAssign.Post("/users/:uuid/roles", AssignRoleToUserHandler(db))
	roleAssign.Delete("/users/:uuid/roles/:role_uuid", RemoveRoleFromUserHandler(db))
}

// ============================================
// Admin Role & Permission Routes
// ============================================

func SetupAdminRoleRoutes(api fiber.Router, db *pgxpool.Pool) {
	admin := api.Group("/admin")
	admin.Use(middleware.JWTProtected(db))
	admin.Use(middleware.ScopeRequired(db, "dashboard"))

	// Role management - view
	roleView := admin.Group("")
	roleView.Use(middleware.PermissionRequired(db, []string{"role.view"}))
	roleView.Get("/roles", ListRolesHandler(db))
	roleView.Get("/roles/:uuid", GetRoleHandler(db))

	// Role management - write (super_admin only)
	roleWrite := admin.Group("")
	roleWrite.Use(middleware.RoleRequired(db, []string{"super_admin"}))
	roleWrite.Post("/roles", CreateRoleHandler(db))
	roleWrite.Put("/roles/:uuid", UpdateRoleHandler(db))
	roleWrite.Delete("/roles/:uuid", DeleteRoleHandler(db))

	// Role-Permission assignment (super_admin only)
	roleWrite.Post("/roles/:uuid/permissions", AssignPermissionsToRoleHandler(db))
	roleWrite.Delete("/roles/:uuid/permissions/:perm_uuid", RemovePermissionFromRoleHandler(db))

	// Permission management - view
	permView := admin.Group("")
	permView.Use(middleware.PermissionRequired(db, []string{"permission.view"}))
	permView.Get("/permissions", ListPermissionsHandler(db))
	permView.Get("/permissions/modules", ListPermissionModulesHandler(db))
	permView.Get("/permissions/:uuid", GetPermissionHandler(db))

	// Permission management - write (super_admin only)
	permWrite := admin.Group("")
	permWrite.Use(middleware.RoleRequired(db, []string{"super_admin"}))
	permWrite.Post("/permissions", CreatePermissionHandler(db))
	permWrite.Put("/permissions/:uuid", UpdatePermissionHandler(db))
	permWrite.Delete("/permissions/:uuid", DeletePermissionHandler(db))
}

// ============================================
// Public Routes (Categories & Products)
// ============================================

func SetupPublicRoutes(api fiber.Router, db *pgxpool.Pool) {
	// Public categories - no auth required
	api.Get("/categories", ListPublicCategoriesHandler(db))

	// Public products - no auth required
	api.Get("/products", ListPublicProductsHandler(db))
	api.Get("/products/:uuid", GetPublicProductHandler(db))
}

// ============================================
// Seller Routes (App)
// ============================================

func SetupSellerRoutes(api fiber.Router, db *pgxpool.Pool) {
	seller := api.Group("/app/my")
	seller.Use(middleware.JWTProtected(db))
	seller.Use(middleware.RoleRequired(db, []string{"seller", "end_user"}))

	// Seller product management
	seller.Get("/products", ListSellerProductsHandler(db))
	seller.Get("/products/:uuid", GetSellerProductHandler(db))
	seller.Post("/products", CreateProductHandler(db))
	seller.Put("/products/:uuid", UpdateProductHandler(db))
	seller.Delete("/products/:uuid", DeleteProductHandler(db))
}

// ============================================
// Admin Product & Category Routes
// ============================================

func SetupAdminProductRoutes(api fiber.Router, db *pgxpool.Pool) {
	admin := api.Group("/admin")
	admin.Use(middleware.JWTProtected(db))
	admin.Use(middleware.ScopeRequired(db, "dashboard"))

	// Category management - view
	catView := admin.Group("")
	catView.Use(middleware.PermissionRequired(db, []string{"category.view"}))
	catView.Get("/categories", ListCategoriesHandler(db))
	catView.Get("/categories/:uuid", GetCategoryHandler(db))

	// Category management - create
	catCreate := admin.Group("")
	catCreate.Use(middleware.PermissionRequired(db, []string{"category.create"}))
	catCreate.Post("/categories", CreateCategoryHandler(db))

	// Category management - update
	catUpdate := admin.Group("")
	catUpdate.Use(middleware.PermissionRequired(db, []string{"category.update"}))
	catUpdate.Put("/categories/:uuid", UpdateCategoryHandler(db))

	// Category management - delete
	catDelete := admin.Group("")
	catDelete.Use(middleware.PermissionRequired(db, []string{"category.delete"}))
	catDelete.Delete("/categories/:uuid", DeleteCategoryHandler(db))

	// Product management - view
	prodView := admin.Group("")
	prodView.Use(middleware.PermissionRequired(db, []string{"product.view"}))
	prodView.Get("/products", ListAdminProductsHandler(db))
	prodView.Get("/products/:uuid", GetAdminProductHandler(db))

	// Product management - moderate (block/unblock)
	prodModerate := admin.Group("")
	prodModerate.Use(middleware.PermissionRequired(db, []string{"product.moderate"}))
	prodModerate.Post("/products/:uuid/block", BlockProductHandler(db))
	prodModerate.Post("/products/:uuid/unblock", UnblockProductHandler(db))
}
