package models

import "testing"

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

func TestActivateDeactivate(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	task.Activate()
	if !task.Active {
		t.Error("expected Active to be true")
	}

	task.Deactivate()
	if task.Active {
		t.Error("expected Active to be false")
	}
}

func TestToggleActive(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	task.ToggleActive()
	if !task.Active {
		t.Error("expected Active to be true after first toggle")
	}

	task.ToggleActive()
	if task.Active {
		t.Error("expected Active to be false after second toggle")
	}
}

func TestDimUndim(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Dim()
	if !task.Dimmed {
		t.Error("expected Dimmed to be true after Dim")
	}
	task.Undim()
	if task.Dimmed {
		t.Error("expected Dimmed to be false after Undim")
	}
}

func TestToggleDimmed(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.ToggleDimmed()
	if !task.Dimmed {
		t.Error("expected Dimmed to be true after first ToggleDimmed")
	}
	task.ToggleDimmed()
	if task.Dimmed {
		t.Error("expected Dimmed to be false after second ToggleDimmed")
	}
}

func TestDimClearsActive(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Active = true
	task.Dim()
	if !task.Dimmed {
		t.Error("expected Dimmed to be true after Dim")
	}
	if task.Active {
		t.Error("expected Active to be false after Dim")
	}
}

func TestActivateClearsDimmed(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Dimmed = true
	task.Activate()
	if !task.Active {
		t.Error("expected Active to be true after Activate")
	}
	if task.Dimmed {
		t.Error("expected Dimmed to be false after Activate")
	}
}
