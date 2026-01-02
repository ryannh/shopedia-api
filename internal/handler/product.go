package handler

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================
// Product Response Types
// ============================================

type ProductCategory struct {
	UUID *string `json:"uuid,omitempty"`
	Name *string `json:"name,omitempty"`
	Icon *string `json:"icon,omitempty"`
}

type ProductOwner struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type ProductResponse struct {
	UUID        string           `json:"uuid"`
	Title       string           `json:"title"`
	Slug        string           `json:"slug"`
	Description *string          `json:"description"`
	Images      []string         `json:"images"`
	Price       int64            `json:"price"`
	Stock       int              `json:"stock"`
	Category    *ProductCategory `json:"category,omitempty"`
	Owner       ProductOwner     `json:"owner"`
	Status      string           `json:"status"`
	IsActive    bool             `json:"is_active"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type ProductDetailResponse struct {
	ProductResponse
	BlockReason *string    `json:"block_reason,omitempty"`
	BlockedAt   *time.Time `json:"blocked_at,omitempty"`
	BlockedBy   *string    `json:"blocked_by,omitempty"`
}

// ============================================
// Public Product Handlers (for End Users)
// ============================================

// ListPublicProductsHandler - GET /products - list active products (public)
func ListPublicProductsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		offset := (page - 1) * limit

		search := c.Query("search", "")
		categoryUUID := c.Query("category", "")

		baseQuery := `FROM products p
			LEFT JOIN product_categories pc ON p.category_id = pc.id
			LEFT JOIN users u ON p.owner_user_id = u.id
			WHERE p.deleted_at IS NULL AND p.status = 'active' AND p.is_active = TRUE`
		args := []interface{}{}
		argCount := 0

		if search != "" {
			argCount++
			baseQuery += ` AND (p.title ILIKE $` + strconv.Itoa(argCount) + ` OR p.description ILIKE $` + strconv.Itoa(argCount) + `)`
			args = append(args, "%"+search+"%")
		}

		if categoryUUID != "" {
			argCount++
			baseQuery += ` AND pc.uuid = $` + strconv.Itoa(argCount)
			args = append(args, categoryUUID)
		}

		var totalItems int
		err := db.QueryRow(ctx, `SELECT COUNT(*) `+baseQuery, args...).Scan(&totalItems)
		if err != nil {
			log.Printf("ListPublicProducts count error: %v", err)
			return fiber.ErrInternalServerError
		}

		argCount++
		limitArg := argCount
		argCount++
		offsetArg := argCount
		args = append(args, limit, offset)

		dataQuery := `SELECT p.uuid, p.title, p.slug, p.description,
			COALESCE(to_json(p.images), '[]'::json)::text, p.price, p.stock,
			pc.uuid, pc.name, pc.icon, u.uuid, COALESCE(u.full_name, ''),
			p.status, p.is_active, p.created_at, p.updated_at ` +
			baseQuery + ` ORDER BY p.created_at DESC LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg)

		rows, err := db.Query(ctx, dataQuery, args...)
		if err != nil {
			log.Printf("ListPublicProducts query error: %v", err)
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		products := []ProductResponse{}
		for rows.Next() {
			var p ProductResponse
			var imagesJSON string
			var catUUID, catName, catIcon *string
			var ownerUUID, ownerName string

			err := rows.Scan(&p.UUID, &p.Title, &p.Slug, &p.Description, &imagesJSON, &p.Price, &p.Stock,
				&catUUID, &catName, &catIcon, &ownerUUID, &ownerName,
				&p.Status, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
			if err != nil {
				log.Printf("ListPublicProducts scan error: %v", err)
				continue
			}

			if err := json.Unmarshal([]byte(imagesJSON), &p.Images); err != nil {
				p.Images = []string{}
			}

			if catUUID != nil {
				p.Category = &ProductCategory{UUID: catUUID, Name: catName, Icon: catIcon}
			}
			p.Owner = ProductOwner{UUID: ownerUUID, Name: ownerName}

			products = append(products, p)
		}

		totalPages := (totalItems + limit - 1) / limit

		return c.JSON(PaginatedResponse{
			Data:       products,
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		})
	}
}

