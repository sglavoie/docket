package focus

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestCollectActionableTasksAtUsesCalendarDateSnoozeBoundary(t *testing.T) {
	referenceLocation := time.FixedZone("reference", 9*60*60)
	snoozeLocation := time.FixedZone("snooze", -8*60*60)
	now := time.Date(2026, 3, 10, 0, 30, 0, 0, referenceLocation)
	sameDay := time.Date(2026, 3, 10, 0, 0, 0, 0, snoozeLocation)
	nextDay := time.Date(2026, 3, 11, 0, 0, 0, 0, snoozeLocation)

	visible := &models.Task{ID: "visible", WorkflowState: models.StateReady}
	reappearsToday := &models.Task{
		ID:            "same-day",
		WorkflowState: models.StateReady,
		SnoozedUntil:  &sameDay,
	}
	stillSnoozed := &models.Task{
		ID:            "next-day",
		WorkflowState: models.StateReady,
		SnoozedUntil:  &nextDay,
	}
	terminal := &models.Task{ID: "done", WorkflowState: models.StateDone}
	project := &models.Project{
		ID:    "project",
		Tasks: []*models.Task{visible, reappearsToday, stillSnoozed, terminal},
	}

	got := CollectActionableTasksAt([]*models.Project{project}, now)
	ids := make([]string, 0, len(got))
	for _, item := range got {
		ids = append(ids, item.Task.ID)
	}
	if !slices.Equal(ids, []string{"visible", "same-day"}) {
		t.Fatalf("actionable IDs = %v, want open work including the task whose snooze expires today", ids)
	}
}
