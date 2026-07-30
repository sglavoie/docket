package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTrip_KitchenSink(t *testing.T) { //nolint:gocyclo
	now := time.Now()
	created := time.Date(2026, 1, 10, 8, 0, 0, 0, time.Local)
	updated := time.Date(2026, 1, 15, 12, 30, 0, 0, time.Local)
	completed := time.Date(2026, 1, 20, 9, 15, 0, 0, time.Local)
	due := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)
	snoozed := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	pinnedAt := time.Date(2026, 1, 12, 14, 30, 45, 0, time.Local)

	task := &models.Task{
		ID:            "kitchen-sink-1",
		WorkflowState: models.StateDone,
		Priority:      1,
		Tags:          []string{"backend", "urgent", "p0"},
		CreatedAt:     created,
		UpdatedAt:     updated,
		CompletedAt:   &completed,
		DueDate:       &due,
		DependsOn:     []string{"dep-1", "dep-2", "dep-3"},
		Pinned:        true,
		PinnedAt:      &pinnedAt,
		SnoozedUntil:  &snoozed,
		Recurrence: &models.RecurrencePattern{
			Interval: 2,
			Unit:     models.RecurrenceWeekly,
		},
		Attachments: []models.Attachment{
			{
				Filename:     "notes.txt",
				OriginalPath: "/tmp/notes.txt",
				AddedAt:      now,
			},
			{
				Filename:     "https://github.com/issue/1",
				IsURL:        true,
				OriginalPath: "https://github.com/issue/1",
				AddedAt:      now,
			},
		},
		Checklist: []models.ChecklistItem{
			{Text: "Step one", Done: true},
			{Text: "Step two", Done: false},
		},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.ID != task.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, task.ID)
	}
	if parsed.State() != task.State() {
		t.Errorf("State = %q, want %q", parsed.State(), task.State())
	}
	if parsed.Priority != task.Priority {
		t.Errorf("Priority = %d, want %d", parsed.Priority, task.Priority)
	}
	if !stringSlicesEqual(parsed.Tags, task.Tags) {
		t.Errorf("Tags = %v, want %v", parsed.Tags, task.Tags)
	}
	// v2 collapses the legacy lifecycle fields into `state`; a done task
	// reconstitutes to state=done with its canonical Completed section shim
	// (the original task carried status=done, section="").
	if parsed.State() != models.StateDone {
		t.Errorf("State = %q, want %q", parsed.State(), models.StateDone)
	}
	if parsed.State() != models.StateDone {
		t.Errorf("Section = %q, want %q (canonical done shim)", parsed.State(), models.StateDone)
	}
	if parsed.CreatedAt.Format("2006-01-02 15:04") != created.Format("2006-01-02 15:04") {
		t.Errorf("CreatedAt = %v, want %v", parsed.CreatedAt, created)
	}
	if parsed.UpdatedAt.Format("2006-01-02 15:04") != updated.Format("2006-01-02 15:04") {
		t.Errorf("UpdatedAt = %v, want %v", parsed.UpdatedAt, updated)
	}
	if parsed.CompletedAt == nil {
		t.Fatal("CompletedAt is nil")
	}
	if parsed.CompletedAt.Format("2006-01-02 15:04") != completed.Format("2006-01-02 15:04") {
		t.Errorf("CompletedAt = %v, want %v", *parsed.CompletedAt, completed)
	}
	if parsed.DueDate == nil {
		t.Fatal("DueDate is nil")
	}
	if parsed.DueDate.Format("2006-01-02") != due.Format("2006-01-02") {
		t.Errorf("DueDate = %v, want %v", *parsed.DueDate, due)
	}
	if !stringSlicesEqual(parsed.DependsOn, task.DependsOn) {
		t.Errorf("DependsOn = %v, want %v", parsed.DependsOn, task.DependsOn)
	}
	if !parsed.Pinned {
		t.Errorf("Pinned = false, want true")
	}
	if parsed.PinnedAt == nil {
		t.Fatal("PinnedAt is nil")
	}
	if parsed.PinnedAt.Format("2006-01-02 15:04:05") != pinnedAt.Format("2006-01-02 15:04:05") {
		t.Errorf("PinnedAt = %q, want %q", parsed.PinnedAt.Format("2006-01-02 15:04:05"), pinnedAt.Format("2006-01-02 15:04:05"))
	}
	if parsed.SnoozedUntil == nil {
		t.Fatal("SnoozedUntil is nil")
	}
	if parsed.SnoozedUntil.Format("2006-01-02") != snoozed.Format("2006-01-02") {
		t.Errorf("SnoozedUntil = %q, want %q", parsed.SnoozedUntil.Format("2006-01-02"), snoozed.Format("2006-01-02"))
	}
	if parsed.Recurrence == nil {
		t.Fatal("Recurrence is nil")
	}
	if !recurrenceEqual(parsed.Recurrence, task.Recurrence) {
		t.Errorf("Recurrence = %+v, want %+v", parsed.Recurrence, task.Recurrence)
	}
	if len(parsed.Attachments) != 2 {
		t.Fatalf("got %d attachments, want 2", len(parsed.Attachments))
	}
	if parsed.Attachments[0].Filename != "notes.txt" {
		t.Errorf("att[0].Filename = %q, want %q", parsed.Attachments[0].Filename, "notes.txt")
	}
	if parsed.Attachments[0].OriginalPath != "/tmp/notes.txt" {
		t.Errorf("att[0].OriginalPath = %q, want %q", parsed.Attachments[0].OriginalPath, "/tmp/notes.txt")
	}
	if !parsed.Attachments[1].IsURL {
		t.Errorf("att[1].IsURL = false, want true")
	}
	if len(parsed.Checklist) != 2 {
		t.Fatalf("got %d checklist items, want 2", len(parsed.Checklist))
	}
	if parsed.Checklist[0].Text != "Step one" || !parsed.Checklist[0].Done {
		t.Errorf("checklist[0] = {%q, %v}, want {%q, true}", parsed.Checklist[0].Text, parsed.Checklist[0].Done, "Step one")
	}
	if parsed.Checklist[1].Text != "Step two" || parsed.Checklist[1].Done {
		t.Errorf("checklist[1] = {%q, %v}, want {%q, false}", parsed.Checklist[1].Text, parsed.Checklist[1].Done, "Step two")
	}
}
