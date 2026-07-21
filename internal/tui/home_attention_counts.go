package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// attentionCounts holds the aggregate signals rendered by Home Attention.
type attentionCounts struct {
	inboxCount          int
	queueLen            int
	dailyPlanLen        int
	overdueCount        int
	inProgressCount     int
	completedTodayCount int
}

func loadAttentionCounts(runtime *config.RuntimeState, queueIDs, dailyPlanIDs map[string]bool) (attentionCounts, []*models.Project) {
	_ = queueIDs
	counts := attentionCounts{dailyPlanLen: len(dailyPlanIDs)}
	if runtime == nil {
		return counts, nil
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	active := runtime.ActiveProjects()
	loaded := make([]*models.Project, 0, len(active))
	for _, project := range active {
		if err := storage.LoadProject(project); err != nil {
			logging.Warn("Home Attention: failed to load project %s: %v", project.Name, err)
			continue
		}
		loaded = append(loaded, project)
		for _, task := range project.Tasks {
			if !task.IsRoot() {
				continue
			}
			counts.inboxCount += countInbox(task)
			counts.overdueCount += countOverdue(task)
			counts.inProgressCount += countInProgress(task)
			counts.completedTodayCount += countCompletedToday(task, startOfDay)
			counts.queueLen += countNext(task)
		}
	}
	return counts, loaded
}

func countInbox(task *models.Task) int {
	n := 0
	if isProcessableInboxTask(task) {
		n++
	}
	for _, child := range task.Children {
		n += countInbox(child)
	}
	return n
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

func countNext(task *models.Task) int {
	n := 0
	if task.State() == models.StateNext {
		n++
	}
	for _, child := range task.Children {
		n += countNext(child)
	}
	return n
}

func countInProgress(task *models.Task) int {
	n := 0
	if task.State() == models.StateInProgress {
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