// GetPublicProductHandler - GET /products/:uuid - get product detail (public)
func GetPublicProductHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		productUUID := c.Params("uuid")
		ctx := context.Background()

		var p ProductResponse
		var imagesJSON string
		var catUUID, catName, catIcon *string
		var ownerUUID, ownerName string

		err := db.QueryRow(ctx, `
			SELECT p.uuid, p.title, p.slug, p.description,
				COALESCE(to_json(p.images), '[]'::json)::text, p.price, p.stock,
				pc.uuid, pc.name, pc.icon, u.uuid, COALESCE(u.full_name, ''),
				p.status, p.is_active, p.created_at, p.updated_at
			FROM products p
			LEFT JOIN product_categories pc ON p.category_id = pc.id
			LEFT JOIN users u ON p.owner_user_id = u.id
			WHERE p.uuid = $1 AND p.deleted_at IS NULL AND p.status = 'active' AND p.is_active = TRUE`,
			productUUID).Scan(&p.UUID, &p.Title, &p.Slug, &p.Description, &imagesJSON, &p.Price, &p.Stock,
			&catUUID, &catName, &catIcon, &ownerUUID, &ownerName,
			&p.Status, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return fiber.ErrNotFound
		}

		if err := json.Unmarshal([]byte(imagesJSON), &p.Images); err != nil {
			p.Images = []string{}
		}

		if catUUID != nil {
			p.Category = &ProductCategory{UUID: catUUID, Name: catName, Icon: catIcon}
		}
		p.Owner = ProductOwner{UUID: ownerUUID, Name: ownerName}

		return c.JSON(p)
	}
}

// ============================================
// Seller Product Handlers (for Apps)
// ============================================

// ListSellerProductsHandler - GET /my/products - list seller's own products
func ListSellerProductsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int)
		ctx := context.Background()

		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		offset := (page - 1) * limit

		status := c.Query("status", "")

		baseQuery := `FROM products p
			LEFT JOIN product_categories pc ON p.category_id = pc.id
			LEFT JOIN users u ON p.owner_user_id = u.id
			WHERE p.deleted_at IS NULL AND p.owner_user_id = $1`
		args := []interface{}{userID}
		argCount := 1

		if status != "" {
			argCount++
			baseQuery += ` AND p.status = $` + strconv.Itoa(argCount)
			args = append(args, status)
		}

		var totalItems int
		err := db.QueryRow(ctx, `SELECT COUNT(*) `+baseQuery, args...).Scan(&totalItems)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		argCount++
		limitArg := argCount
		argCount++
		offsetArg := argCount
		args = append(args, limit, offset)

		dataQuery := `SELECT p.uuid, p.title, p.slug, p.description,
			COALESCE(to_json(p.images), '[]'::json)::text, p.price, p.stock,
			pc.uuid, pc.name, pc.icon, u.uuid, COALESCE(u.full_name, ''),
			p.status, p.is_active, p.created_at, p.updated_at ` +
			baseQuery + ` ORDER BY p.created_at DESC LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg)

		rows, err := db.Query(ctx, dataQuery, args...)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		products := []ProductResponse{}
		for rows.Next() {
			var p ProductResponse
			var imagesJSON string
			var catUUID, catName, catIcon *string
			var ownerUUID, ownerName string

			err := rows.Scan(&p.UUID, &p.Title, &p.Slug, &p.Description, &imagesJSON, &p.Price, &p.Stock,
				&catUUID, &catName, &catIcon, &ownerUUID, &ownerName,
				&p.Status, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
			if err != nil {
				continue
			}

			if err := json.Unmarshal([]byte(imagesJSON), &p.Images); err != nil {
				p.Images = []string{}
			}

			if catUUID != nil {
				p.Category = &ProductCategory{UUID: catUUID, Name: catName, Icon: catIcon}
			}
			p.Owner = ProductOwner{UUID: ownerUUID, Name: ownerName}

			products = append(products, p)
		}

		totalPages := (totalItems + limit - 1) / limit

		return c.JSON(PaginatedResponse{
			Data:       products,
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		})
	}
}

// GetSellerProductHandler - GET /my/products/:uuid - get seller's product detail
func GetSellerProductHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int)
		productUUID := c.Params("uuid")
		ctx := context.Background()

		var p ProductDetailResponse
		var imagesJSON string
		var catUUID, catName, catIcon *string
		var ownerUUID, ownerName string

		err := db.QueryRow(ctx, `
			SELECT p.uuid, p.title, p.slug, p.description,
				COALESCE(to_json(p.images), '[]'::json)::text, p.price, p.stock,
				pc.uuid, pc.name, pc.icon, u.uuid, COALESCE(u.full_name, ''),
				p.status, p.is_active, p.created_at, p.updated_at,
				p.block_reason, p.blocked_at, COALESCE(bu.full_name, '')
			FROM products p
			LEFT JOIN product_categories pc ON p.category_id = pc.id
			LEFT JOIN users u ON p.owner_user_id = u.id
			LEFT JOIN users bu ON p.blocked_by = bu.id
			WHERE p.uuid = $1 AND p.owner_user_id = $2 AND p.deleted_at IS NULL`,
			productUUID, userID).Scan(&p.UUID, &p.Title, &p.Slug, &p.Description, &imagesJSON, &p.Price, &p.Stock,
			&catUUID, &catName, &catIcon, &ownerUUID, &ownerName,
			&p.Status, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
			&p.BlockReason, &p.BlockedAt, &p.BlockedBy)
		if err != nil {
			return fiber.ErrNotFound
		}

		if err := json.Unmarshal([]byte(imagesJSON), &p.Images); err != nil {
			p.Images = []string{}
		}

		if catUUID != nil {
			p.Category = &ProductCategory{UUID: catUUID, Name: catName, Icon: catIcon}
		}
		p.Owner = ProductOwner{UUID: ownerUUID, Name: ownerName}

		return c.JSON(p)
	}
}

// CreateProductHandler - POST /my/products - create new product
func CreateProductHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int)

		type Input struct {
			Title        string   `json:"title"`
			Description  *string  `json:"description"`
			Images       []string `json:"images"`
			Price        int64    `json:"price"`
			Stock        int      `json:"stock"`
			CategoryUUID *string  `json:"category_uuid"`
			Slug         *string  `json:"slug"`
			IsActive     *bool    `json:"is_active"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		if input.Title == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Title is required")
		}
		if input.Price < 0 {
			return fiber.NewError(fiber.StatusBadRequest, "Price must be positive")
		}
		if input.Stock < 0 {
			return fiber.NewError(fiber.StatusBadRequest, "Stock must be positive")
		}

		ctx := context.Background()

		slug := ""
		if input.Slug != nil && *input.Slug != "" {
			slug = generateSlug(*input.Slug)
		} else {
			slug = generateSlug(input.Title)
		}

		var categoryID *int
		if input.CategoryUUID != nil && *input.CategoryUUID != "" {
			var catID int
			err := db.QueryRow(ctx, `SELECT id FROM product_categories WHERE uuid = $1 AND deleted_at IS NULL AND is_active = TRUE`,
				*input.CategoryUUID).Scan(&catID)
			if err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid category")
			}
			categoryID = &catID
		}

		isActive := true
		if input.IsActive != nil {
			isActive = *input.IsActive
		}

		if input.Images == nil {
			input.Images = []string{}
		}

		var productUUID string
		err := db.QueryRow(ctx, `
			INSERT INTO products (owner_user_id, category_id, title, slug, description, images, price, stock, is_active, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'active')
			RETURNING uuid`,
			userID, categoryID, input.Title, slug, input.Description, input.Images, input.Price, input.Stock, isActive).Scan(&productUUID)
		if err != nil {
			log.Printf("CreateProduct error: %v", err)
			return fiber.ErrInternalServerError
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "Product created successfully",
			"uuid":    productUUID,
			"slug":    slug,
		})
	}
}

