package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// The shell half of the advisory coverage: the `--no-advisories` option, the
// keyless View-menu toggle, and the checkbox that reads it back. What an
// advisory is and how the two helpers gate one stays in advisories_test.go.

// TestHiddenAdvisoriesOptionSuppressesTheIndicatorEndToEnd is the flag's path:
// `--no-advisories` becomes this option, and a real launch holding a bundle draws
// no conflict row.
func TestHiddenAdvisoriesOptionSuppressesTheIndicatorEndToEnd(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	dataDir := driver.App().domain.runtime.DataDir()
	store := storage.NewConflictStore(config.ConflictsDir(dataDir))
	if _, err = store.Save(storage.ConflictRecord{ProjectName: "Alpha"}, nil, nil, nil); err != nil {
		t.Fatalf("save conflict bundle: %v", err)
	}

	driver.App().refreshOutstandingConflicts()
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, "1 unresolved merge conflict") {
		t.Fatalf("the baseline launch does not draw the advisory:\n%s", screen)
	}

	WithHiddenAdvisories()(driver.App())
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; strings.Contains(screen, "unresolved merge conflict") {
		t.Fatalf("advisories are hidden and the row is still drawn:\n%s", screen)
	}
}

// TestAdvisoriesMenuEntryTogglesAndPersists holds the activation decision: no key
// is spent on this, so the View menu row is the only in-TUI route, and it writes
// machine-local chrome state like the bar's own toggle does.
func TestAdvisoriesMenuEntryTogglesAndPersists(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)

	if driver.App().domain.runtime.Machine().AdvisoriesHidden {
		t.Fatal("the fixture launched with advisories already hidden")
	}

	pressMenuBarKey(t, driver, "ctrl+m")
	view := menuWithID(t, openMenuBarMenusForTest(t, outlinerView), outliner.MenuIDView)
	if !menuHasLabel(view, "Advisories") {
		t.Fatalf("the View menu does not offer Advisories: %+v", view.Entries)
	}

	if err := driver.App().coreToggleAdvisories(); err != nil {
		t.Fatalf("coreToggleAdvisories: %v", err)
	}
	if !driver.App().domain.runtime.Machine().AdvisoriesHidden {
		t.Fatal("hiding advisories did not reach machine state")
	}
	if !outlinerView.AdvisoriesSuppressed() {
		t.Fatal("the Outliner still reads advisories as shown after the toggle")
	}

	if err := driver.App().coreToggleAdvisories(); err != nil {
		t.Fatalf("coreToggleAdvisories back: %v", err)
	}
	if driver.App().domain.runtime.Machine().AdvisoriesHidden {
		t.Fatal("showing advisories again did not reach machine state")
	}
	if outlinerView.AdvisoriesSuppressed() {
		t.Fatal("the Outliner still suppresses advisories after toggling back")
	}
}
