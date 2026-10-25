package chrome

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

func TestPickHeroAttribute(t *testing.T) {
	now := time.Now()
	pastDue := util.StartOfDay(now.Add(-48 * time.Hour))
	futureDue := util.StartOfDay(now.Add(48 * time.Hour))
	today := util.StartOfDay(now)

	tests := []struct {
		name string
		task *models.Task
		want HeroKind
	}{
		{
			name: "nil task → HeroNone",
			task: nil,
			want: HeroNone,
		},
		{
			name: "overdue + review + due-today + P1 → HeroOverdue (precedence top)",
			task: &models.Task{
				ID:            "t-overdue-everything",
				WorkflowState: models.StateBacklog,
				Priority:      1,
				Review:        true,
				DueDate:       &pastDue,
			},
			want: HeroOverdue,
		},
		{
			name: "review flag + due today + P1 → HeroReview (beats due today)",
			task: &models.Task{
				ID:            "t-review-flag",
				WorkflowState: models.StateBacklog,
				Priority:      1,
				Review:        true,
				DueDate:       &today,
			},
			want: HeroReview,
		},
		{
			name: "arrived check-in → HeroReview",
			task: &models.Task{
				ID:            "t-check-in-due",
				WorkflowState: models.StateBacklog,
				Priority:      3,
				NextCheckIn:   &today,
			},
			want: HeroReview,
		},
		{
			name: "check-in still ahead → no hero of its own",
			task: &models.Task{
				ID:            "t-check-in-ahead",
				WorkflowState: models.StateBacklog,
				Priority:      3,
				NextCheckIn:   &futureDue,
			},
			want: HeroNone,
		},
		{
			name: "due today + P1 → HeroDueToday (beats P1)",
			task: &models.Task{ID: "t-due-today-p1", WorkflowState: models.StateBacklog, Priority: 1, DueDate: &today},
			want: HeroDueToday,
		},
		{
			name: "due today + P3 → HeroDueToday",
			task: &models.Task{ID: "t-due-today", WorkflowState: models.StateBacklog, Priority: 3, DueDate: &today},
			want: HeroDueToday,
		},
		{
			name: "P1 only → HeroP1",
			task: &models.Task{ID: "t-p1", WorkflowState: models.StateBacklog, Priority: 1, DueDate: &futureDue},
			want: HeroP1,
		},
		{
			name: "todo P3 no due → HeroNone",
			task: &models.Task{ID: "t-mundane", WorkflowState: models.StateBacklog, Priority: 3},
			want: HeroNone,
		},
		{
			name: "waiting alone stays quiet",
			task: &models.Task{ID: "t-waiting", WorkflowState: models.StateWaiting, Priority: 3},
			want: HeroNone,
		},
		{
			name: "waiting P1 earns the hero every P1 earns",
			task: &models.Task{ID: "t-waiting-p1", WorkflowState: models.StateWaiting, Priority: 1},
			want: HeroP1,
		},
		{
			name: "waiting overdue is overdue, not a state hero",
			task: &models.Task{ID: "t-waiting-overdue", WorkflowState: models.StateWaiting, Priority: 3, DueDate: &pastDue},
			want: HeroOverdue,
		},
		{
			name: "done P1 → HeroNone (completed is quiet)",
			task: &models.Task{ID: "t-done", WorkflowState: models.StateDone, Priority: 1, DueDate: &pastDue},
			want: HeroNone,
		},
		{
			name: "canceled P1 overdue → HeroNone (canceled is quiet)",
			task: &models.Task{ID: "t-canceled", WorkflowState: models.StateCanceled, Priority: 1, DueDate: &pastDue},
			want: HeroNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PickHeroAttribute(tc.task, now)
			if got != tc.want {
				t.Errorf("PickHeroAttribute() = %v, want %v", got, tc.want)
			}
			if gotPriority, want := PriorityIsHero(got), tc.want == HeroP1; gotPriority != want {
				t.Errorf("PriorityIsHero(%v) = %v, want %v", got, gotPriority, want)
			}
		})
	}
}

// TestPickHeroAttributeReadsTheSuppliedInstant answers overdue and due-today
// against the row's instant, not the wall clock, so one render pass cannot
// call a deadline overdue in its tint and due today in its badge.
func TestPickHeroAttributeReadsTheSuppliedInstant(t *testing.T) {
	now := time.Date(2031, time.February, 3, 10, 0, 0, 0, time.Local)
	due := util.StartOfDay(now)
	task := &models.Task{ID: "t-due", WorkflowState: models.StateBacklog, DueDate: &due}
	if got := PickHeroAttribute(task, now); got != HeroDueToday {
		t.Fatalf("PickHeroAttribute on its due day = %v, want HeroDueToday", got)
	}
	if got := PickHeroAttribute(task, now.AddDate(0, 0, 1)); got != HeroOverdue {
		t.Fatalf("PickHeroAttribute the day after = %v, want HeroOverdue", got)
	}
	if got := PickHeroAttribute(task, now.AddDate(0, 0, -1)); got != HeroNone {
		t.Fatalf("PickHeroAttribute the day before = %v, want HeroNone", got)
	}
}

func TestDemoteBadgeEmpty(t *testing.T) {
	if got := DemoteBadge(""); got != "" {
		t.Errorf("demoteBadge(\"\") = %q, want \"\"", got)
	}
}

func TestDemoteBadgePreservesGlyph(t *testing.T) {
	original := theme.GetStateBadge(models.StateDone)
	if original == "" {
		t.Fatal("expected non-empty status badge")
	}
	demoted := DemoteBadge(original)
	if demoted == "" {
		t.Fatal("demoteBadge returned empty string for non-empty input")
	}
	if demoted == original {
		t.Error("expected demoted badge to differ from original (different style)")
	}
}
