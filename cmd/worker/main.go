package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"shopedia-api/internal/queue"
	"shopedia-api/internal/repository"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file (for local development)
	godotenv.Load()

	log.Println("Starting Shopedia Worker...")

	// Connect to database
	db, err := repository.ConnectDB(os.Getenv("DB_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("DB connected")

	// Setup Redis connection
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	// Create worker server
	srv := asynq.NewServer(
		opt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Printf("[Error] Task %s failed: %v", task.Type(), err)
			}),
		},
	)

	// Create task handler
	handler := queue.NewTaskHandler(db)

	// Setup task handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeSendEmail, handler.HandleSendEmail)
	mux.HandleFunc(queue.TypeSendOTP, handler.HandleSendOTP)
	mux.HandleFunc(queue.TypeSendWelcome, handler.HandleSendWelcome)
	mux.HandleFunc(queue.TypeSendPasswordReset, handler.HandleSendPasswordReset)
	mux.HandleFunc(queue.TypeNotification, handler.HandleNotification)
	mux.HandleFunc(queue.TypeProductIndexing, handler.HandleProductIndexing)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down worker...")
		srv.Shutdown()
	}()

	log.Println("Worker is ready to process tasks")
	log.Printf("Queues: critical(6), default(3), low(1)")
	log.Printf("Concurrency: 10")

	// Start processing
	if err := srv.Run(mux); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}
