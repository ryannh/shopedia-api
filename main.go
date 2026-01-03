package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"shopedia-api/internal/cache"
	"shopedia-api/internal/handler"
	"shopedia-api/internal/queue"
	"shopedia-api/internal/repository"
	utils "shopedia-api/internal/util"
)

func main() {
	// Load env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Connect DB
	db, err := repository.ConnectDB(os.Getenv("DB_URL"))
	if err != nil {
		log.Fatal("DB connection failed:", err)
	}
	fmt.Println("DB connected")

	// Run migrations
	err = repository.RunMigrations(db, "migration")
	if err != nil {
		log.Fatal("Migration failed:", err)
	}

	// Initialize queue client
	queue.InitClient()
	defer queue.CloseClient()
	fmt.Println("Queue client initialized")

	// Initialize Redis cache
	err = cache.InitRedis()
	if err != nil {
		log.Printf("Redis connection failed (caching disabled): %v", err)
	} else {
		defer cache.CloseRedis()
		fmt.Println("Redis cache initialized")
	}

	// Start cleanup goroutine
	go startTokenCleanup(db)

	// Fiber instance
	app := fiber.New()

	// Routes
	handler.SetupRoutes(app, db)

	// Start
	port := ":3000"
	fmt.Println("Server running on", port)
	log.Fatal(app.Listen(port))
}

// startTokenCleanup - goroutine untuk cleanup expired tokens setiap 1 jam
func startTokenCleanup(db *pgxpool.Pool) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Jalankan cleanup pertama kali saat startup
	if err := utils.CleanupExpiredTokens(context.Background(), db); err != nil {
		log.Printf("Initial token cleanup failed: %v", err)
	} else {
		log.Println("Initial token cleanup completed")
	}

	// Loop untuk cleanup berkala
	for range ticker.C {
		if err := utils.CleanupExpiredTokens(context.Background(), db); err != nil {
			log.Printf("Token cleanup failed: %v", err)
		} else {
			log.Println("Token cleanup completed")
		}
	}
}
