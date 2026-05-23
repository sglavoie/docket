package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/util"
)

func newTestWeeklyReviewModel(cfg *config.Config) *WeeklyReviewModel {
	return &WeeklyReviewModel{
		ViewBundle:        ViewBundle{config: cfg},
		collapsedSections: make(map[int]bool),
		reviewedSections:  make(map[WeeklyReviewSection]bool),
		mutationCounts:    make(map[string]int),
		tabs:              TabState{Names: weeklyReviewTabNames},
	}
}

func TestWeeklyReview_EmptyState(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestWeeklyReviewModel(cfg)
	m.SetSize(80, 24)
	m.calculateNavigation()

	output := m.View().Content
	if output == "" {
		t.Error("expected non-empty output for empty weekly review")
	}
	if !strings.Contains(output, "Weekly Review") {
		t.Error("expected output to contain 'Weekly Review' title")
	}
	if !strings.Contains(output, "all clear") {
		t.Error("expected output to contain 'all clear' subtitle")
	}
}

func TestWeeklyReview_WithStaleProjects(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestWeeklyReviewModel(cfg)
	m.SetSize(120, 40)

	project, err := models.NewProject("p1", "Stale Project", "")
	if err != nil {
		t.Fatal(err)
	}
	m.staleProjects = []WeeklyReviewProject{{
		Project:        project,
		NoActivityDays: 30,
	}}

	m.calculateNavigation()

	output := m.View().Content
	if !strings.Contains(output, "Stale Project") {
		t.Error("expected output to contain 'Stale Project'")
	}
}

func TestWeeklyReview_ReviewCompletion(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestWeeklyReviewModel(cfg)
	m.SetSize(80, 24)

	// Mark all sections reviewed
	allSections := []WeeklyReviewSection{
		WeeklySectionStaleProjects, WeeklySectionInboxBacklog,
		WeeklySectionBlockedWork, WeeklySectionOvercommitted,
		WeeklySectionStaleTasks, WeeklySectionPriorityMismatch,
		WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview,
	}
	for _, s := range allSections {
		m.reviewedSections[s] = true
	}

	count := m.reviewedCount()
	if count != 8 {
		t.Errorf("expected 8 reviewed sections, got %d", count)
	}
}

func TestWeeklyReview_TabCounts(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestWeeklyReviewModel(cfg)

	project, err := models.NewProject("p1", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	m.staleProjects = []WeeklyReviewProject{{Project: project, NoActivityDays: 30}}
	m.inboxBacklog = []WeeklyReviewProject{{Project: project, InboxCount: 10}}

	projectCount, taskCount := m.countByType()
	if projectCount != 2 {
		t.Errorf("expected 2 project items, got %d", projectCount)
	}
	if taskCount != 0 {
		t.Errorf("expected 0 task items, got %d", taskCount)
	}
}

// TestWeeklyReview_UpcomingWeekWindowIsSevenDays verifies that the "Upcoming Week"
// section spans exactly 7 days (today through today+6). A task due today+7 must
// not be included; otherwise the window is 8 days, contradicting the section name
// and the in-code comment.
func TestWeeklyReview_UpcomingWeekWindowIsSevenDays(t *testing.T) {
	mockRepo := testutil.NewMockRepository()
	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = mockRepo
	defer func() { storage.DefaultRepository = oldRepo }()

	today := util.StartOfDay(time.Now())

	// Build stored project with two tasks.
	storedProject, err := models.NewProject("p1", "Proj", "")
	if err != nil {
		t.Fatal(err)
	}
	dueIn6 := today.AddDate(0, 0, 6).Add(12 * time.Hour)
	task6, err := models.NewTask("t6", 1, models.TaskTypeTask, "In window")
	if err != nil {
		t.Fatal(err)
	}
	task6.DueDate = &dueIn6
	task6.Status = models.StatusTodo

	dueIn7 := today.AddDate(0, 0, 7).Add(12 * time.Hour)
	task7, err := models.NewTask("t7", 2, models.TaskTypeTask, "Out of window")
	if err != nil {
		t.Fatal(err)
	}
	task7.DueDate = &dueIn7
	task7.Status = models.StatusTodo
	storedProject.Tasks = []*models.Task{task6, task7}
	mockRepo.Projects[storedProject.ID] = storedProject

	// Use a separate project pointer in config so MockRepository.Load can
	// deep-copy without aliasing the source.
	configProject := &models.Project{ID: storedProject.ID, Name: storedProject.Name}
	cfg := &config.Config{Projects: []*models.Project{configProject}}
	m := newTestWeeklyReviewModel(cfg)
	m.loadData()

	var titles []string
	for _, wt := range m.upcomingWeek {
		titles = append(titles, wt.Task.Title)
	}

	foundIn := false
	foundOut := false
	for _, title := range titles {
		if title == "In window" {
			foundIn = true
		}
		if title == "Out of window" {
			foundOut = true
		}
	}
	if !foundIn {
		t.Errorf("expected task due in 6 days to appear in upcoming week, got titles=%v", titles)
	}
	if foundOut {
		t.Errorf("task due 7 days from today must not appear in 'Upcoming Week' (window should be 7 days, not 8); got titles=%v", titles)
	}
}

func TestWeeklyReview_CompletionSummary(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestWeeklyReviewModel(cfg)
	m.SetSize(80, 24)

	m.showingSummary = true
	m.mutationCounts["status"] = 3
	m.mutationCounts["priority"] = 1

	// Need some data so the model has content to render
	project, err := models.NewProject("p1", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	m.staleProjects = []WeeklyReviewProject{{Project: project, NoActivityDays: 5}}
	m.calculateNavigation()

	output := m.View().Content
	if !strings.Contains(output, "Review Complete") {
		t.Error("expected output to contain 'Review Complete'")
	}
	if !strings.Contains(output, "Status changes: 3") {
		t.Error("expected output to contain 'Status changes: 3'")
	}
}
