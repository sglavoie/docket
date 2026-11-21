package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The harness clock is noon on Sunday 2026-11-15.

func TestADayIsReadBackAsItIsTyped(t *testing.T) {
	h := project(t).wide()
	// On the capture line.
	h.press("a", `"Pay rent @+10d"`)
	if got := h.bottom(); !strings.Contains(got, "@+10d is Wed Nov 25 · in 10 days") {
		t.Fatalf("capture line = %q", got)
	}
	h.press("esc", "u")
	// On the stamp line: eow is the end of this week, today on a Sunday.
	h.press("@", `"eow"`)
	if got := h.bottom(); !strings.Contains(got, "@eow is Sun Nov 15 · today") {
		t.Fatalf("stamp line = %q", got)
	}
	h.press("esc")
	// On a row being typed, start days included.
	h.press("A", `" @@dec4"`)
	if got := h.status(); got != "@@dec4 is Fri Dec 4 · in 19 days" {
		t.Fatalf("row = %q", got)
	}
	h.press("esc")
	if got := h.field("Second", "start"); got != "2026-12-04" {
		t.Fatalf("start = %q", got)
	}
	// A day that cannot be read says nothing of the kind.
	h.press("A", `" @someday"`)
	if got := h.status(); strings.Contains(got, " is ") {
		t.Fatalf("unreadable day read back: %q", got)
	}
}

func TestTheWindowTitleAndProgressCountWhatWaits(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Project\n\tFirst\n\tSecond\n\tLate @2026-11-01")
	v := h.m.View()
	if v.WindowTitle != "Docket · overdue 1" {
		t.Fatalf("title = %q", v.WindowTitle)
	}
	if v.ProgressBar == nil || v.ProgressBar.State != tea.ProgressBarNone {
		t.Fatalf("progress with nothing picked = %+v", v.ProgressBar)
	}
	h.press("a", `"Buy milk"`, "enter", "esc")
	h.press("g", "g", "j", "t", "j", "t")
	v = h.m.View()
	if v.WindowTitle != "Docket · inbox 1 · today 2 · overdue 1" {
		t.Fatalf("title = %q", v.WindowTitle)
	}
	if p := v.ProgressBar; p == nil || p.State != tea.ProgressBarWarning || p.Value != 0 {
		t.Fatalf("progress = %+v", p)
	}
	// Closing a pick fills the bar; with nothing overdue it is no warning.
	h.press("x")
	h.press("j", `"@"`, "backspace", `"-@"`, "enter")
	v = h.m.View()
	if v.WindowTitle != "Docket · inbox 1 · today 1" {
		t.Fatalf("title = %q", v.WindowTitle)
	}
	if p := v.ProgressBar; p == nil || p.State != tea.ProgressBarDefault || p.Value != 50 {
		t.Fatalf("progress = %+v", p)
	}
}

func TestRNotesWhereWorkWasLeftOff(t *testing.T) {
	h := project(t).wide()
	h.press("R")
	if got := h.bottom(); !strings.HasPrefix(got, "Left off ›") {
		t.Fatalf("prompt = %q", got)
	}
	h.press(`"Next: reproduce with an expired token"`, "enter")
	h.wantStatus("noted where it was left off")
	if got := h.field("Second", "resume"); got != "Next: reproduce with an expired token" {
		t.Fatalf("resume = %q", got)
	}
	// It is read whenever the cursor is on the row, and nowhere else.
	h.press("k")
	if got := h.status(); strings.Contains(got, "left off") {
		t.Fatalf("another row said %q", got)
	}
	h.press("j")
	h.wantStatus("left off: Next: reproduce with an expired token")
	// Taken up again, the row reads the note back.
	h.press("$", `"doing"`, "enter")
	h.wantStatus("left off: Next: reproduce with an expired token")
	// R opens the note as it stands; a blank line takes it off, one undo step.
	h.press("R")
	if got := h.bottom(); !strings.Contains(got, "Next: reproduce with an expired token") {
		t.Fatalf("prompt = %q", got)
	}
	h.press("ctrl+u", "enter")
	h.wantStatus("left-off note taken off")
	if got := h.field("Second", "resume"); got != "" {
		t.Fatalf("resume = %q", got)
	}
	h.press("u")
	if got := h.field("Second", "resume"); got != "Next: reproduce with an expired token" {
		t.Fatalf("undo left resume = %q", got)
	}
	// Esc leaves it as it was, and a selection is refused.
	h.press("R", `" more"`, "esc")
	if got := h.field("Second", "resume"); got != "Next: reproduce with an expired token" {
		t.Fatalf("esc changed resume to %q", got)
	}
	h.press("v", "R")
	if got := h.status(); !strings.HasPrefix(got, "that works on one row: press Esc to clear the selection") {
		t.Fatalf("status = %q", got)
	}
	if h.m.ask != nil {
		t.Fatal("R opened a prompt over a selection")
	}
}

