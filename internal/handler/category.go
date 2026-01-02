package handler

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================
// Category Response Types
// ============================================

type CategoryResponse struct {
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Icon        *string   `json:"icon"`
	Description *string   `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ============================================
// Helper Functions
// ============================================

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	slug = reg.ReplaceAllString(slug, "")
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

// ============================================
// Public Category Handlers (for Apps)
// ============================================

// ListPublicCategoriesHandler - GET /categories - list active categories (public)
func ListPublicCategoriesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		rows, err := db.Query(ctx, `
			SELECT uuid, name, slug, icon, description, is_active, created_at, updated_at
			FROM product_categories
			WHERE deleted_at IS NULL AND is_active = TRUE
			ORDER BY name`)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		categories := []CategoryResponse{}
		for rows.Next() {
			var cat CategoryResponse
			err := rows.Scan(&cat.UUID, &cat.Name, &cat.Slug, &cat.Icon, &cat.Description,
				&cat.IsActive, &cat.CreatedAt, &cat.UpdatedAt)
			if err != nil {
				continue
			}
			categories = append(categories, cat)
		}

		return c.JSON(fiber.Map{
			"categories": categories,
		})
	}
}

// ============================================
// Admin Category Handlers (Dashboard)
// ============================================

// ListCategoriesHandler - GET /categories - list all categories with pagination
func ListCategoriesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		// Pagination params
		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		offset := (page - 1) * limit

		// Filter params
		search := c.Query("search", "")
		activeFilter := c.Query("is_active", "")

		// Build query
		baseQuery := `FROM product_categories WHERE deleted_at IS NULL`
		args := []interface{}{}
		argCount := 0

		if search != "" {
			argCount++
			baseQuery += ` AND (name ILIKE $` + strconv.Itoa(argCount) + ` OR description ILIKE $` + strconv.Itoa(argCount) + `)`
			args = append(args, "%"+search+"%")
		}

		if activeFilter != "" {
			argCount++
			baseQuery += ` AND is_active = $` + strconv.Itoa(argCount)
			isActive := activeFilter == "true"
			args = append(args, isActive)
		}

		// Count total
		var totalItems int
		countQuery := `SELECT COUNT(*) ` + baseQuery
		err := db.QueryRow(ctx, countQuery, args...).Scan(&totalItems)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		// Get data
		argCount++
		limitArg := argCount
		argCount++
		offsetArg := argCount
		dataQuery := `SELECT uuid, name, slug, icon, description, is_active, created_at, updated_at ` +
			baseQuery + ` ORDER BY name LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg)
		args = append(args, limit, offset)

		rows, err := db.Query(ctx, dataQuery, args...)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		categories := []CategoryResponse{}
		for rows.Next() {
			var cat CategoryResponse
			err := rows.Scan(&cat.UUID, &cat.Name, &cat.Slug, &cat.Icon, &cat.Description,
				&cat.IsActive, &cat.CreatedAt, &cat.UpdatedAt)
			if err != nil {
				continue
			}
			categories = append(categories, cat)
		}

		totalPages := (totalItems + limit - 1) / limit

		return c.JSON(PaginatedResponse{
			Data:       categories,
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		})
	}
}

// GetCategoryHandler - GET /categories/:uuid - get category detail
func GetCategoryHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		catUUID := c.Params("uuid")
		ctx := context.Background()

		var cat CategoryResponse
		err := db.QueryRow(ctx, `
			SELECT uuid, name, slug, icon, description, is_active, created_at, updated_at
			FROM product_categories WHERE uuid = $1 AND deleted_at IS NULL`,
			catUUID).Scan(
			&cat.UUID, &cat.Name, &cat.Slug, &cat.Icon, &cat.Description,
			&cat.IsActive, &cat.CreatedAt, &cat.UpdatedAt)
		if err != nil {
			return fiber.ErrNotFound
		}

		return c.JSON(cat)
	}
}

