package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func newActivityTask(t *testing.T) *models.Task {
	t.Helper()
	task, err := models.NewTask("t1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestGetEntriesDefaultNewestFirst(t *testing.T) {
	task := newActivityTask(t)

	// Add entries in chronological order (oldest first via append).
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	task.Activity = []*models.ActivityEntry{
		{Timestamp: base, Field: "created"},
		{Timestamp: base.Add(1 * time.Hour), Field: "status", OldValue: "todo", NewValue: "in_progress"},
		{Timestamp: base.Add(2 * time.Hour), Field: "priority", OldValue: "3", NewValue: "1"},
	}

	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")
	m := NewTaskActivityModel(task, project)
	// Default: sortAsc=false → newest first
	entries := m.getEntries()

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// First entry should be the newest (priority change)
	if entries[0].Field != "priority" {
		t.Errorf("expected newest entry first (priority), got %q", entries[0].Field)
	}
	// Last entry should be the oldest (created)
	if entries[2].Field != "created" {
		t.Errorf("expected oldest entry last (created), got %q", entries[2].Field)
	}
}

func TestGetEntriesAscOldestFirst(t *testing.T) {
	task := newActivityTask(t)

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	task.Activity = []*models.ActivityEntry{
		{Timestamp: base, Field: "created"},
		{Timestamp: base.Add(1 * time.Hour), Field: "status", OldValue: "todo", NewValue: "in_progress"},
		{Timestamp: base.Add(2 * time.Hour), Field: "priority", OldValue: "3", NewValue: "1"},
	}

	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")
	m := NewTaskActivityModel(task, project)
	m.sortAsc = true // oldest first

	entries := m.getEntries()

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// First entry should be the oldest (created)
	if entries[0].Field != "created" {
		t.Errorf("expected oldest entry first (created), got %q", entries[0].Field)
	}
	// Last entry should be the newest (priority change)
	if entries[2].Field != "priority" {
		t.Errorf("expected newest entry last (priority), got %q", entries[2].Field)
	}
}

func TestGetEntriesTabFilter(t *testing.T) {
	task := newActivityTask(t)

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	task.Activity = []*models.ActivityEntry{
		{Timestamp: base, Field: "created"},
		{Timestamp: base.Add(1 * time.Hour), Field: "status", OldValue: "todo", NewValue: "in_progress"},
		{Timestamp: base.Add(2 * time.Hour), Field: "priority", OldValue: "3", NewValue: "1"},
		{Timestamp: base.Add(3 * time.Hour), Field: "comment", NewValue: "a note"},
	}

	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")
	m := NewTaskActivityModel(task, project)

	// Status tab: only "status" and "created" entries
	m.activeTab = activityTabStatus
	entries := m.getEntries()
	if len(entries) != 2 {
		t.Fatalf("status tab: expected 2 entries, got %d", len(entries))
	}

	// Edits tab: excludes status, created, comment
	m.activeTab = activityTabEdits
	entries = m.getEntries()
	if len(entries) != 1 {
		t.Fatalf("edits tab: expected 1 entry, got %d", len(entries))
	}
	if entries[0].Field != "priority" {
		t.Errorf("edits tab: expected priority entry, got %q", entries[0].Field)
	}

	// Comments tab: only comment entries
	m.activeTab = activityTabComments
	entries = m.getEntries()
	if len(entries) != 1 {
		t.Fatalf("comments tab: expected 1 entry, got %d", len(entries))
	}
	if entries[0].Field != "comment" {
		t.Errorf("comments tab: expected comment entry, got %q", entries[0].Field)
	}
}

func TestGetEntriesEmptyActivity(t *testing.T) {
	task := newActivityTask(t)
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")
	m := NewTaskActivityModel(task, project)

	entries := m.getEntries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty activity, got %d", len(entries))
	}

	m.sortAsc = true
	entries = m.getEntries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty activity (asc), got %d", len(entries))
	}
}
