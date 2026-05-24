package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestDetectChangesReportsRecurrenceChange(t *testing.T) {
	original := &models.Task{ID: "t1", Recurrence: nil}
	modified := &models.Task{ID: "t1", Recurrence: &models.RecurrencePattern{
		Interval: 1, Unit: models.RecurrenceDaily,
	}}

	changes := detectChanges(original, modified)

	var found bool
	for _, c := range changes {
		if c.Field == "recurrence" {
			found = true
			if c.OldValue != "(none)" {
				t.Errorf("OldValue: want (none), got %q", c.OldValue)
			}
			if c.NewValue == "" || c.NewValue == "(none)" {
				t.Errorf("NewValue should describe the pattern, got %q", c.NewValue)
			}
		}
	}
	if !found {
		t.Errorf("expected a 'recurrence' change entry, got %+v", changes)
	}
}

func TestDetectChangesReportsDimmedChange(t *testing.T) {
	original := &models.Task{ID: "t1", Dimmed: true}
	modified := &models.Task{ID: "t1", Dimmed: false}

	changes := detectChanges(original, modified)

	var found bool
	for _, c := range changes {
		if c.Field == "dimmed" {
			found = true
			if c.OldValue != "true" || c.NewValue != "false" {
				t.Errorf("dimmed change: got %s -> %s", c.OldValue, c.NewValue)
			}
		}
	}
	if !found {
		t.Errorf("expected a 'dimmed' change entry, got %+v", changes)
	}
}

func TestDeepCopyTaskForDryRunCopiesDimmed(t *testing.T) {
	src := &models.Task{ID: "t1", Dimmed: true}
	cp := deepCopyTaskForDryRun(src)
	if !cp.Dimmed {
		t.Error("expected copy to preserve Dimmed=true")
	}
}
