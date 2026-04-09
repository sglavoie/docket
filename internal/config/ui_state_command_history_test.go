package config

import "testing"

func TestGetCommandHistory_Empty(t *testing.T) {
	cfg := NewConfig()

	history := cfg.GetCommandHistory()

	if history == nil {
		t.Error("GetCommandHistory should return empty slice, not nil")
	}
	if len(history) != 0 {
		t.Errorf("expected 0 items, got %d", len(history))
	}
}

func TestAddCommandToHistory(t *testing.T) {
	cfg := NewConfig()

	cfg.AddCommandToHistory("task-add")

	history := cfg.GetCommandHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 item, got %d", len(history))
	}
	if history[0] != "task-add" {
		t.Errorf("expected 'task-add', got '%s'", history[0])
	}
}

func TestAddCommandToHistory_MoveToFront(t *testing.T) {
	cfg := NewConfig()

	cfg.AddCommandToHistory("first")
	cfg.AddCommandToHistory("second")
	cfg.AddCommandToHistory("third")
	cfg.AddCommandToHistory("first")

	history := cfg.GetCommandHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 items (no duplicates), got %d", len(history))
	}
	if history[0] != "first" {
		t.Error("'first' should be at the front after re-adding")
	}
	if history[1] != "third" {
		t.Error("'third' should be second")
	}
	if history[2] != "second" {
		t.Error("'second' should be third")
	}
}

func TestAddCommandToHistory_MaxLimit(t *testing.T) {
	cfg := NewConfig()

	for i := range 12 {
		cfg.AddCommandToHistory("cmd" + string(rune('a'+i)))
	}

	history := cfg.GetCommandHistory()
	if len(history) != 10 {
		t.Errorf("expected max 10 items, got %d", len(history))
	}

	if history[0] != "cmdl" {
		t.Errorf("expected 'cmdl' (most recent) at front, got '%s'", history[0])
	}
	if history[9] != "cmdc" {
		t.Errorf("expected 'cmdc' at back, got '%s'", history[9])
	}
}

func TestAddCommandToHistory_Deduplication(t *testing.T) {
	cfg := NewConfig()

	cfg.AddCommandToHistory("cmd")
	cfg.AddCommandToHistory("cmd")
	cfg.AddCommandToHistory("cmd")

	history := cfg.GetCommandHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 item (deduplicated), got %d", len(history))
	}
}

func TestAddCommandToHistory_MaxWithDuplicates(t *testing.T) {
	cfg := NewConfig()

	for i := range 10 {
		cfg.AddCommandToHistory("cmd" + string(rune('a'+i)))
	}

	cfg.AddCommandToHistory("cmda")

	history := cfg.GetCommandHistory()
	if len(history) != 10 {
		t.Errorf("expected 10 items, got %d", len(history))
	}
	if history[0] != "cmda" {
		t.Error("'cmda' should be at front after re-adding")
	}
}
