package metrics

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TaskSummary is the compact task shape needed by metrics hot paths.
// Archived callers can provide these directly without constructing full tasks.
type TaskSummary struct {
	ID             string
	Status         models.TaskStatus
	CompletedAt    *time.Time
	FirstStartedAt *time.Time
	EstimatedDays  float64
	Effort         models.EffortEstimate
	StoryPoints    int
}

func TaskSummaryFromTask(task *models.Task) (TaskSummary, bool) {
	if task == nil {
		return TaskSummary{}, false
	}

	startedAt := firstStartedAtFromTask(task)
	return TaskSummary{
		ID:             task.ID,
		Status:         task.Status,
		CompletedAt:    task.CompletedAt,
		FirstStartedAt: startedAt,
		EstimatedDays:  task.EstimatedDays,
		Effort:         taskEffort(task),
		StoryPoints:    taskStoryPoints(task),
	}, true
}

func TaskSummariesFromTasks(tasks []*models.Task) []TaskSummary {
	summaries := make([]TaskSummary, 0, len(tasks))
	for _, task := range tasks {
		summary, ok := TaskSummaryFromTask(task)
		if !ok {
			continue
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func firstStartedAtFromTask(task *models.Task) *time.Time {
	startedAt := findFirstInProgress(task)
	if startedAt.IsZero() {
		return nil
	}
	return &startedAt
}

func taskEffort(task *models.Task) models.EffortEstimate {
	if task.TaskAttrs != nil && task.TaskAttrs.Effort != "" {
		return task.TaskAttrs.Effort
	}
	if task.IdeaAttrs != nil && task.IdeaAttrs.Effort != "" {
		return task.IdeaAttrs.Effort
	}
	return ""
}

func taskStoryPoints(task *models.Task) int {
	if task.StoryAttrs == nil {
		return 0
	}
	return task.StoryAttrs.StoryPoints
}
