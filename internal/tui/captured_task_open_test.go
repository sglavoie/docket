package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
)

// captureText types one draft into the global composer and submits it.
func captureText(t *testing.T, driver *UITestDriver, text string) UITestObservation {
	t.Helper()
	var obs UITestObservation
	for _, action := range []UITestAction{
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: text},
		{Type: UITestActionKey, Key: "enter"},
	} {
		var err error
		if obs, err = driver.ApplyAction(action, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return obs
}

func capturedCursorTitle(driver *UITestDriver) string {
	out := driver.App().crossOutliner
	if out.Cursor < 0 || out.Cursor >= len(out.Rows) || out.Rows[out.Cursor].Task == nil {
		return ""
	}
	return out.Rows[out.Cursor].Task.Title
}

// A capture keeps the cursor where the user was reading and says how to get
// to what it saved; gl goes there and Esc comes back to the same task.
func TestCaptureOffersTheWayToItsTaskAndBack(t *testing.T) {
	driver := motionFixture(t, false, 3, 0)
	pressKeys(t, driver, "j", "j")
	reading := capturedCursorTitle(driver)
	if reading == "" {
		t.Fatal("the fixture parked the cursor on no task")
	}

	obs := captureText(t, driver, "Freshly captured")
	if got := driver.App().feedback.message; !strings.HasSuffix(got, " · gl open · u undo") {
		t.Fatalf("the capture toast offers no way to its task: %q", got)
	}
	if !strings.Contains(obs.PlainScreen, "gl open · u undo") {
		t.Fatalf("the footer drew no open ending:\n%s", obs.PlainScreen)
	}
	if got := capturedCursorTitle(driver); got != reading {
		t.Fatalf("the capture moved the cursor to %q, want %q", got, reading)
	}

	pressKeys(t, driver, "g", "l")
	if got := capturedCursorTitle(driver); got != "Freshly captured" {
		t.Fatalf("gl landed on %q", got)
	}
	if got := driver.App().crossOutliner.EscHint(); !strings.Contains(got, "origin") {
		t.Fatalf("the footer offers no way back: %q", got)
	}
	pressKeys(t, driver, "esc")
	if got := capturedCursorTitle(driver); got != reading {
		t.Fatalf("Esc returned to %q, want %q", got, reading)
	}
}

// A task captured outside the view is reached by changing the view, and Esc
// puts the whole view back, not only the cursor.
func TestOpenCapturedTaskHopsToAViewThatDrawsIt(t *testing.T) {
	driver := motionFixture(t, false, 3, 0)
	out := driver.App().crossOutliner
	out.SelectOutlinerMode(chrome.ModeToday)
	out.RebuildRows()

	// Capture from Today plans for Today by default, so the draft names Inbox
	// to land outside the view it was typed over.
	captureText(t, driver, "Lands in the Inbox $inbox")
	pressKeys(t, driver, "g", "l")
	if got := capturedCursorTitle(driver); got != "Lands in the Inbox" {
		t.Fatalf("gl landed on %q", got)
	}
	if out.PrimaryMode != chrome.ModeAll {
		t.Fatalf("the hop stayed in %v", out.PrimaryMode)
	}
	pressKeys(t, driver, "esc")
	if out.PrimaryMode != chrome.ModeToday {
		t.Fatalf("Esc returned to %v, want Today", out.PrimaryMode)
	}
}

// gl with nothing remembered says so, and a multiline capture names none of
// its tasks, so its toast offers no way to one.
func TestOpenCapturedTaskRefusesWithoutASingleCapture(t *testing.T) {
	driver := motionFixture(t, false, 3, 0)
	pressKeys(t, driver, "g", "l")
	if got := driver.App().crossOutliner.Rows; len(got) == 0 {
		t.Fatal("the fixture drew no rows")
	}
	if got := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(got, "Nothing captured yet this session") {
		t.Fatalf("gl without a capture said nothing:\n%s", got)
	}

	captureText(t, driver, "One\nTwo")
	if got := driver.App().feedback.message; strings.Contains(got, "gl open") {
		t.Fatalf("a multiline capture offered one task: %q", got)
	}
}

// A narrow footer gives up the head of the toast, never its keys.
func TestCaptureOpenEndingSurvivesANarrowRow(t *testing.T) {
	line := "Captured to website · Inbox: " + strings.Repeat("long title ", 6) + recoveryOpenSuffix + recoveryUndoSuffix
	got := fitRecoveryFeedback(line, chrome.MinTerminalWidth)
	if !strings.HasSuffix(got, "gl open · u undo") {
		t.Fatalf("the fit dropped a key: %q", got)
	}
	head, endings := splitRecoverySuffix(line)
	if !endings.open || !endings.undo || endings.origin || strings.Contains(head, "gl open") {
		t.Fatalf("split misread the endings: %q %+v", head, endings)
	}
}

// A capture into a folded lane opens no fold to show itself: the reader's
// view is what they left. gl opens the fold to reach it and Esc closes it again.
func TestCaptureIntoAFoldedLaneLeavesTheFoldAlone(t *testing.T) {
	driver := motionFixture(t, false, 3, 4)
	folded := driver.Observe(ObserveOptions{}).PlainScreen
	if strings.Contains(folded, "Later task 01") {
		t.Fatalf("the launch did not fold Backlog:\n%s", folded)
	}
	captureText(t, driver, "Fresh %bulk $backlog")
	if got := driver.Observe(ObserveOptions{}).PlainScreen; strings.Contains(got, "Later task 01") {
		t.Fatalf("the capture opened the Backlog fold:\n%s", got)
	}
	if got := driver.App().feedback.message; !strings.Contains(got, "gl open") {
		t.Fatalf("a capture into a folded lane offered no way there: %q", got)
	}
	pressKeys(t, driver, "g", "l")
	if got := capturedCursorTitle(driver); got != "Fresh" {
		t.Fatalf("gl landed on %q", got)
	}
	pressKeys(t, driver, "esc")
	if got := driver.Observe(ObserveOptions{}).PlainScreen; strings.Contains(got, "Later task 01") {
		t.Fatalf("Esc left the Backlog fold open:\n%s", got)
	}
}
