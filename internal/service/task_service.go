package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// cancelKeyFmt mirrors the key used in the task queue package.
// Defined here to avoid an import cycle (service → task).
const cancelKeyFmt = "task:cancel:%s"

type taskService struct {
	repo        port.TaskRepository
	redisClient *redis.Client
	log         logger.Logger
}

// NewTaskService creates a new task service.
// redisClient is used to signal cancellation to running workers.
func NewTaskService(repo port.TaskRepository, redisClient *redis.Client, log logger.Logger) port.TaskService {
	return &taskService{
		repo:        repo,
		redisClient: redisClient,
		log:         log,
	}
}

func (s *taskService) GetByID(ctx context.Context, id string) (*model.Task, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *taskService) List(ctx context.Context, page port.Page) (*port.PageResult[model.Task], error) {
	return s.repo.List(ctx, page)
}

func (s *taskService) ListByVMID(ctx context.Context, vmID string, page port.Page) (*port.PageResult[model.Task], error) {
	return s.repo.ListByVMID(ctx, vmID, page)
}

// Cancel marks a task as cancelled in the DB and sets the Redis cancellation
// flag so a running handler can detect it cooperatively.
func (s *taskService) Cancel(ctx context.Context, id string) error {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if t.Status == model.TaskStatusCompleted ||
		t.Status == model.TaskStatusFailed ||
		t.Status == model.TaskStatusCancelled {
		return fmt.Errorf("task %s is already in terminal state %s", id, t.Status)
	}

	// Set the Redis cancel flag so a running handler sees it on the next
	// IsCancelled() check. TTL of 1 hour is generous — the engine clears it
	// once the handler acknowledges the cancellation.
	key := fmt.Sprintf(cancelKeyFmt, id)
	if err := s.redisClient.Set(ctx, key, "1", time.Hour).Err(); err != nil {
		// Non-fatal: the DB status update below is the authoritative record.
		s.log.Warn("failed to set Redis cancel flag",
			logger.String("task_id", id),
			logger.Error(err),
		)
	}

	return s.repo.UpdateStatus(ctx, id, model.TaskStatusCancelled, nil, "cancelled by user")
}

// GetLogs returns paginated log entries for a task from Postgres.
func (s *taskService) GetLogs(ctx context.Context, taskID string, page port.Page) (*port.PageResult[model.TaskLog], error) {
	return s.repo.GetLogs(ctx, taskID, page)
}
