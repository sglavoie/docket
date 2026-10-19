package tui

import (
	"strings"
	"testing"
)

// TestUXRefinementFloorHoldsContextPeekAndCapture is the one combination the
// refinement leaves the least room for, read at the one size that has the least
// of it: the app's own floor, carrying a context row, a Peek dock, a capture
// panel, and the guidance capture requires, all at once. Every other check in
// this file measures one occupant against the frame; this one measures four
// against each other, because the rows they compete for are the same rows and
// the dock is the occupant that yields.
//
// It is a Go test rather than a scenario for the reason the matrix above is: the
// claim is about where three blocks sit relative to one another, which a
// contains assertion over the whole screen cannot say.
func TestUXRefinementFloorHoldsContextPeekAndCapture(t *testing.T) {
	size := uxPresentationMatrixSizes[0]
	driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, size[0], size[1])
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatal("the fixture app has no Outliner")
	}

	// The context row first, since it is the occupant charged against the list
	// before either of the other two exists: a narrowing search and an explicit
	// sort are two live axes, so the identity row gains the second row naming them.
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "/"},
		UITestAction{Type: UITestActionText, Text: uxClaritySearchQuery},
		UITestAction{Type: UITestActionKey, Key: "enter"},
		UITestAction{Type: UITestActionCommand, Command: "Sort by Priority"},
	)
	uxClarityPress(t, driver, uxClarityKeys("b")...)
	if !view.PeekDockVisible() {
		t.Fatal("b did not open the Peek dock over the searched projection")
	}
	// The height the frame gives the dock with nothing else raised over it, which
	// is what closing capture has to return to.
	resting := view.PeekDockRows()
	uxClarityPress(t, driver, uxClarityKeys("a")...)

	obs := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, obs, size)
	plain := obs.PlainScreen
	lines := strings.Split(plain, "\n")

	// Four occupants, each named by something only it draws.
	if _, ok := uxLineContaining(plain, "Search:"); !ok {
		t.Fatalf("the context row is gone from the crowded floor\n%s", plain)
	}
	if !strings.Contains(plain, "── Capture ─") {
		t.Fatalf("the capture panel is not drawn\n%s", plain)
	}
	if footer := uxClarityFooter(driver); !strings.Contains(footer, "Enter create") {
		t.Fatalf("the footer does not carry the guidance capture requires:\n%s", footer)
	}
	dockRows := view.PeekDockRows()
	if drawn := len(view.PeekDockLines()); drawn != dockRows {
		t.Fatalf("the dock reports %d rows and drew %d", dockRows, drawn)
	}
	// Three rows is what a displayed dock owes: one content line, the divider, and
	// the position line. Under this much pressure it may be exactly that and no
	// more, and it may not be less while it is still on screen.
	if dockRows < 3 {
		t.Fatalf("the dock is %d rows under capture at the floor, want at least three", dockRows)
	}
	if dockRows > resting {
		t.Fatalf("capture grew the dock from %d rows to %d", resting, dockRows)
	}

	// Where they sit, which is the part a whole-screen assertion cannot state.
	// Everything between the context row and the dock's divider belongs to the
	// list.
	divider, _, drawn := uxPeekDockSpan(lines, view)
	if !drawn {
		t.Fatalf("the dock is not drawn whole, so it is clipped rather than shortened\n%s", plain)
	}
	if !strings.Contains(lines[divider], "─") {
		t.Fatalf("the dock does not open on its divider\n%s", plain)
	}
	// And the list keeps a row of its own between the two, which is the floor the
	// dock's availability is capped against. A task title drawn there is what
	// proves the row is a task row rather than chrome.
	var listRows int
	for _, line := range lines[2:divider] {
		if uxTrimRowFurniture(line) != "" {
			listRows++
		}
	}
	if listRows < 1 {
		t.Fatalf("the dock and the panel left the list no rows at all\n%s", plain)
	}
	listText := strings.Join(lines[2:divider], "\n")
	taskRowDrawn := false
	for _, row := range view.Rows {
		// The row may have clipped its title to fit the project label a merged
		// group leads with, so its opening words are what has to be there.
		if row.IsTaskRow() && row.Task != nil && strings.Contains(listText, string([]rune(row.Task.Title)[:min(16, len([]rune(row.Task.Title)))])) {
			taskRowDrawn = true
			break
		}
	}
	if !taskRowDrawn {
		t.Fatalf("no task row survives between the context row and the dock\n%s", plain)
	}

	// Closing capture gives the presentation, the cursor, and the dock back. The
	// draft is empty, so Esc closes it outright rather than raising a discard
	// question, and the dock returns to the height the frame alone gives it.
	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	if !view.PeekDockVisible() {
		t.Fatal("closing capture closed the Peek dock with it")
	}
	if got := view.PeekDockRows(); got != resting {
		t.Fatalf("the dock came back at %d rows, want the %d the frame gives it", got, resting)
	}
	after := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, after, size)
	if _, ok := uxLineContaining(after.PlainScreen, "Search:"); !ok {
		t.Fatalf("closing capture dropped the context row\n%s", after.PlainScreen)
	}
}

// uxDrawnListLines counts the rows the list actually put on screen: everything
// between the operational title and the footer divider that carries anything.
// The padding a short projection is grown with is blank by construction, so
// dropping the blanks is what separates the rows drawn from the window sized.
func uxDrawnListLines(t *testing.T, plain string) int {
	t.Helper()
	lines := strings.Split(plain, "\n")
	footer := -1
	for index, line := range lines {
		if uxFullWidthRule.MatchString(uxTrimRowFurniture(line)) {
			footer = index
		}
	}
	if footer <= 0 {
		t.Fatalf("the screen draws no footer divider\n%s", plain)
	}
	var drawn int
	for _, line := range lines[1:footer] {
		if uxTrimRowFurniture(line) != "" {
			drawn++
		}
	}
	return drawn
}

// uxTrimRowFurniture drops the trailing padding and the scrollbar column, which
// are the two things a blank list row still carries.
func uxTrimRowFurniture(line string) string {
	return strings.TrimRight(line, " ▐│▕")
}

// uxLineContaining returns the first screen line holding needle.
func uxLineContaining(plain, needle string) (string, bool) {
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, needle) {
			return line, true
		}
	}
	return "", false
}
