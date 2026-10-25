package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/presentation"
)

// The shell half of Docket's background attention: the focus reload's owed state
// and the --notify gate. The fingerprint and the arrival rules are tested where
// they live, in internal/workspace and internal/tui/pulse.

// TestFocusWithoutBlurOwesNoReload keeps a launch, whose first report may be a
// focus with no blur before it, from reloading the workspace it just read.
func TestFocusWithoutBlurOwesNoReload(t *testing.T) {
	app := typedTestApp(t)
	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
	if handled, _ := app.handleFocusMessage(tea.FocusMsg{}); !handled {
		t.Fatal("focus was not handled")
	}
	if app.externalReload.owed {
		t.Fatal("a focus with no blur before it owes a reload")
	}
}

// TestExternalChangeWaitsForTheOutliner covers the owed half: a change the shell
// cannot adopt mid-interaction stays owed, and the tick that finds the Outliner
// live again adopts it once and announces it.
func TestExternalChangeWaitsForTheOutliner(t *testing.T) {
	app := typedTestApp(t)
	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	app.externalReload.owed = true

	if cmd := app.adoptExternalWorkspaceChange(); cmd != nil {
		t.Fatal("a reload ran while another view was live")
	}
	if !app.externalReload.owed {
		t.Fatal("a refused reload stopped being owed")
	}

	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
	if cmd := app.adoptExternalWorkspaceChange(); cmd == nil {
		t.Fatal("the owed reload did not run on the Outliner")
	}
	if app.externalReload.owed {
		t.Fatal("an adopted reload is still owed")
	}
	if got := app.feedback.message; got != "Reloaded: the workspace changed on disk" {
		t.Fatalf("status = %q, want the reload notice", got)
	}
	if cmd := app.adoptExternalWorkspaceChange(); cmd != nil {
		t.Fatal("a second attempt reloaded with nothing owed")
	}
}

func TestNotifyArrivalsIsOptIn(t *testing.T) {
	app := typedTestApp(t)
	app.notifications.since = time.Now().Add(-time.Hour)
	if cmd := app.notifyArrivals(time.Now()); cmd != nil {
		t.Fatal("a run without --notify produced a notification")
	}
}
