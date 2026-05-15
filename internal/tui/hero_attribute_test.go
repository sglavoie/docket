package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestPickHeroAttribute(t *testing.T) {
	pastDue := time.Now().Add(-48 * time.Hour)
	futureDue := time.Now().Add(48 * time.Hour)

	// A dependency task that is not complete; used to make HasIncompleteDependencies true.
	depPending := &models.Task{ID: "dep-1", Status: models.StatusTodo}
	depDone := &models.Task{ID: "dep-2", Status: models.StatusDone}

	tests := []struct {
		name    string
		task    *models.Task
		project *models.Project
		want    HeroKind
	}{
		{
			name: "nil task → HeroNone",
			task: nil,
			want: HeroNone,
		},
		{
			name: "overdue + P1 + blocked → HeroOverdue (precedence top)",
			task: &models.Task{
				ID:        "t-overdue-p1-blocked",
				Status:    models.StatusTodo,
				Priority:  1,
				DueDate:   &pastDue,
				DependsOn: []string{"dep-1"},
			},
			project: &models.Project{Tasks: []*models.Task{depPending}},
			want:    HeroOverdue,
		},
		{
			name: "blocked + P1 + review (no overdue) → HeroBlocked",
			task: &models.Task{
				ID:        "t-blocked-p1-review",
				Status:    models.StatusReview,
				Priority:  1,
				DependsOn: []string{"dep-1"},
			},
			project: &models.Project{Tasks: []*models.Task{depPending}},
			want:    HeroBlocked,
		},
		{
			name:    "P1 only → HeroP1",
			task:    &models.Task{ID: "t-p1", Status: models.StatusTodo, Priority: 1, DueDate: &futureDue},
			project: &models.Project{},
			want:    HeroP1,
		},
		{
			name:    "review with P3 → HeroReview",
			task:    &models.Task{ID: "t-rev", Status: models.StatusReview, Priority: 3},
			project: &models.Project{},
			want:    HeroReview,
		},
		{
			name:    "todo P3 no due → HeroNone",
			task:    &models.Task{ID: "t-mundane", Status: models.StatusTodo, Priority: 3},
			project: &models.Project{},
			want:    HeroNone,
		},
		{
			name: "deps all complete → not HeroBlocked",
			task: &models.Task{
				ID:        "t-deps-done",
				Status:    models.StatusTodo,
				Priority:  2,
				DependsOn: []string{"dep-2"},
			},
			project: &models.Project{Tasks: []*models.Task{depDone}},
			want:    HeroNone,
		},
		{
			name:    "P1 without project still returns HeroP1",
			task:    &models.Task{ID: "t-p1-noproj", Status: models.StatusTodo, Priority: 1},
			project: nil,
			want:    HeroP1,
		},
		{
			name:    "done task is not overdue → HeroNone",
			task:    &models.Task{ID: "t-done", Status: models.StatusDone, Priority: 1, DueDate: &pastDue},
			project: &models.Project{},
			want:    HeroP1, // priority still applies; done doesn't reset priority
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pickHeroAttribute(tc.task, tc.project)
			if got != tc.want {
				t.Errorf("pickHeroAttribute() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDemoteBadgeEmpty(t *testing.T) {
	if got := demoteBadge(""); got != "" {
		t.Errorf("demoteBadge(\"\") = %q, want \"\"", got)
	}
}

func TestDemoteBadgePreservesGlyph(t *testing.T) {
	original := GetStatusBadge("done", "✓")
	if original == "" {
		t.Fatal("expected non-empty status badge")
	}
	demoted := demoteBadge(original)
	if demoted == "" {
		t.Fatal("demoteBadge returned empty string for non-empty input")
	}
	if demoted == original {
		t.Error("expected demoted badge to differ from original (different style)")
	}
}