// UpdateProductHandler - PUT /my/products/:uuid - update product
func UpdateProductHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int)
		productUUID := c.Params("uuid")

		type Input struct {
			Title        *string  `json:"title"`
			Description  *string  `json:"description"`
			Images       []string `json:"images"`
			Price        *int64   `json:"price"`
			Stock        *int     `json:"stock"`
			CategoryUUID *string  `json:"category_uuid"`
			Slug         *string  `json:"slug"`
			IsActive     *bool    `json:"is_active"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		ctx := context.Background()

		var productID int
		var currentStatus string
		err := db.QueryRow(ctx, `SELECT id, status FROM products WHERE uuid = $1 AND owner_user_id = $2 AND deleted_at IS NULL`,
			productUUID, userID).Scan(&productID, &currentStatus)
		if err != nil {
			return fiber.ErrNotFound
		}

		if currentStatus == "blocked" {
			return fiber.NewError(fiber.StatusForbidden, "Cannot edit blocked product")
		}

		if input.Title != nil {
			_, err = db.Exec(ctx, `UPDATE products SET title = $1, updated_at = NOW() WHERE id = $2`, *input.Title, productID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Slug != nil {
			slug := generateSlug(*input.Slug)
			_, err = db.Exec(ctx, `UPDATE products SET slug = $1, updated_at = NOW() WHERE id = $2`, slug, productID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Description != nil {
			_, err = db.Exec(ctx, `UPDATE products SET description = $1, updated_at = NOW() WHERE id = $2`, *input.Description, productID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Images != nil {
			_, err = db.Exec(ctx, `UPDATE products SET images = $1, updated_at = NOW() WHERE id = $2`, input.Images, productID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Price != nil {
			if *input.Price < 0 {
				return fiber.NewError(fiber.StatusBadRequest, "Price must be positive")
			}
			_, err = db.Exec(ctx, `UPDATE products SET price = $1, updated_at = NOW() WHERE id = $2`, *input.Price, productID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.Stock != nil {
			if *input.Stock < 0 {
				return fiber.NewError(fiber.StatusBadRequest, "Stock must be positive")
			}
			_, err = db.Exec(ctx, `UPDATE products SET stock = $1, updated_at = NOW() WHERE id = $2`, *input.Stock, productID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.CategoryUUID != nil {
			var categoryID *int
			if *input.CategoryUUID != "" {
				var catID int
				err := db.QueryRow(ctx, `SELECT id FROM product_categories WHERE uuid = $1 AND deleted_at IS NULL AND is_active = TRUE`,
					*input.CategoryUUID).Scan(&catID)
				if err != nil {
					return fiber.NewError(fiber.StatusBadRequest, "Invalid category")
				}
				categoryID = &catID
			}
			_, err = db.Exec(ctx, `UPDATE products SET category_id = $1, updated_at = NOW() WHERE id = $2`, categoryID, productID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		if input.IsActive != nil {
			_, err = db.Exec(ctx, `UPDATE products SET is_active = $1, updated_at = NOW() WHERE id = $2`, *input.IsActive, productID)
			if err != nil {
				return fiber.ErrInternalServerError
			}
		}

		return c.JSON(fiber.Map{"message": "Product updated successfully"})
	}
}

// DeleteProductHandler - DELETE /my/products/:uuid - soft delete product
func DeleteProductHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int)
		productUUID := c.Params("uuid")
		ctx := context.Background()

		var productID int
		err := db.QueryRow(ctx, `SELECT id FROM products WHERE uuid = $1 AND owner_user_id = $2 AND deleted_at IS NULL`,
			productUUID, userID).Scan(&productID)
		if err != nil {
			return fiber.ErrNotFound
		}

		_, err = db.Exec(ctx, `UPDATE products SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, productID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{"message": "Product deleted successfully"})
	}
}

