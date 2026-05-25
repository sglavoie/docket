package models

import (
	"testing"
	"time"
)

func TestRecordActivity(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	if len(task.Activity) != 0 {
		t.Fatalf("expected 0 activity entries initially, got %d", len(task.Activity))
	}

	task.RecordActivity("status", "todo", "in_progress")

	if len(task.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(task.Activity))
	}

	entry := task.Activity[0]
	if entry.Field != "status" {
		t.Errorf("expected field 'status', got %q", entry.Field)
	}
	if entry.OldValue != "todo" {
		t.Errorf("expected old value 'todo', got %q", entry.OldValue)
	}
	if entry.NewValue != "in_progress" {
		t.Errorf("expected new value 'in_progress', got %q", entry.NewValue)
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	// Add another
	task.RecordActivity("priority", "3", "1")
	if len(task.Activity) != 2 {
		t.Errorf("expected 2 entries, got %d", len(task.Activity))
	}
}

func TestRecordCreated(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.RecordCreated()

	if len(task.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(task.Activity))
	}

	entry := task.Activity[0]
	if entry.Field != "created" {
		t.Errorf("expected field 'created', got %q", entry.Field)
	}
	if entry.OldValue != "" || entry.NewValue != "" {
		t.Error("expected empty old/new values for created entry")
	}
	if !entry.Timestamp.Equal(task.CreatedAt) {
		t.Error("expected timestamp to match CreatedAt")
	}
}

func TestPruneActivity(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Add 10 entries
	for range 10 {
		task.Activity = append(task.Activity, &ActivityEntry{
			Timestamp: time.Now(),
			Field:     "status",
			OldValue:  "old",
			NewValue:  "new",
		})
	}

	// Prune to 5 — should keep the newest 5
	task.PruneActivity(5)
	if len(task.Activity) != 5 {
		t.Errorf("expected 5 entries after prune, got %d", len(task.Activity))
	}

	// Prune with limit larger than count — no-op
	task.PruneActivity(100)
	if len(task.Activity) != 5 {
		t.Errorf("expected 5 entries (no-op), got %d", len(task.Activity))
	}

	// Prune with 0 — no-op
	task.PruneActivity(0)
	if len(task.Activity) != 5 {
		t.Errorf("expected 5 entries (zero limit no-op), got %d", len(task.Activity))
	}

	// Prune with negative — no-op
	task.PruneActivity(-1)
	if len(task.Activity) != 5 {
		t.Errorf("expected 5 entries (negative limit no-op), got %d", len(task.Activity))
	}

	// Prune to 1
	task.PruneActivity(1)
	if len(task.Activity) != 1 {
		t.Errorf("expected 1 entry, got %d", len(task.Activity))
	}
}

func TestPruneActivityPreservesCreatedEntry(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.RecordCreated()
	for i := range 50 {
		task.Activity = append(task.Activity, &ActivityEntry{
			Timestamp: time.Now(),
			Field:     "status",
			OldValue:  "old",
			NewValue:  string(rune('a' + i%26)),
		})
	}

	if len(task.Activity) != 51 {
		t.Fatalf("setup: expected 51 entries, got %d", len(task.Activity))
	}

	task.PruneActivity(5)

	if len(task.Activity) != 5 {
		t.Fatalf("expected 5 entries after prune, got %d", len(task.Activity))
	}
	if task.Activity[0].Field != "created" {
		t.Errorf("expected first entry to be 'created', got %q", task.Activity[0].Field)
	}
	for i := 1; i < len(task.Activity); i++ {
		if task.Activity[i].Field == "created" {
			t.Errorf("created entry should appear only once, found duplicate at index %d", i)
		}
	}

	// Last entry should be the most recent ("y" for i=49 -> 49%26 = 23 -> 'x')
	wantLast := string(rune('a' + 49%26))
	if task.Activity[len(task.Activity)-1].NewValue != wantLast {
		t.Errorf("expected last entry NewValue %q, got %q", wantLast, task.Activity[len(task.Activity)-1].NewValue)
	}
}

func TestPruneActivityPreservesCreatedWhenMaxIsOne(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.RecordCreated()
	for range 5 {
		task.Activity = append(task.Activity, &ActivityEntry{
			Timestamp: time.Now(),
			Field:     "status",
		})
	}

	task.PruneActivity(1)
	if len(task.Activity) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(task.Activity))
	}
	if task.Activity[0].Field != "created" {
		t.Errorf("expected created entry, got %q", task.Activity[0].Field)
	}
}

func TestPruneActivityNoCreatedEntry(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	for i := range 10 {
		task.Activity = append(task.Activity, &ActivityEntry{
			Timestamp: time.Now(),
			Field:     "status",
			NewValue:  string(rune('a' + i)),
		})
	}

	task.PruneActivity(3)
	if len(task.Activity) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(task.Activity))
	}
	if task.Activity[0].NewValue != "h" || task.Activity[2].NewValue != "j" {
		t.Errorf("expected newest 3 entries h/i/j, got %q/%q/%q",
			task.Activity[0].NewValue, task.Activity[1].NewValue, task.Activity[2].NewValue)
	}
}

func TestPruneActivityKeepsNewest(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Add entries with identifiable values
	for i := range 5 {
		task.Activity = append(task.Activity, &ActivityEntry{
			Timestamp: time.Now(),
			Field:     "priority",
			OldValue:  "",
			NewValue:  string(rune('a' + i)), // "a", "b", "c", "d", "e"
		})
	}

	task.PruneActivity(2)

	// Should keep the last 2: "d" and "e"
	if task.Activity[0].NewValue != "d" {
		t.Errorf("expected 'd', got %q", task.Activity[0].NewValue)
	}
	if task.Activity[1].NewValue != "e" {
		t.Errorf("expected 'e', got %q", task.Activity[1].NewValue)
	}
}
