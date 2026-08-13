package models

import (
	"testing"
	"time"
)

func TestPinUnpin(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	task.Pin()
	if !task.Pinned {
		t.Error("expected Pinned to be true")
	}
	if task.PinnedAt == nil {
		t.Error("expected PinnedAt to be set")
	}

	task.Unpin()
	if task.Pinned {
		t.Error("expected Pinned to be false")
	}
	if task.PinnedAt != nil {
		t.Error("expected PinnedAt to be nil")
	}
}

func TestApplyPinnedSetsClearsAndPreservesPinnedAt(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	first := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	later := first.Add(48 * time.Hour)

	if !task.ApplyPinned(true, first) {
		t.Fatal("pinning an unpinned task should report a change")
	}
	if !task.Pinned || task.PinnedAt == nil || !task.PinnedAt.Equal(first) {
		t.Fatalf("pinned = %v, pinnedAt = %v, want true at %v", task.Pinned, task.PinnedAt, first)
	}

	task.UpdatedAt = first
	if task.ApplyPinned(true, later) {
		t.Fatal("re-pinning should report no change")
	}
	if !task.PinnedAt.Equal(first) {
		t.Errorf("pinnedAt = %v, want preserved %v", task.PinnedAt, first)
	}
	if !task.UpdatedAt.Equal(first) {
		t.Errorf("updatedAt = %v, want untouched %v", task.UpdatedAt, first)
	}

	if !task.ApplyPinned(false, later) {
		t.Fatal("unpinning a pinned task should report a change")
	}
	if task.Pinned || task.PinnedAt != nil {
		t.Fatalf("pinned = %v, pinnedAt = %v, want false and nil", task.Pinned, task.PinnedAt)
	}
	if !task.UpdatedAt.Equal(later) {
		t.Errorf("updatedAt = %v, want %v", task.UpdatedAt, later)
	}

	if task.ApplyPinned(false, later.Add(time.Hour)) {
		t.Fatal("unpinning an unpinned task should report no change")
	}
}

func TestTogglePinned(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	task.TogglePinned()
	if !task.Pinned {
		t.Error("expected Pinned to be true after first toggle")
	}

	task.TogglePinned()
	if task.Pinned {
		t.Error("expected Pinned to be false after second toggle")
	}
}
