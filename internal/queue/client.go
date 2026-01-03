package queue

import (
	"os"

	"github.com/hibiken/asynq"
)

var Client *asynq.Client

// InitClient initializes the Asynq client for enqueueing tasks
func InitClient() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		// Fallback to default
		opt = asynq.RedisClientOpt{Addr: "localhost:6379"}
	}

	Client = asynq.NewClient(opt)
}

// CloseClient closes the Asynq client
func CloseClient() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}

// Enqueue adds a task to the queue
func Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return Client.Enqueue(task, opts...)
}
