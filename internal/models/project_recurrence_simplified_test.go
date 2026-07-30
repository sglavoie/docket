package models

import (
	"testing"
	"time"
)

//nolint:gocyclo // One table-driven assertion covers every retained and discarded task field.
func TestGenerateRecurringInstancesUsesExactCopyPolicy(t *testing.T) {
	now := dateAtNoon(2026, time.July, 10)
	due := date(2026, time.July, 1)
	parentID := "parent"
	pinnedAt := date(2026, time.June, 1)
	task := &Task{
		ID:            "source",
		ShortID:       7,
		Title:         "Ship recurring report",
		Description:   "Use the standard template",
		WorkflowState: StateDone,
		Priority:      2,
		ParentID:      &parentID,
		Tags:          []string{"ops", "monthly"},
		CompletedAt:   &now,
		DueDate:       &due,
		DependsOn:     []string{"dependency"},
		Pinned:        true,
		PinnedAt:      &pinnedAt,
		Comments:      []*Comment{{Text: "private history", CreatedAt: now}},
		Decisions:     []*Decision{{Text: "private decision", CreatedAt: now}},
		Attachments:   []Attachment{{Filename: "secret.txt"}},
		SnoozedUntil:  &pinnedAt,
		Checklist:     []ChecklistItem{{Text: "Draft", Done: true}, {Text: "Send"}},
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
	}
	project := &Project{ID: "project", Tasks: []*Task{task}, NextShortID: 8}
	idCalls := 0

	count := project.GenerateRecurringInstancesAtWithID(now, func() string {
		idCalls++
		return "generated"
	})

	if count != 1 || idCalls != 1 || len(project.Tasks) != 2 {
		t.Fatalf("generation count/id calls/tasks = %d/%d/%d, want 1/1/2", count, idCalls, len(project.Tasks))
	}
	generated := project.Tasks[0]
	if generated.ID != "generated" || generated.ShortID != 8 {
		t.Errorf("generated identity = %q/#%d", generated.ID, generated.ShortID)
	}
	if generated.State() != StateInbox || generated.Title != task.Title || generated.Description != task.Description ||
		generated.Priority != task.Priority {
		t.Errorf("generated core fields = %+v", generated)
	}
	if generated.ParentID == nil || *generated.ParentID != parentID || generated.ParentID == task.ParentID {
		t.Errorf("generated parent = %v; want an independent %q pointer", generated.ParentID, parentID)
	}
	if len(generated.Tags) != 2 || &generated.Tags[0] == &task.Tags[0] {
		t.Errorf("generated tags = %v; want independent copy", generated.Tags)
	}
	if len(generated.Checklist) != 2 || generated.Checklist[0].Done || generated.Checklist[1].Done {
		t.Errorf("generated checklist = %+v; want unchecked copies", generated.Checklist)
	}
	if generated.Recurrence == nil || generated.Recurrence.Interval != 1 || generated.Recurrence.Unit != RecurrenceWeekly {
		t.Errorf("generated recurrence = %+v", generated.Recurrence)
	}
	wantDue := date(2026, time.July, 15)
	if generated.DueDate == nil || !generated.DueDate.Equal(wantDue) {
		t.Errorf("generated due = %v, want %v", generated.DueDate, wantDue)
	}
	if !generated.CreatedAt.Equal(now) || !generated.UpdatedAt.Equal(now) {
		t.Errorf("generated timestamps = %v/%v, want %v", generated.CreatedAt, generated.UpdatedAt, now)
	}

	if len(generated.DependsOn) != 0 ||
		generated.Pinned || generated.PinnedAt != nil || len(generated.Comments) != 0 ||
		len(generated.Decisions) != 0 || len(generated.Attachments) != 0 ||
		generated.SnoozedUntil != nil {
		t.Errorf("generated task copied retired/history fields: %+v", generated)
	}
	if task.Recurrence != nil {
		t.Error("source recurrence was not moved to the generated instance")
	}
	if again := project.GenerateRecurringInstancesAtWithID(now, func() string { return "duplicate" }); again != 0 {
		t.Errorf("second generation = %d, want idempotent zero", again)
	}
}

func TestGenerateRecurringInstancesOnlyForDone(t *testing.T) {
	now := dateAtNoon(2026, time.July, 10)
	for _, state := range []WorkflowState{StateInbox, StateReady, StateInProgress, StateCanceled} {
		task := &Task{
			ID:            string(state),
			WorkflowState: state,
			Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 1}
		if got := project.GenerateRecurringInstancesAt(now); got != 0 {
			t.Errorf("state %q generated %d instances, want zero", state, got)
		}
	}
}
