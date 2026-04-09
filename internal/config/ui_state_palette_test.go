package config

import "testing"

func TestGetRecentPaletteItems_Empty(t *testing.T) {
	cfg := NewConfig()

	items := cfg.GetRecentPaletteItems()

	if items == nil {
		t.Error("GetRecentPaletteItems should return empty slice, not nil")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestSetRecentPaletteItems(t *testing.T) {
	cfg := NewConfig()

	items := []RecentPaletteItem{
		{Category: "Commands", ID: "task-add"},
		{Category: "Projects", ID: "my-project"},
	}
	cfg.SetRecentPaletteItems(items)

	result := cfg.GetRecentPaletteItems()
	if len(result) != 2 {
		t.Errorf("expected 2 items, got %d", len(result))
	}
	if result[0].Category != "Commands" || result[0].ID != "task-add" {
		t.Error("first item mismatch")
	}
}

func TestGetLastPaletteCategory(t *testing.T) {
	cfg := NewConfig()

	if category := cfg.GetLastPaletteCategory(); category != "" {
		t.Errorf("expected empty string, got '%s'", category)
	}

	cfg.SetLastPaletteCategory(":task ")
	if category := cfg.GetLastPaletteCategory(); category != ":task " {
		t.Errorf("expected ':task ', got '%s'", category)
	}
}

func TestSetLastPaletteCategory(t *testing.T) {
	cfg := NewConfig()

	cfg.SetLastPaletteCategory(":project ")
	if cfg.LastPaletteCategory != ":project " {
		t.Errorf("expected ':project ', got '%s'", cfg.LastPaletteCategory)
	}
}

func TestGetPaletteSearchHistory_Empty(t *testing.T) {
	cfg := NewConfig()

	history := cfg.GetPaletteSearchHistory()

	if history == nil {
		t.Error("GetPaletteSearchHistory should return empty slice, not nil")
	}
	if len(history) != 0 {
		t.Errorf("expected 0 items, got %d", len(history))
	}
}

func TestSetPaletteSearchHistory(t *testing.T) {
	cfg := NewConfig()

	history := []string{"task add", "project list", "help"}
	cfg.SetPaletteSearchHistory(history)

	result := cfg.GetPaletteSearchHistory()
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
	if result[0] != "task add" {
		t.Errorf("expected 'task add', got '%s'", result[0])
	}
}