func TestClosingRowsSaysWhatTheDayHasDone(t *testing.T) {
	h := project(t)
	h.press("x")
	h.wantStatus("1 done today")
	h.press("k", "x")
	h.wantStatus("2 done today")
	// Reopening finishes nothing, and says nothing of it.
	h.press("x")
	if got := h.status(); strings.Contains(got, "done today") {
		t.Fatalf("reopening said %q", got)
	}
}

func TestBuiltInFiltersAreAlwaysThere(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Alpha\n\tWrite $doing\n\tLate @2026-11-01\n\tLater\n\tChase Ann $waiting")
	h.press("F", `"/suggest"`, "enter")
	// Soonest deadline first, as a flat filter that asks about deadlines.
	if got := h.shown(); got != "Late, Write" {
		t.Fatalf("/suggest = %q", got)
	}
	h.press("esc", "F", `"/slipping"`, "enter")
	if got := h.shown(); got != "Late" {
		t.Fatalf("/slipping = %q", got)
	}
	// A week on, the doing row has stalled; the waiting one not yet.
	h.clock = h.clock.AddDate(0, 0, 8)
	h.tick()
	h.press("esc", "F", `"/stuck"`, "enter")
	if got := h.shown(); got != "Write" {
		t.Fatalf("/stuck = %q", got)
	}
	// They are reached by name, not listed in the sidebar, and not forgotten.
	if strings.Contains(h.side(), "/suggest") {
		t.Fatalf("sidebar lists a built-in filter:\n%s", h.side())
	}
	h.press("esc", ":", `"forget stuck"`, "enter")
	h.wantStatus("/stuck is built in: :save stuck over it puts your own in its place")
	// Saved under its name, one becomes the user's and takes a place there.
	h.press("F", `"/stuck"`, "enter", ":", `"save stuck"`, "enter")
	if !strings.Contains(h.side(), "/stuck") {
		t.Fatalf("a saved built-in is not in the sidebar:\n%s", h.side())
	}
	h.press(":", `"forget stuck"`, "enter")
	h.wantStatus("forgot /stuck")
	h.press("F", `"/stuck"`, "enter")
	if h.m.at.query != "$doing -~7d | $waiting -~2w" {
		t.Fatalf("the built-in did not come back: %q", h.m.at.query)
	}
}

// /now is the work in hand: each doing row where it lives, with the steps
// still open under it, so the next one is read beside what it serves.
func TestNowShowsTheWorkInHandWithItsOpenSteps(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Alpha\n\tDraft $doing\n\t\tOutline $done\n\t\tWrite intro\n\t\tSend $waiting\n\tLater\nBeta\n\tFix $doing")
	h.press("F", `"/now"`, "enter")
	if got := h.shown(); got != "Alpha, Draft, Write intro, Beta, Fix" {
		t.Fatalf("/now = %q", got)
	}
	if h.m.at.flat {
		t.Fatal("/now is flat; it is read in context")
	}
}

