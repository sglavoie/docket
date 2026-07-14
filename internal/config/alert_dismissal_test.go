package config

import (
	"testing"
	"time"
)

func TestAlertDismissalHelpers(t *testing.T) {
	t.Run("DismissAlert adds new entry", func(t *testing.T) {
		cfg := NewPlanningState()
		cfg.DismissAlert("overdue:t1", "overdue")

		if !cfg.IsAlertDismissed("overdue:t1") {
			t.Error("expected alert to be dismissed")
		}
		if len(cfg.GetAlertDismissals()) != 1 {
			t.Errorf("expected 1 dismissal, got %d", len(cfg.GetAlertDismissals()))
		}
	})

	t.Run("DismissAlert deduplicates on re-dismiss", func(t *testing.T) {
		cfg := NewPlanningState()
		cfg.DismissAlert("overdue:t1", "overdue")
		cfg.DismissAlert("overdue:t1", "overdue")

		if len(cfg.GetAlertDismissals()) != 1 {
			t.Errorf("expected 1 dismissal after re-dismiss, got %d", len(cfg.GetAlertDismissals()))
		}
	})

	t.Run("IsAlertDismissed returns false for unknown key", func(t *testing.T) {
		cfg := NewPlanningState()
		if cfg.IsAlertDismissed("nonexistent") {
			t.Error("expected false for unknown key")
		}
	})

	t.Run("ClearAlertDismissals removes all", func(t *testing.T) {
		cfg := NewPlanningState()
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
		cfg := NewPlanningState()
		cfg.DismissAlert("overdue:t1", "overdue")
		cfg.SetAlertDismissals(nil)

		if len(cfg.GetAlertDismissals()) != 0 {
			t.Errorf("expected empty list after SetAlertDismissals(nil), got %d", len(cfg.GetAlertDismissals()))
		}
	})

	t.Run("UndismissAlert removes existing entry", func(t *testing.T) {
		cfg := NewPlanningState()
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
		cfg := NewPlanningState()
		cfg.DismissAlert("overdue:t1", "overdue")
		if cfg.UndismissAlert("nonexistent") {
			t.Error("expected UndismissAlert to return false for unknown key")
		}
		if len(cfg.GetAlertDismissals()) != 1 {
			t.Error("original dismissal should be unaffected")
		}
	})

	t.Run("Multiple dismissals tracked independently", func(t *testing.T) {
		cfg := NewPlanningState()
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

	t.Run("PruneExpiredAlertDismissals removes old entries", func(t *testing.T) {
		cfg := NewPlanningState()
		// Add a recent dismissal
		cfg.DismissAlert("recent:t1", "overdue")
		// Add an old dismissal (31 days ago)
		cfg.AlertDismissals = append(cfg.AlertDismissals, AlertDismissal{
			Key:         "old:t2",
			Category:    "stale",
			DismissedAt: time.Now().Add(-31 * 24 * time.Hour),
		})

		pruned := cfg.PruneExpiredAlertDismissals()
		if !pruned {
			t.Error("expected pruning to return true")
		}
		if len(cfg.AlertDismissals) != 1 {
			t.Errorf("expected 1 dismissal after prune, got %d", len(cfg.AlertDismissals))
		}
		if !cfg.IsAlertDismissed("recent:t1") {
			t.Error("recent dismissal should be kept")
		}
		if cfg.IsAlertDismissed("old:t2") {
			t.Error("old dismissal should be pruned")
		}
	})

	t.Run("PruneExpiredAlertDismissals noop when nothing expired", func(t *testing.T) {
		cfg := NewPlanningState()
		cfg.DismissAlert("recent:t1", "overdue")

		pruned := cfg.PruneExpiredAlertDismissals()
		if pruned {
			t.Error("expected pruning to return false when nothing expired")
		}
		if len(cfg.AlertDismissals) != 1 {
			t.Errorf("expected 1 dismissal unchanged, got %d", len(cfg.AlertDismissals))
		}
	})

	t.Run("PruneExpiredAlertDismissals noop on empty list", func(t *testing.T) {
		cfg := NewPlanningState()
		pruned := cfg.PruneExpiredAlertDismissals()
		if pruned {
			t.Error("expected pruning to return false on empty list")
		}
	})
}