// CreateCategoryHandler - POST /categories - create new category
func CreateCategoryHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type Input struct {
			Name        string  `json:"name"`
			Description *string `json:"description"`
			Icon        *string `json:"icon"`
			Slug        *string `json:"slug"`
			IsActive    *bool   `json:"is_active"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		if input.Name == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Name is required")
		}

		ctx := context.Background()

		// Generate slug if not provided
		slug := ""
		if input.Slug != nil && *input.Slug != "" {
			slug = generateSlug(*input.Slug)
		} else {
			slug = generateSlug(input.Name)
		}

		// Check if slug already exists
		var exists bool
		err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM product_categories WHERE slug = $1 AND deleted_at IS NULL)`,
			slug).Scan(&exists)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		if exists {
			return fiber.NewError(fiber.StatusConflict, "Category slug already exists")
		}

		isActive := true
		if input.IsActive != nil {
			isActive = *input.IsActive
		}

		// Create category
		var catUUID string
		err = db.QueryRow(ctx, `
			INSERT INTO product_categories (name, slug, icon, description, is_active)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING uuid`,
			input.Name, slug, input.Icon, input.Description, isActive).Scan(&catUUID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "Category created successfully",
			"uuid":    catUUID,
			"slug":    slug,
		})
	}
}

// UpdateCategoryHandler - PUT /categories/:uuid - update category
func UpdateCategoryHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		catUUID := c.Params("uuid")

		type Input struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
			Icon        *string `json:"icon"`
			Slug        *string `json:"slug"`
			IsActive    *bool   `json:"is_active"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		// Check if category exists
		var catID int
		err := db.QueryRow(ctx, `SELECT id FROM product_categories WHERE uuid = $1 AND deleted_at IS NULL`,
			catUUID).Scan(&catID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Update fields
		if input.Name != nil {
			_, err = db.Exec(ctx, `UPDATE product_categories SET name = $1, updated_at = NOW() WHERE id = $2`,
				*input.Name, catID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Slug != nil {
			slug := generateSlug(*input.Slug)
			var exists bool
			err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM product_categories WHERE slug = $1 AND id != $2 AND deleted_at IS NULL)`,
				slug, catID).Scan(&exists)
			if exists {
				return fiber.NewError(fiber.StatusConflict, "Category slug already exists")
			}

			_, err = db.Exec(ctx, `UPDATE product_categories SET slug = $1, updated_at = NOW() WHERE id = $2`,
				slug, catID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Description != nil {
			_, err = db.Exec(ctx, `UPDATE product_categories SET description = $1, updated_at = NOW() WHERE id = $2`,
				*input.Description, catID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Icon != nil {
			_, err = db.Exec(ctx, `UPDATE product_categories SET icon = $1, updated_at = NOW() WHERE id = $2`,
				*input.Icon, catID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.IsActive != nil {
			_, err = db.Exec(ctx, `UPDATE product_categories SET is_active = $1, updated_at = NOW() WHERE id = $2`,
				*input.IsActive, catID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		return c.JSON(fiber.Map{
			"message": "Category updated successfully",
		})
	}
}

// DeleteCategoryHandler - DELETE /categories/:uuid - soft delete category
func DeleteCategoryHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		catUUID := c.Params("uuid")
		ctx := context.Background()

		// Check if category exists
		var catID int
		err := db.QueryRow(ctx, `SELECT id FROM product_categories WHERE uuid = $1 AND deleted_at IS NULL`,
			catUUID).Scan(&catID)
		if err != nil {
			return fiber.ErrNotFound
		}

		// Check if category has active products
		var productCount int
		err = db.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE category_id = $1 AND deleted_at IS NULL`,
			catID).Scan(&productCount)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		if productCount > 0 {
			return fiber.NewError(fiber.StatusConflict, "Cannot delete category with existing products")
		}

		// Soft delete
		_, err = db.Exec(ctx, `UPDATE product_categories SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`,
			catID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{
			"message": "Category deleted successfully",
		})
	}
}
