package metrics

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

func makeTask(status models.TaskStatus, completedAt *time.Time, activity []*models.ActivityEntry) *models.Task {
	return &models.Task{
		ID:          "test-task",
		Status:      status,
		CompletedAt: completedAt,
		Activity:    activity,
	}
}
