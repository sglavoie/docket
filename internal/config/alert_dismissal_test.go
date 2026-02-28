package config

import "testing"

func TestAlertDismissalHelpers(t *testing.T) {
	t.Run("DismissAlert adds new entry", func(t *testing.T) {
		cfg := NewConfig()
		cfg.DismissAlert("overdue:t1", "overdue")

		if !cfg.IsAlertDismissed("overdue:t1") {
			t.Error("expected alert to be dismissed")
		}
		if len(cfg.GetAlertDismissals()) != 1 {
			t.Errorf("expected 1 dismissal, got %d", len(cfg.GetAlertDismissals()))
		}
	})

	t.Run("DismissAlert deduplicates on re-dismiss", func(t *testing.T) {
		cfg := NewConfig()
		cfg.DismissAlert("overdue:t1", "overdue")
		cfg.DismissAlert("overdue:t1", "overdue")

		if len(cfg.GetAlertDismissals()) != 1 {
			t.Errorf("expected 1 dismissal after re-dismiss, got %d", len(cfg.GetAlertDismissals()))
		}
	})

	t.Run("IsAlertDismissed returns false for unknown key", func(t *testing.T) {
		cfg := NewConfig()
		if cfg.IsAlertDismissed("nonexistent") {
			t.Error("expected false for unknown key")
		}
	})

	t.Run("ClearAlertDismissals removes all", func(t *testing.T) {
		cfg := NewConfig()
		cfg.DismissAlert("overdue:t1", "overdue")
		cfg.DismissAlert("stale:t2", "stale")
		cfg.ClearAlertDismissals()

		if len(cfg.GetAlertDismissals()) != 0 {
			t.Errorf("expected 0 dismissals after clear, got %d", len(cfg.GetAlertDismissals()))
		}
		if cfg.IsAlertDismissed("overdue:t1") {
			t.Error("expected overdue:t1 to no longer be dismissed")
		}
	})

	t.Run("SetAlertDismissals replaces entire list", func(t *testing.T) {
		cfg := NewConfig()
		cfg.DismissAlert("overdue:t1", "overdue")
		cfg.SetAlertDismissals(nil)

		if len(cfg.GetAlertDismissals()) != 0 {
			t.Errorf("expected empty list after SetAlertDismissals(nil), got %d", len(cfg.GetAlertDismissals()))
		}
	})

	t.Run("UndismissAlert removes existing entry", func(t *testing.T) {
		cfg := NewConfig()
		cfg.DismissAlert("overdue:t1", "overdue")
		cfg.DismissAlert("stale:t2", "stale")

		removed := cfg.UndismissAlert("overdue:t1")
		if !removed {
			t.Error("expected UndismissAlert to return true")
		}
		if cfg.IsAlertDismissed("overdue:t1") {
			t.Error("expected overdue:t1 to no longer be dismissed")
		}
		if !cfg.IsAlertDismissed("stale:t2") {
			t.Error("expected stale:t2 to still be dismissed")
		}
		if len(cfg.GetAlertDismissals()) != 1 {
			t.Errorf("expected 1 dismissal remaining, got %d", len(cfg.GetAlertDismissals()))
		}
	})

	t.Run("UndismissAlert returns false for unknown key", func(t *testing.T) {
		cfg := NewConfig()
		cfg.DismissAlert("overdue:t1", "overdue")
		if cfg.UndismissAlert("nonexistent") {
			t.Error("expected UndismissAlert to return false for unknown key")
		}
		if len(cfg.GetAlertDismissals()) != 1 {
			t.Error("original dismissal should be unaffected")
		}
	})

	t.Run("Multiple dismissals tracked independently", func(t *testing.T) {
		cfg := NewConfig()
		cfg.DismissAlert("overdue:t1", "overdue")
		cfg.DismissAlert("stale:t2", "stale")
		cfg.DismissAlert("daily_review", "daily_review")

		if !cfg.IsAlertDismissed("overdue:t1") || !cfg.IsAlertDismissed("stale:t2") || !cfg.IsAlertDismissed("daily_review") {
			t.Error("expected all three to be dismissed")
		}
		if len(cfg.GetAlertDismissals()) != 3 {
			t.Errorf("expected 3 dismissals, got %d", len(cfg.GetAlertDismissals()))
		}
	})
}
