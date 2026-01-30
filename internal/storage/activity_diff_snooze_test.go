package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestDiffTask_SnoozeAdded(t *testing.T) {
	old := &models.Task{ID: "1"}
	snoozeDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	newTask := &models.Task{ID: "1", SnoozedUntil: &snoozeDate}

	diffTask(old, newTask)

	found := false
	for _, entry := range newTask.Activity {
		if entry.Field == "snoozed" {
			found = true
			if entry.OldValue != "none" {
				t.Errorf("OldValue = %s, want none", entry.OldValue)
			}
			if entry.NewValue != "2025-06-15" {
				t.Errorf("NewValue = %s, want 2025-06-15", entry.NewValue)
			}
		}
	}
	if !found {
		t.Error("Expected snoozed activity entry")
	}
}

func TestDiffTask_SnoozeCleared(t *testing.T) {
	snoozeDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	old := &models.Task{ID: "1", SnoozedUntil: &snoozeDate}
	newTask := &models.Task{ID: "1"}

	diffTask(old, newTask)

	found := false
	for _, entry := range newTask.Activity {
		if entry.Field == "snoozed" {
			found = true
			if entry.OldValue != "2025-06-15" {
				t.Errorf("OldValue = %s, want 2025-06-15", entry.OldValue)
			}
			if entry.NewValue != "none" {
				t.Errorf("NewValue = %s, want none", entry.NewValue)
			}
		}
	}
	if !found {
		t.Error("Expected snoozed activity entry")
	}
}

func TestDiffTask_SnoozeUnchanged(t *testing.T) {
	snoozeDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	old := &models.Task{ID: "1", SnoozedUntil: &snoozeDate}
	sameDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	newTask := &models.Task{ID: "1", SnoozedUntil: &sameDate}

	diffTask(old, newTask)

	for _, entry := range newTask.Activity {
		if entry.Field == "snoozed" {
			t.Error("Should not have snoozed activity entry when unchanged")
		}
	}
}
