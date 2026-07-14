package config

import "testing"

func TestRecentTaskVisitHelpers(t *testing.T) {
	t.Run("SetRecentTaskVisits replaces entire list", func(t *testing.T) {
		cfg := NewMachineState()
		cfg.SetRecentTaskVisits([]RecentTaskVisit{
			{TaskID: "t1", ProjectID: "p1"},
			{TaskID: "t2", ProjectID: "p1"},
		})

		v := cfg.GetRecentTaskVisits()
		if len(v) != 2 {
			t.Fatalf("expected 2 items, got %d", len(v))
		}
		if v[0].TaskID != "t1" || v[1].TaskID != "t2" {
			t.Errorf("unexpected visits: %v", v)
		}
	})

	t.Run("SetRecentTaskVisits overwrites previous list", func(t *testing.T) {
		cfg := NewMachineState()
		cfg.SetRecentTaskVisits([]RecentTaskVisit{
			{TaskID: "t1", ProjectID: "p1"},
		})
		cfg.SetRecentTaskVisits([]RecentTaskVisit{
			{TaskID: "t3", ProjectID: "p2"},
		})

		v := cfg.GetRecentTaskVisits()
		if len(v) != 1 || v[0].TaskID != "t3" {
			t.Errorf("expected replaced list with t3, got %v", v)
		}
	})
}

// TestGetMaxRecentTaskVisits_ZeroReturnsDefault verifies that a stored value of
// 0 (which is below the minimum valid value of 1) returns the default instead.
func TestGetMaxRecentTaskVisits_ZeroReturnsDefault(t *testing.T) {
	cfg := NewSettings()
	zero := 0
	cfg.MaxRecentTaskVisits = &zero

	got := cfg.GetMaxRecentTaskVisits()
	if got != DefaultMaxRecentTaskVisits {
		t.Errorf("expected default %d for MaxRecentTaskVisits=0, got %d", DefaultMaxRecentTaskVisits, got)
	}
}

// TestGetMaxRecentTaskVisits_NilReturnsDefault verifies nil returns the default.
func TestGetMaxRecentTaskVisits_NilReturnsDefault(t *testing.T) {
	cfg := NewSettings()
	cfg.MaxRecentTaskVisits = nil

	got := cfg.GetMaxRecentTaskVisits()
	if got != DefaultMaxRecentTaskVisits {
		t.Errorf("expected default %d for nil MaxRecentTaskVisits, got %d", DefaultMaxRecentTaskVisits, got)
	}
}

// TestGetMaxRecentTaskVisits_ValidValuePassesThrough ensures normal values are not modified.
func TestGetMaxRecentTaskVisits_ValidValuePassesThrough(t *testing.T) {
	cfg := NewSettings()
	v := 25
	cfg.MaxRecentTaskVisits = &v

	got := cfg.GetMaxRecentTaskVisits()
	if got != 25 {
		t.Errorf("expected 25, got %d", got)
	}
}

// TestGetMaxRecentTaskVisits_AboveUpperBoundClamped guarantees that values
// exceeding the [1, 50] range enforced by the loader and settings validator
// are clamped on read. Without this clamp, an in-memory Config that bypassed
// Load() (or a programmatic caller) could yield a runaway slice cap when used
// by the quick-switch trimming logic in the TUI.
func TestGetMaxRecentTaskVisits_AboveUpperBoundClamped(t *testing.T) {
	cfg := NewSettings()
	v := 9999
	cfg.MaxRecentTaskVisits = &v

	got := cfg.GetMaxRecentTaskVisits()
	if got != MaxRecentTaskVisitsUpperBound {
		t.Errorf("expected %d for out-of-range MaxRecentTaskVisits=%d, got %d",
			MaxRecentTaskVisitsUpperBound, v, got)
	}
}

// TestGetMaxRecentTaskVisits_AtUpperBoundPassesThrough ensures values exactly
// at the upper bound are not modified.
func TestGetMaxRecentTaskVisits_AtUpperBoundPassesThrough(t *testing.T) {
	cfg := NewSettings()
	v := MaxRecentTaskVisitsUpperBound
	cfg.MaxRecentTaskVisits = &v

	got := cfg.GetMaxRecentTaskVisits()
	if got != MaxRecentTaskVisitsUpperBound {
		t.Errorf("expected %d, got %d", MaxRecentTaskVisitsUpperBound, got)
	}
}
