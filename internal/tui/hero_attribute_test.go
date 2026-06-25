package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestPickHeroAttribute(t *testing.T) {
	pastDue := time.Now().Add(-48 * time.Hour)
	futureDue := time.Now().Add(48 * time.Hour)
	today := time.Now()

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
			name: "overdue + P1 + blocked + due-today → HeroOverdue (precedence top)",
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
			name: "explicit StatusBlocked + P1 → HeroBlocked",
			task: &models.Task{
				ID:       "t-status-blocked",
				Status:   models.StatusBlocked,
				Priority: 1,
			},
			project: &models.Project{},
			want:    HeroBlocked,
		},
		{
			name: "explicit StatusBlocked without deps still → HeroBlocked",
			task: &models.Task{
				ID:       "t-status-blocked-only",
				Status:   models.StatusBlocked,
				Priority: 3,
			},
			project: &models.Project{},
			want:    HeroBlocked,
		},
		{
			name:    "due today + P1 → HeroDueToday (beats P1)",
			task:    &models.Task{ID: "t-due-today-p1", Status: models.StatusTodo, Priority: 1, DueDate: &today},
			project: &models.Project{},
			want:    HeroDueToday,
		},
		{
			name:    "due today + review + P3 → HeroDueToday (beats review)",
			task:    &models.Task{ID: "t-due-today-review", Status: models.StatusReview, Priority: 3, DueDate: &today},
			project: &models.Project{},
			want:    HeroDueToday,
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
			name:    "done P1 → HeroNone (completed is quiet)",
			task:    &models.Task{ID: "t-done", Status: models.StatusDone, Priority: 1, DueDate: &pastDue},
			project: &models.Project{},
			want:    HeroNone,
		},
		{
			name:    "canceled P1 overdue → HeroNone (canceled is quiet)",
			task:    &models.Task{ID: "t-canceled", Status: models.StatusCanceled, Priority: 1, DueDate: &pastDue},
			project: &models.Project{},
			want:    HeroNone,
		},
		{
			name:    "dimmed overdue P1 → HeroNone (dimmed is quiet)",
			task:    &models.Task{ID: "t-dimmed", Status: models.StatusTodo, Priority: 1, DueDate: &pastDue, Dimmed: true},
			project: &models.Project{},
			want:    HeroNone,
		},
		{
			name: "dimmed blocked → HeroNone (dimmed beats blocked)",
			task: &models.Task{
				ID:        "t-dimmed-blocked",
				Status:    models.StatusBlocked,
				Priority:  1,
				DependsOn: []string{"dep-1"},
				Dimmed:    true,
			},
			project: &models.Project{Tasks: []*models.Task{depPending}},
			want:    HeroNone,
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

func TestDueIsInlineHeroHonorsQuietStates(t *testing.T) {
	today := time.Now()

	open := &models.Task{ID: "open", Status: models.StatusTodo, DueDate: &today}
	done := &models.Task{ID: "done", Status: models.StatusDone, DueDate: &today}
	canceled := &models.Task{ID: "canceled", Status: models.StatusCanceled, DueDate: &today}
	dimmed := &models.Task{ID: "dimmed", Status: models.StatusTodo, DueDate: &today, Dimmed: true}

	if !dueIsInlineHero(open) {
		t.Fatal("open due-today task should be eligible for inline due hero")
	}
	for _, task := range []*models.Task{done, canceled, dimmed} {
		if dueIsInlineHero(task) {
			t.Fatalf("%s task should not be eligible for inline due hero", task.Status)
		}
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
