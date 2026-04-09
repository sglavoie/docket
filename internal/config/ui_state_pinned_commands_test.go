package config

import "testing"

func TestGetPinnedCommands_Empty(t *testing.T) {
	cfg := NewConfig()

	commands := cfg.GetPinnedCommands()

	if commands == nil {
		t.Error("GetPinnedCommands should return empty slice, not nil")
	}
	if len(commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(commands))
	}
}

func TestSetPinnedCommands(t *testing.T) {
	cfg := NewConfig()

	commands := []string{"task-add", "project-switch"}
	cfg.SetPinnedCommands(commands)

	result := cfg.GetPinnedCommands()
	if len(result) != 2 {
		t.Errorf("expected 2 commands, got %d", len(result))
	}
}

func TestIsCommandPinned(t *testing.T) {
	cfg := NewConfig()
	cfg.PinnedCommands = []string{"task-add", "project-list"}

	if !cfg.IsCommandPinned("task-add") {
		t.Error("task-add should be pinned")
	}
	if !cfg.IsCommandPinned("project-list") {
		t.Error("project-list should be pinned")
	}
	if cfg.IsCommandPinned("nonexistent") {
		t.Error("nonexistent should not be pinned")
	}
}

func TestIsCommandPinned_EmptyList(t *testing.T) {
	cfg := NewConfig()

	if cfg.IsCommandPinned("anything") {
		t.Error("nothing should be pinned on empty list")
	}
}

func TestTogglePinnedCommand_Pin(t *testing.T) {
	cfg := NewConfig()

	pinned := cfg.TogglePinnedCommand("task-add")

	if !pinned {
		t.Error("TogglePinnedCommand should return true when pinning")
	}
	if !cfg.IsCommandPinned("task-add") {
		t.Error("task-add should be pinned after toggle")
	}
}

func TestTogglePinnedCommand_Unpin(t *testing.T) {
	cfg := NewConfig()
	cfg.PinnedCommands = []string{"task-add", "project-list"}

	pinned := cfg.TogglePinnedCommand("task-add")

	if pinned {
		t.Error("TogglePinnedCommand should return false when unpinning")
	}
	if cfg.IsCommandPinned("task-add") {
		t.Error("task-add should not be pinned after toggle")
	}
	if !cfg.IsCommandPinned("project-list") {
		t.Error("project-list should still be pinned")
	}
}

func TestTogglePinnedCommand_MultipleToggles(t *testing.T) {
	cfg := NewConfig()

	if !cfg.TogglePinnedCommand("cmd") {
		t.Error("first toggle should pin")
	}
	if cfg.TogglePinnedCommand("cmd") {
		t.Error("second toggle should unpin")
	}
	if !cfg.TogglePinnedCommand("cmd") {
		t.Error("third toggle should pin again")
	}
}
