package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// pipelineCounts holds aggregated task counts shown in the Pipeline Hub.
type pipelineCounts struct {
	inboxCount          int // processable root inbox tasks across all projects
	queueLen            int // number of IDs in the focus queue
	dailyPlanLen        int // number of IDs in the daily plan
	overdueCount        int // overdue non-complete tasks
	inProgressCount     int // tasks with in-progress status
	completedTodayCount int // tasks completed today
}

// loadPipelineCounts aggregates counts from all active projects.
func loadPipelineCounts(cfg *config.Config, queueIDs, dailyPlanIDs map[string]bool) pipelineCounts {
	counts := pipelineCounts{
		queueLen:     len(queueIDs),
		dailyPlanLen: len(dailyPlanIDs),
	}

	today := time.Now()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	for _, project := range cfg.GetActiveProjects() {
		if err := storage.LoadProject(project); err != nil {
			logging.Warn("pipeline hub: failed to load project %s: %v", project.Name, err)
			continue
		}
		for _, task := range project.Tasks {
			if !task.IsRoot() {
				continue
			}
			counts.inboxCount += countInbox(task)
			counts.overdueCount += countOverdue(task)
			counts.inProgressCount += countInProgress(task)
			counts.completedTodayCount += countCompletedToday(task, startOfDay)
		}
	}

	return counts
}

func countInbox(task *models.Task) int {
	if isProcessableInboxTask(task) {
		return 1
	}
	return 0
}

func countOverdue(task *models.Task) int {
	n := 0
	if task.IsOverdue() && !task.IsComplete() {
		n++
	}
	for _, child := range task.Children {
		n += countOverdue(child)
	}
	return n
}

func countInProgress(task *models.Task) int {
	n := 0
	if task.Status == models.StatusInProgress {
		n++
	}
	for _, child := range task.Children {
		n += countInProgress(child)
	}
	return n
}

func countCompletedToday(task *models.Task, startOfDay time.Time) int {
	n := 0
	if task.IsComplete() && task.CompletedAt != nil && !task.CompletedAt.Before(startOfDay) {
		n++
	}
	for _, child := range task.Children {
		n += countCompletedToday(child, startOfDay)
	}
	return n
}
