package tui

import (
	"strings"
	"testing"
)

func clearTodayDriver(t *testing.T, motion bool) *UITestDriver {
	t.Helper()
	driver := launchReadmeFixture(t)
	if motion {
		WithMotion()(driver.App())
	}
	return driver
}

func pressKeys(t *testing.T, driver *UITestDriver, keys ...string) UITestObservation {
	t.Helper()
	var obs UITestObservation
	for _, key := range keys {
		var err error
		if obs, err = driver.ApplyAction(UITestAction{Type: UITestActionKey, Key: key}, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return obs
}

// Planning one task for Today and finishing it empties Today: the toast says
// so ahead of its undo, and with motion the divider sweeps in the success color.
func TestFinishingTheLastTodayTaskCelebrates(t *testing.T) {
	driver := clearTodayDriver(t, true)
	obs := pressKeys(t, driver, "j", "j", "j", "j", "w", "t", "tab", "w", "d")
	if !strings.Contains(obs.PlainScreen, "Done · Today is clear · u undo") {
		t.Fatalf("emptying Today was not said:\n%s", obs.PlainScreen)
	}
	flash := driver.App().feedback.flash
	if !flash.active() || flash.severity != flashCelebrate {
		t.Fatalf("flash = %+v, want a live celebration", flash)
	}

	// Undo refills Today; finishing it again the same day says nothing more.
	pressKeys(t, driver, "u")
	obs = pressKeys(t, driver, "w", "d")
	if strings.Contains(obs.PlainScreen, "Today is clear") {
		t.Fatalf("a second clear the same day spoke again:\n%s", obs.PlainScreen)
	}
}

func TestClearingTodayWithoutMotionSpeaksQuietly(t *testing.T) {
	driver := clearTodayDriver(t, false)
	obs := pressKeys(t, driver, "j", "j", "j", "j", "w", "t", "tab", "w", "d")
	if !strings.Contains(obs.PlainScreen, "Today is clear") {
		t.Fatalf("emptying Today was not said:\n%s", obs.PlainScreen)
	}
	if driver.App().feedback.flash.active() {
		t.Fatalf("a session without motion animated: %+v", driver.App().feedback.flash)
	}
}

// Replanning the last Today task away empties Today without finishing it,
// which is not a celebration.
func TestReplanningTodayAwayIsNotACelebration(t *testing.T) {
	driver := clearTodayDriver(t, true)
	obs := pressKeys(t, driver, "j", "j", "j", "j", "w", "t", "tab", "w", "b")
	if strings.Contains(obs.PlainScreen, "Today is clear") {
		t.Fatalf("replanning celebrated:\n%s", obs.PlainScreen)
	}
}
