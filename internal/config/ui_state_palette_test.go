package config

import "testing"

func TestGetRecentPaletteItems_Empty(t *testing.T) {
	cfg := NewMachineState()

	items := cfg.GetRecentPaletteItems()

	if items == nil {
		t.Error("GetRecentPaletteItems should return empty slice, not nil")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestSetRecentPaletteItems(t *testing.T) {
	cfg := NewMachineState()

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
