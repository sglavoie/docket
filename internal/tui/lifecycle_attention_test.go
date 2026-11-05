package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/handoff"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/util"
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

// TestANudgeAsksTheTerminalToRaiseTheWindow covers the running half of a
// second launch's hand-off: the nudge writes the raise sequence and nothing
// else, wherever the session stands.
func TestANudgeAsksTheTerminalToRaiseTheWindow(t *testing.T) {
	app := typedTestApp(t)
	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	handled, cmd := app.handleFocusMessage(raiseWindowMsg{})
	if !handled || cmd == nil {
		t.Fatalf("nudge handled = %v, cmd = %v; want a raise", handled, cmd)
	}
	raw, ok := cmd().(tea.RawMsg)
	if !ok || raw.Msg != handoff.RaiseSequence {
		t.Fatalf("nudge wrote %#v, want the raise sequence", cmd())
	}
}

// TestOnlyATerminalSessionAdvertisesTheNudge keeps a headless session, which
// has no terminal to raise, from subscribing to the signal or claiming it.
func TestOnlyATerminalSessionAdvertisesTheNudge(t *testing.T) {
	session, nudges, stop := sessionNudges(true)
	stop()
	if session != util.SessionHeadless || nudges != nil {
		t.Fatalf("headless = %q, %v; want a headless session with no nudge", session, nudges)
	}
	session, _, stop = sessionNudges(false)
	stop()
	if session != util.SessionTerminal {
		t.Fatalf("terminal session = %q", session)
	}
}
