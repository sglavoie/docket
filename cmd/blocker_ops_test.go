package cmd

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestWithBlockerUpdateRejectsEmptyPrefix ensures WithBlockerUpdate refuses an
// empty ID prefix. storage.FindBlockerByPrefix uses HasPrefix(id, "") which
// returns true for every blocker, so without this guard an empty arg (for
// example `tt blocker delete ""`) would silently mutate the first blocker.
//
// The function must reject the empty prefix before reaching LoadCurrentProject,
// otherwise the test would need a full project fixture.
func TestWithBlockerUpdateRejectsEmptyPrefix(t *testing.T) {
	called := false
	err := WithBlockerUpdate("", func(blockers []*models.Blocker, blocker *models.Blocker) ([]*models.Blocker, error) {
		called = true
		return blockers, nil
	})
	if err == nil {
		t.Fatal("expected error for empty blocker prefix")
	}
	if called {
		t.Error("update callback should not have been invoked for empty prefix")
	}
	if !strings.Contains(err.Error(), "blocker not found") {
		t.Errorf("expected 'blocker not found' in error, got: %v", err)
	}
}