func TestBlockedHoldsForWorkWaitingOnAnOpenLink(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Alpha\n\tShip\n\tReview\n\tNotes:")
	h.press("g", "g", "j", "L", `"Review"`, "enter")
	h.press("j", "j", "L", `"Ship"`, "enter") // a note pointing at work is not blocked
	h.press("F", `"/blocked"`, "enter")
	if got := h.shown(); got != "Ship" {
		t.Fatalf("/blocked = %q", got)
	}
	h.press("esc", "/", `"$blocked"`)
	if got := h.bottom(); !strings.Contains(got, "waiting on a linked task still to do") {
		t.Fatalf("read back = %q", got)
	}
	h.press("esc")
	// Once the work it waits on is done, it is no longer blocked.
	h.press("f", `"Review"`, "enter", "x")
	h.press("F", `"/blocked"`, "enter")
	if got := h.shown(); got != "" {
		t.Fatalf("/blocked after review = %q", got)
	}
}

func TestTheFilterAndSortReadWhenARowWasEdited(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Alpha\n\tOne\n\tTwo\n\tThree")
	h.clock = h.clock.AddDate(0, 0, 3)
	h.tick()
	// Two is renamed; One is only moved, which is no edit.
	h.press("g", "g", "j", "j", "A", `" again"`, "esc", "k", "J")
	h.filters(map[string]string{
		"~~1d":  "Two again",
		"-~~1d": "Alpha, One, Three",
		"~1d":   "", // its state did not change
	})
	h.press("/", `"~~1d"`)
	if got := h.bottom(); !strings.Contains(got, "edited since") {
		t.Fatalf("read back = %q", got)
	}
	h.press("esc", "f", `"Alpha"`, "enter", "j", ":", `"sort ~~"`, "enter")
	if got := h.shown(); got != "Two again, One, Three" {
		t.Fatalf("sorted = %q", got)
	}
	// A sort moves rows and edits none, so it is still in order.
	h.press(":", `"sort ~~"`, "enter")
	h.wantStatus("already in order")
	// The edit is read from the stored stamps after a restart.
	again := resume(t, h.path, h.st, h.clock)
	again.wide()
	again.filters(map[string]string{"~~1d": "Two again"})
}

func TestSortReadsATagsValue(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Alpha\n\tA #p=3\n\tB #p=1\n\tC\n\tD #p\n\tE #p=10")
	h.press("g", "g", "j", ":", `"sort #p"`, "enter")
	if got := h.shown(); got != "Alpha, B, A, E, D, C" {
		t.Fatalf("sorted = %q", got)
	}
	h.press(":", `"sort #p=2"`, "enter")
	h.wantStatus("unknown sort key: #p=2: the keys are @ ! $ ~ ~~ #tag, or none for text")
}

func TestCaptureFromTodayPicksTheRowForTheDay(t *testing.T) {
	h := project(t).wide()
	h.press("t", "f", `"Today"`, "enter")
	h.press("a", `"Buy milk"`)
	if got := h.bottom(); !strings.Contains(got, "files in Inbox · picked for Today") {
		t.Fatalf("capture line = %q", got)
	}
	h.press("enter")
	if got := h.bottom(); !strings.Contains(got, "added to Inbox · picked for Today") {
		t.Fatalf("after enter = %q", got)
	}
	h.press("esc")
	if got := h.shown(); got != "Second, Buy milk" {
		t.Fatalf("Today = %q", got)
	}
	if got := h.field("Buy milk", "today"); got != "2026-11-15" {
		t.Fatalf("today = %q", got)
	}
	// Elsewhere, capture picks nothing.
	h.press("f", `"Project"`, "enter", "a", `"Call Bob"`, "enter", "esc")
	if got := h.field("Call Bob", "today"); got != "" {
		t.Fatalf("captured outside Today, today = %q", got)
	}
}
