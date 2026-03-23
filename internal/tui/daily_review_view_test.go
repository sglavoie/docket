package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newTestDailyReviewModel(cfg *config.Config) *DailyReviewModel {
	return &DailyReviewModel{
		ViewBundle:        ViewBundle{config: cfg},
		collapsedSections: make(map[int]bool),
		FilterMixin:       FilterMixin{filter: newFilterState()},
		tabs:              TabState{Names: dailyReviewTabNames},
	}
}

func newTestDailyReviewTask(t *testing.T, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestDailyReview_EmptyState(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(80, 24)
	m.calculateNavigation()

	output := m.View().Content
	if output == "" {
		t.Error("expected non-empty output for empty daily review")
	}
	if !strings.Contains(output, "Review") {
		t.Error("expected output to contain 'Review' title")
	}
}

func TestDailyReview_SectionNavigation(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(80, 24)

	project, err := models.NewProject("p1", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Add overdue and due-today tasks (both on the "Due" tab, index 0)
	overdueTask := newTestDailyReviewTask(t, "Overdue task")
	past := time.Now().AddDate(0, 0, -5)
	overdueTask.DueDate = &past
	m.overdue = []CrossProjectTask{{Task: overdueTask, Project: project}}

	dueTodayTask := newTestDailyReviewTask(t, "Due today task")
	today := time.Now()
	dueTodayTask.DueDate = &today
	m.dueToday = []CrossProjectTask{{Task: dueTodayTask, Project: project}}

	m.calculateNavigation()

	// Default tab is "Due" (index 0), which shows overdue + dueToday sections
	// Each section has 1 header + 1 task = 2 items, total = 4
	if m.totalTasks != 4 {
		t.Errorf("expected 4 total navigable items, got %d", m.totalTasks)
	}

	if len(m.sectionOrder) != 2 {
		t.Errorf("expected 2 sections, got %d", len(m.sectionOrder))
	}
}

func TestDailyReview_ReviewCompletion(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(80, 24)

	m.reviewCompleted = true
	m.completedAt = time.Now()
	m.calculateNavigation()

	output := m.View().Content
	if !strings.Contains(output, "completed today at") {
		t.Error("expected output to contain completion message")
	}
}

func TestDailyReview_WithTasks_Render(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(120, 40)

	project, err := models.NewProject("p1", "Test Project", "")
	if err != nil {
		t.Fatal(err)
	}

	task := newTestDailyReviewTask(t, "Important task")
	task.Status = models.StatusInProgress
	m.inProgress = []CrossProjectTask{{Task: task, Project: project}}

	// Switch to Active tab (index 1) which shows in_progress and blocked
	m.tabs.Active = 1
	m.calculateNavigation()

	output := m.View().Content
	if !strings.Contains(output, "Important task") {
		t.Error("expected output to contain 'Important task'")
	}
}
