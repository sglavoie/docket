package tui

import (
	"strings"
	"testing"
)

// triageInboxToZero moves every Inbox task to Backlog from Inbox mode and
// returns the observation the last move published.
func triageInboxToZero(t *testing.T, driver *UITestDriver) UITestObservation {
	t.Helper()
	obs := pressKeys(t, driver, "g", "i")
	if !strings.Contains(obs.PlainScreen, "Inbox") {
		t.Fatalf("gi did not reach Inbox:\n%s", obs.PlainScreen)
	}
	for range 12 {
		if driver.App().workspacePulse.Inbox == 0 {
			return obs
		}
		before := driver.App().workspacePulse.Inbox
		obs = pressKeys(t, driver, "w", "b")
		if driver.App().workspacePulse.Inbox == before {
			obs = pressKeys(t, driver, "j")
		}
	}
	t.Fatalf("the Inbox never emptied:\n%s", obs.PlainScreen)
	return obs
}

// Triaging the last Inbox task says so ahead of the undo, and with motion the
// divider sweeps in the celebration color.
func TestTriagingTheLastInboxTaskCelebrates(t *testing.T) {
	driver := clearTodayDriver(t, true)
	obs := triageInboxToZero(t, driver)
	if !strings.Contains(obs.PlainScreen, "Inbox zero · u undo") {
		t.Fatalf("emptying the Inbox was not said:\n%s", obs.PlainScreen)
	}
	flash := driver.App().feedback.flash
	if !flash.active() || flash.severity != flashCelebrate {
		t.Fatalf("flash = %+v, want a live celebration", flash)
	}

	// Undo refills the Inbox, which is no triage; emptying it again the same
	// day says nothing more.
	obs = pressKeys(t, driver, "u")
	if strings.Contains(obs.PlainScreen, "Inbox zero") {
		t.Fatalf("an undo celebrated:\n%s", obs.PlainScreen)
	}
	obs = pressKeys(t, driver, "u")
	if strings.Contains(obs.PlainScreen, "Inbox zero") {
		t.Fatalf("a second undo celebrated:\n%s", obs.PlainScreen)
	}
	// Forget the day's celebration so only the restore rule can keep a redo
	// that empties the Inbox quiet.
	driver.App().feedback.inboxCelebratedDay = ""
	obs = pressKeys(t, driver, "ctrl+r", "ctrl+r")
	if driver.App().workspacePulse.Inbox != 0 {
		t.Fatalf("redo left %d Inbox tasks", driver.App().workspacePulse.Inbox)
	}
	if strings.Contains(obs.PlainScreen, "Inbox zero") {
		t.Fatalf("a redo celebrated:\n%s", obs.PlainScreen)
	}
}

func TestClearingTheInboxWithoutMotionSpeaksQuietly(t *testing.T) {
	driver := clearTodayDriver(t, false)
	obs := triageInboxToZero(t, driver)
	if !strings.Contains(obs.PlainScreen, "Inbox zero") {
		t.Fatalf("emptying the Inbox was not said:\n%s", obs.PlainScreen)
	}
	if driver.App().feedback.flash.active() {
		t.Fatalf("a session without motion animated: %+v", driver.App().feedback.flash)
	}
}
