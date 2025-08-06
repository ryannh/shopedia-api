package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"

	"shopedia-api/internal/handler"
	"shopedia-api/internal/repository"
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
	} else {
		fmt.Println("DB connected")
	}

	// Fiber instance
	app := fiber.New()

	// Routes
	handler.SetupRoutes(app, db)

	// Start
	port := ":8081"
	fmt.Println("Server running on", port)
	log.Fatal(app.Listen(port))
}
