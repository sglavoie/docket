package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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

	output := m.View()
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

	project := models.NewProject("p1", "Stale Project", "")
	m.staleProjects = []WeeklyReviewProject{{
		Project:        project,
		NoActivityDays: 30,
	}}

	m.calculateNavigation()

	output := m.View()
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

	project := models.NewProject("p1", "Test", "")
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

func TestWeeklyReview_CompletionSummary(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestWeeklyReviewModel(cfg)
	m.SetSize(80, 24)

	m.showingSummary = true
	m.mutationCounts["status"] = 3
	m.mutationCounts["priority"] = 1

	// Need some data so the model has content to render
	project := models.NewProject("p1", "Test", "")
	m.staleProjects = []WeeklyReviewProject{{Project: project, NoActivityDays: 5}}
	m.calculateNavigation()

	output := m.View()
	if !strings.Contains(output, "Review Complete") {
		t.Error("expected output to contain 'Review Complete'")
	}
	if !strings.Contains(output, "Status changes: 3") {
		t.Error("expected output to contain 'Status changes: 3'")
	}
}
