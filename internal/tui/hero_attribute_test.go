package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

func TestPickHeroAttribute(t *testing.T) {
	pastDue := util.StartOfDay(time.Now().Add(-48 * time.Hour))
	futureDue := util.StartOfDay(time.Now().Add(48 * time.Hour))
	today := util.StartOfDay(time.Now())

	tests := []struct {
		name    string
		task    *models.Task
		project *models.Project
		want    chrome.HeroKind
	}{
		{
			name: "nil task → HeroNone",
			task: nil,
			want: chrome.HeroNone,
		},
		{
			name: "overdue + P1 + due-today → HeroOverdue (precedence top)",
			task: &models.Task{
				ID:            "t-overdue-p1-waiting",
				WorkflowState: models.StateBacklog,
				Priority:      1,
				DueDate:       &pastDue,
			},
			project: &models.Project{},
			want:    chrome.HeroOverdue,
		},
		{
			name: "explicit StateWaiting + P1 → HeroWaiting",
			task: &models.Task{
				ID:            "t-status-waiting",
				WorkflowState: models.StateWaiting,
				Priority:      1,
			},
			project: &models.Project{},
			want:    chrome.HeroWaiting,
		},
		{
			name: "explicit StateWaiting alone → HeroWaiting",
			task: &models.Task{
				ID:            "t-status-waiting-only",
				WorkflowState: models.StateWaiting,
				Priority:      3,
			},
			project: &models.Project{},
			want:    chrome.HeroWaiting,
		},
		{
			name:    "due today + P1 → HeroDueToday (beats P1)",
			task:    &models.Task{ID: "t-due-today-p1", WorkflowState: models.StateBacklog, Priority: 1, DueDate: &today},
			project: &models.Project{},
			want:    chrome.HeroDueToday,
		},
		{
			name:    "due today + P3 → HeroDueToday",
			task:    &models.Task{ID: "t-due-today", WorkflowState: models.StateBacklog, Priority: 3, DueDate: &today},
			project: &models.Project{},
			want:    chrome.HeroDueToday,
		},
		{
			name:    "P1 only → HeroP1",
			task:    &models.Task{ID: "t-p1", WorkflowState: models.StateBacklog, Priority: 1, DueDate: &futureDue},
			project: &models.Project{},
			want:    chrome.HeroP1,
		},
		{
			name:    "todo P3 no due → HeroNone",
			task:    &models.Task{ID: "t-mundane", WorkflowState: models.StateBacklog, Priority: 3},
			project: &models.Project{},
			want:    chrome.HeroNone,
		},
		{
			name:    "P1 without project still returns HeroP1",
			task:    &models.Task{ID: "t-p1-noproj", WorkflowState: models.StateBacklog, Priority: 1},
			project: nil,
			want:    chrome.HeroP1,
		},
		{
			name:    "done P1 → HeroNone (completed is quiet)",
			task:    &models.Task{ID: "t-done", WorkflowState: models.StateDone, Priority: 1, DueDate: &pastDue},
			project: &models.Project{},
			want:    chrome.HeroNone,
		},
		{
			name:    "canceled P1 overdue → HeroNone (canceled is quiet)",
			task:    &models.Task{ID: "t-canceled", WorkflowState: models.StateCanceled, Priority: 1, DueDate: &pastDue},
			project: &models.Project{},
			want:    chrome.HeroNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := chrome.PickHeroAttribute(tc.task)
			if got != tc.want {
				t.Errorf("pickHeroAttribute() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDemoteBadgeEmpty(t *testing.T) {
	if got := chrome.DemoteBadge(""); got != "" {
		t.Errorf("demoteBadge(\"\") = %q, want \"\"", got)
	}
}

func TestDemoteBadgePreservesGlyph(t *testing.T) {
	original := theme.GetStateBadge(models.StateDone)
	if original == "" {
		t.Fatal("expected non-empty status badge")
	}
	demoted := chrome.DemoteBadge(original)
	if demoted == "" {
		t.Fatal("demoteBadge returned empty string for non-empty input")
	}
	if demoted == original {
		t.Error("expected demoted badge to differ from original (different style)")
	}
}