// ============================================
// Admin Product Handlers (Dashboard)
// ============================================

// ListAdminProductsHandler - GET /products - list all products with filters
func ListAdminProductsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		offset := (page - 1) * limit

		search := c.Query("search", "")
		status := c.Query("status", "")
		categoryUUID := c.Query("category", "")
		ownerUUID := c.Query("owner", "")

		baseQuery := `FROM products p
			LEFT JOIN product_categories pc ON p.category_id = pc.id
			LEFT JOIN users u ON p.owner_user_id = u.id
			WHERE p.deleted_at IS NULL`
		args := []interface{}{}
		argCount := 0

		if search != "" {
			argCount++
			baseQuery += ` AND (p.title ILIKE $` + strconv.Itoa(argCount) + ` OR p.description ILIKE $` + strconv.Itoa(argCount) + `)`
			args = append(args, "%"+search+"%")
		}

		if status != "" {
			argCount++
			baseQuery += ` AND p.status = $` + strconv.Itoa(argCount)
			args = append(args, status)
		}

		if categoryUUID != "" {
			argCount++
			baseQuery += ` AND pc.uuid = $` + strconv.Itoa(argCount)
			args = append(args, categoryUUID)
		}

		if ownerUUID != "" {
			argCount++
			baseQuery += ` AND u.uuid = $` + strconv.Itoa(argCount)
			args = append(args, ownerUUID)
		}

		var totalItems int
		err := db.QueryRow(ctx, `SELECT COUNT(*) `+baseQuery, args...).Scan(&totalItems)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		argCount++
		limitArg := argCount
		argCount++
		offsetArg := argCount
		args = append(args, limit, offset)

		dataQuery := `SELECT p.uuid, p.title, p.slug, p.description,
			COALESCE(to_json(p.images), '[]'::json)::text, p.price, p.stock,
			pc.uuid, pc.name, pc.icon, u.uuid, COALESCE(u.full_name, ''),
			p.status, p.is_active, p.created_at, p.updated_at ` +
			baseQuery + ` ORDER BY p.created_at DESC LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg)

		rows, err := db.Query(ctx, dataQuery, args...)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer rows.Close()

		products := []ProductResponse{}
		for rows.Next() {
			var p ProductResponse
			var imagesJSON string
			var catUUID, catName, catIcon *string
			var ownerUUID, ownerName string

			err := rows.Scan(&p.UUID, &p.Title, &p.Slug, &p.Description, &imagesJSON, &p.Price, &p.Stock,
				&catUUID, &catName, &catIcon, &ownerUUID, &ownerName,
				&p.Status, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
			if err != nil {
				continue
			}

			if err := json.Unmarshal([]byte(imagesJSON), &p.Images); err != nil {
				p.Images = []string{}
			}

			if catUUID != nil {
				p.Category = &ProductCategory{UUID: catUUID, Name: catName, Icon: catIcon}
			}
			p.Owner = ProductOwner{UUID: ownerUUID, Name: ownerName}

			products = append(products, p)
		}

		totalPages := (totalItems + limit - 1) / limit

		return c.JSON(PaginatedResponse{
			Data:       products,
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		})
	}
}

// GetAdminProductHandler - GET /products/:uuid - get product detail (admin)
func GetAdminProductHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		productUUID := c.Params("uuid")
		ctx := context.Background()

		var p ProductDetailResponse
		var imagesJSON string
		var catUUID, catName, catIcon *string
		var ownerUUID, ownerName string

		err := db.QueryRow(ctx, `
			SELECT p.uuid, p.title, p.slug, p.description,
				COALESCE(to_json(p.images), '[]'::json)::text, p.price, p.stock,
				pc.uuid, pc.name, pc.icon, u.uuid, COALESCE(u.full_name, ''),
				p.status, p.is_active, p.created_at, p.updated_at,
				p.block_reason, p.blocked_at, COALESCE(bu.full_name, '')
			FROM products p
			LEFT JOIN product_categories pc ON p.category_id = pc.id
			LEFT JOIN users u ON p.owner_user_id = u.id
			LEFT JOIN users bu ON p.blocked_by = bu.id
			WHERE p.uuid = $1 AND p.deleted_at IS NULL`,
			productUUID).Scan(&p.UUID, &p.Title, &p.Slug, &p.Description, &imagesJSON, &p.Price, &p.Stock,
			&catUUID, &catName, &catIcon, &ownerUUID, &ownerName,
			&p.Status, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
			&p.BlockReason, &p.BlockedAt, &p.BlockedBy)
		if err != nil {
			return fiber.ErrNotFound
		}

		if err := json.Unmarshal([]byte(imagesJSON), &p.Images); err != nil {
			p.Images = []string{}
		}

		if catUUID != nil {
			p.Category = &ProductCategory{UUID: catUUID, Name: catName, Icon: catIcon}
		}
		p.Owner = ProductOwner{UUID: ownerUUID, Name: ownerName}

		return c.JSON(p)
	}
}

// BlockProductHandler - POST /products/:uuid/block - block a product
func BlockProductHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int)
		productUUID := c.Params("uuid")

		type Input struct {
			Reason string `json:"reason"`
		}
		var input Input
		if err := c.BodyParser(&input); err != nil {
			return fiber.ErrBadRequest
		}

		if input.Reason == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Block reason is required")
		}

		ctx := context.Background()

		var productID int
		var currentStatus string
		err := db.QueryRow(ctx, `SELECT id, status FROM products WHERE uuid = $1 AND deleted_at IS NULL`,
			productUUID).Scan(&productID, &currentStatus)
		if err != nil {
			return fiber.ErrNotFound
		}

		if currentStatus == "blocked" {
			return fiber.NewError(fiber.StatusConflict, "Product is already blocked")
		}

		_, err = db.Exec(ctx, `
			UPDATE products SET status = 'blocked', block_reason = $1, blocked_at = NOW(), blocked_by = $2, updated_at = NOW()
			WHERE id = $3`, input.Reason, userID, productID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{"message": "Product blocked successfully"})
	}
}

// UnblockProductHandler - POST /products/:uuid/unblock - unblock a product
func UnblockProductHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		productUUID := c.Params("uuid")
		ctx := context.Background()

		var productID int
		var currentStatus string
		err := db.QueryRow(ctx, `SELECT id, status FROM products WHERE uuid = $1 AND deleted_at IS NULL`,
			productUUID).Scan(&productID, &currentStatus)
		if err != nil {
			return fiber.ErrNotFound
		}

		if currentStatus != "blocked" {
			return fiber.NewError(fiber.StatusConflict, "Product is not blocked")
		}

		_, err = db.Exec(ctx, `
			UPDATE products SET status = 'active', block_reason = NULL, blocked_at = NULL, blocked_by = NULL, updated_at = NOW()
			WHERE id = $1`, productID)
		if err != nil {
			return fiber.ErrInternalServerError
		}

		return c.JSON(fiber.Map{"message": "Product unblocked successfully"})
	}
}
