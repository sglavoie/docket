package config

import (
	"testing"
	"time"
)

// TestPlanningAlertDismissalRoundTrip confirms the planning-domain dismissal
// accessors mirror the config.Config facade behavior for the alert engine.
func TestPlanningAlertDismissalRoundTrip(t *testing.T) {
	p := NewPlanningState()

	if p.IsAlertDismissed("k1") {
		t.Fatal("fresh planning should have no dismissals")
	}
	p.DismissAlert("k1", "overdue")
	if !p.IsAlertDismissed("k1") {
		t.Fatal("DismissAlert did not record the dismissal")
	}
	// A duplicate dismiss of a still-live key is a no-op (single entry).
	p.DismissAlert("k1", "overdue")
	if got := len(p.GetAlertDismissals()); got != 1 {
		t.Fatalf("dismissals = %d, want 1 after duplicate dismiss", got)
	}

	if !p.UndismissAlert("k1") {
		t.Fatal("UndismissAlert should report removal")
	}
	if p.IsAlertDismissed("k1") {
		t.Fatal("UndismissAlert did not clear the dismissal")
	}
}

// TestPlanningPruneExpiredAlertDismissals confirms only expired dismissals are pruned.
func TestPlanningPruneExpiredAlertDismissals(t *testing.T) {
	p := NewPlanningState()
	p.SetAlertDismissals([]AlertDismissal{
		{Key: "fresh", DismissedAt: time.Now()},
		{Key: "stale", DismissedAt: time.Now().Add(-alertDismissalMaxAge - time.Hour)},
	})
	if !p.PruneExpiredAlertDismissals() {
		t.Fatal("prune should report a change")
	}
	if len(p.GetAlertDismissals()) != 1 || p.GetAlertDismissals()[0].Key != "fresh" {
		t.Fatalf("prune kept the wrong set: %+v", p.GetAlertDismissals())
	}
	if p.PruneExpiredAlertDismissals() {
		t.Fatal("second prune should report no change")
	}
}

// TestPlanningReviewCompletionRoundTrip confirms the review completion accessors
// live on the planning domain.
func TestPlanningReviewCompletionRoundTrip(t *testing.T) {
	p := NewPlanningState()
	if p.GetLastDailyReviewAt() != nil || p.GetLastWeeklyReviewAt() != nil {
		t.Fatal("review timestamps should default to nil")
	}
	now := time.Now()
	p.SetLastDailyReviewAt(now)
	p.SetLastWeeklyReviewAt(now)
	if p.GetLastDailyReviewAt() == nil || p.GetLastWeeklyReviewAt() == nil {
		t.Fatal("review timestamps not set")
	}
	p.ClearLastDailyReviewAt()
	p.ClearLastWeeklyReviewAt()
	if p.GetLastDailyReviewAt() != nil || p.GetLastWeeklyReviewAt() != nil {
		t.Fatal("review timestamps not cleared")
	}
}

// TestPlanningSavedFilterAccessors confirms the saved-filter accessors on the
// planning domain mirror the config.Config facade: upsert-by-ID identity,
// name/ID lookup and deletion.
func TestPlanningSavedFilterAccessors(t *testing.T) {
	p := NewPlanningState()

	if got := p.GetSavedFilters(); got == nil || len(got) != 0 {
		t.Fatalf("fresh planning saved filters = %v, want empty non-nil", got)
	}

	p.SaveFilter(SavedFilter{ID: "a", Name: "Alpha", Mode: "snapshot"})
	p.SaveFilter(SavedFilter{ID: "b", Name: "Beta", Mode: "criteria"})
	if len(p.GetSavedFilters()) != 2 {
		t.Fatalf("saved filters = %d, want 2", len(p.GetSavedFilters()))
	}

	// Upsert by ID replaces in place rather than appending.
	p.SaveFilter(SavedFilter{ID: "a", Name: "Alpha Renamed", Mode: "snapshot"})
	if len(p.GetSavedFilters()) != 2 {
		t.Fatalf("upsert changed count to %d, want 2", len(p.GetSavedFilters()))
	}
	if sf := p.GetSavedFilter("a"); sf == nil || sf.Name != "Alpha Renamed" {
		t.Fatalf("GetSavedFilter(a) = %v, want Alpha Renamed", sf)
	}

	// Name lookup is case-insensitive.
	if sf := p.GetSavedFilterByName("beta"); sf == nil || sf.ID != "b" {
		t.Fatalf("GetSavedFilterByName(beta) = %v, want id b", sf)
	}

	if !p.DeleteSavedFilter("a") {
		t.Fatal("DeleteSavedFilter(a) should report removal")
	}
	if p.GetSavedFilter("a") != nil {
		t.Fatal("filter a should be gone after delete")
	}
	if p.DeleteSavedFilter("missing") {
		t.Fatal("deleting an unknown filter should report false")
	}
}

// TestDomainOwnershipSeparation confirms alert policy and review bookkeeping
// remain in distinct explicit domains.
func TestDomainOwnershipSeparation(t *testing.T) {
	enabled := false
	settings := NewSettings()
	settings.AlertOverdueEnabled = &enabled
	when := time.Now()
	planning := NewPlanningState()
	planning.LastDailyReviewAt = &when
	planning.AlertDismissals = []AlertDismissal{{Key: "k", DismissedAt: when}}

	if settings.AlertOverdueEnabled == nil || *settings.AlertOverdueEnabled {
		t.Error("alert policy should be owned by the Settings domain")
	}
	if planning.GetLastDailyReviewAt() == nil {
		t.Error("review completion should be owned by the Planning domain")
	}
	if !planning.IsAlertDismissed("k") {
		t.Error("alert dismissals should be owned by the Planning domain")
	}
	if settings.GetAlertDueSoonDays() != DefaultAlertDueSoonDays {
		t.Errorf("Settings.GetAlertDueSoonDays = %d, want default %d",
			settings.GetAlertDueSoonDays(), DefaultAlertDueSoonDays)
	}
}
