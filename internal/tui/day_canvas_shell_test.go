package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/pulse"
)

// Today with room under its rows draws the day canvas there: once a task is
// finished, the Done block lists it struck under the day's count, and the week
// ahead follows. It is paint in the padding, so the cursor's screen row is the
// one the list alone decides.
func TestTodayDrawsTheDayCanvasUnderItsRows(t *testing.T) {
	driver := launchReadmeFixture(t)
	// Plan two tasks for Today, open Today, and finish the first of them.
	obs := pressKeys(t, driver, "j", "w", "t", "j", "w", "t", "g", "t", "g", "g", "w", "d")
	if !strings.Contains(obs.PlainScreen, "Done today · 1") {
		t.Fatalf("Today drew no Done block after a completion:\n%s", obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "✓ ") || !strings.Contains(obs.PlainScreen, "[website]") {
		t.Fatalf("the Done block names no finished task:\n%s", obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "Next 7 days") {
		t.Fatalf("Today drew no week ahead:\n%s", obs.PlainScreen)
	}
	if len(obs.LayoutIssues) != 0 {
		t.Fatalf("layout issues: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
	// The canvas answers no row: the outline still holds only Today's work.
	for _, row := range driver.App().crossOutliner.Rows {
		if row.Task != nil && row.Task.IsTerminalState() {
			t.Fatalf("a finished task became a row: %q", row.Task.Title)
		}
	}
}

// The canvas reads the whole scope, so a narrowed Today — a search here —
// draws none rather than a canvas that disagrees with the list beside it.
func TestTheDayCanvasStandsDownUnderASearch(t *testing.T) {
	driver := launchReadmeFixture(t)
	pressKeys(t, driver, "j", "w", "t", "j", "w", "t", "g", "t", "g", "g", "w", "d")
	obs := pressKeys(t, driver, "/", "f", "e", "e", "d", "enter")
	if strings.Contains(obs.PlainScreen, "Next 7 days") {
		t.Fatalf("a narrowed Today still drew the canvas:\n%s", obs.PlainScreen)
	}
}

// Finishing the last task planned for Today leaves the resting card under the
// empty state's own sentence: the plan is complete — never the day — with its
// count, and looking back and choosing more from Next are offered as equals.
func TestAnEmptiedTodayRestsOnTheDoneCard(t *testing.T) {
	w, err := newUITestWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Cleanup() })
	if _, err = w.addProject("Home", func(p *models.Project) {
		newFixtureTask(p, "Water the plants").ApplyWorkflowState(models.StateInbox)
	}); err != nil {
		t.Fatal(err)
	}
	driver, err := w.launch(100, 32)
	if err != nil {
		t.Fatal(err)
	}
	obs := pressKeys(t, driver, "w", "t", "g", "t", "g", "g", "w", "d")
	if want := "Today's plan is complete. You finished 1 task today. Press gr to review completed work, or gn to choose from Next."; !strings.Contains(strings.Join(strings.Fields(obs.PlainScreen), " "), want) {
		t.Fatalf("the finished Today does not acknowledge its plan as %q:\n%s", want, obs.PlainScreen)
	}
	if strings.Contains(obs.PlainScreen, "Nothing planned") {
		t.Fatalf("a finished Today reads as one nothing was planned into:\n%s", obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "gn to choose from Next · gr review completed") {
		t.Fatalf("the footer does not offer both next steps:\n%s", obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "Finished today") || !strings.Contains(obs.PlainScreen, "1 task · ") {
		t.Fatalf("the emptied Today drew no resting card:\n%s", obs.PlainScreen)
	}
	if len(obs.LayoutIssues) != 0 {
		t.Fatalf("layout issues: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
}

// A workspace holding no task at all opens on the welcome, which teaches
// capture by example; capturing the first task retires it for good.
func TestAnEmptyWorkspaceOpensOnTheWelcome(t *testing.T) {
	w, err := newUITestWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Cleanup() })
	driver, err := w.launch(120, 36)
	if err != nil {
		t.Fatal(err)
	}
	obs := driver.Observe(ObserveOptions{})
	for _, want := range []string{"Welcome to Docket", "Call the bank !1 @fri #errands", "priority 1 · due Friday"} {
		if !strings.Contains(obs.PlainScreen, want) {
			t.Fatalf("the welcome is missing %q:\n%s", want, obs.PlainScreen)
		}
	}
	if len(obs.LayoutIssues) != 0 {
		t.Fatalf("layout issues: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
	for _, action := range []UITestAction{
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: "First task"},
		{Type: UITestActionKey, Key: "enter"},
	} {
		if obs, err = driver.ApplyAction(action, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(obs.PlainScreen, "Welcome to Docket") {
		t.Fatalf("the welcome outlived the first capture:\n%s", obs.PlainScreen)
	}
}

// A workspace whose work is all finished is not new: an empty All there keeps
// its plain sentence and no welcome.
func TestAFinishedWorkspaceIsNotWelcomed(t *testing.T) {
	w, err := newUITestWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Cleanup() })
	if _, err = w.addProject("Old", func(p *models.Project) {
		newFixtureTask(p, "Long finished").ApplyWorkflowState(models.StateDone)
	}); err != nil {
		t.Fatal(err)
	}
	driver, err := w.launch(120, 36)
	if err != nil {
		t.Fatal(err)
	}
	if obs := driver.Observe(ObserveOptions{}); strings.Contains(obs.PlainScreen, "Welcome to Docket") {
		t.Fatalf("a workspace with history was welcomed:\n%s", obs.PlainScreen)
	}
}

// s folds the opened Backlog lane back. With motion the first frame holds the
// closed block's rows open as blank space under its header, so the rows below
// slide up instead of jumping; the frame keeps its height, the window and the
// cursor's screen row are the settled ones, and the wait lands on exactly the
// frame a session without motion drew at once.
func TestFoldCloseSlidesTheRowsBelowUp(t *testing.T) {
	still := motionFixture(t, false, 3, 6)
	motionFrame(t, still, "s")
	want := motionFrame(t, still, "s")
	moving := motionFixture(t, true, 3, 6)
	motionFrame(t, moving, "s")
	settledFrame(t, moving)
	frame := motionFrame(t, moving, "s")
	if frame.PlainScreen == want.PlainScreen {
		t.Fatalf("closing the fold drew the closed list at once with motion:\n%s", frame.PlainScreen)
	}
	if lines, wantLines := strings.Split(frame.PlainScreen, "\n"), strings.Split(want.PlainScreen, "\n"); len(lines) != len(wantLines) {
		t.Fatalf("collapse drew %d lines, want %d", len(lines), len(wantLines))
	}
	if got, wantRow := cursorRow(t, moving), cursorRow(t, still); got != wantRow {
		t.Fatalf("CursorScreenRow = %d during the collapse, want the settled %d", got, wantRow)
	}
	if got := settledFrame(t, moving); got.PlainScreen != want.PlainScreen {
		t.Fatalf("collapse did not land on the closed list:\n%s\nwant:\n%s", got.PlainScreen, want.PlainScreen)
	}
}

// b opens Peek. With motion the dock's divider and the head of its document
// draw first, and its tail rolls out under them; the dock stands at its full
// height from the first frame, so nothing above it moves.
func TestOpeningPeekRollsTheDocumentOut(t *testing.T) {
	still := motionFixture(t, false, 3, 0)
	want := motionFrame(t, still, "b")
	moving := motionFixture(t, true, 3, 0)
	frame := motionFrame(t, moving, "b")
	if frame.PlainScreen == want.PlainScreen {
		t.Fatalf("b drew the whole dock at once with motion:\n%s", frame.PlainScreen)
	}
	lines, wantLines := strings.Split(frame.PlainScreen, "\n"), strings.Split(want.PlainScreen, "\n")
	if len(lines) != len(wantLines) {
		t.Fatalf("the rolling dock drew %d lines, want %d", len(lines), len(wantLines))
	}
	// Everything above the dock is the settled frame's: the dock stands at its
	// reserved height from the first frame.
	dockTop := len(wantLines) - moving.App().crossOutliner.PeekDockRows() - 2
	for i := range max(dockTop, 0) {
		if lines[i] != wantLines[i] {
			t.Fatalf("row %d above the dock moved:\n%q\nwant\n%q", i, lines[i], wantLines[i])
		}
	}
	if got := settledFrame(t, moving); got.PlainScreen != want.PlainScreen {
		t.Fatalf("the dock did not land whole:\n%s\nwant:\n%s", got.PlainScreen, want.PlainScreen)
	}
	// Moving the cursor under an open dock swaps the document in place.
	motionFrame(t, still, "j")
	if got, want := motionFrame(t, moving, "j"), still.Observe(ObserveOptions{}); got.PlainScreen != want.PlainScreen {
		t.Fatalf("a standing dock rolled out again:\n%s\nwant:\n%s", got.PlainScreen, want.PlainScreen)
	}
}

// a opens capture. With motion its rows above the caret ease in from the
// background, bottom first; the text and layout of the frame are the settled
// one's from the start, and only color differs until it lands.
func TestCapturePanelRisesIntoItsRows(t *testing.T) {
	still := motionFixture(t, false, 3, 0)
	want := pressKeys(t, still, "a")
	wantANSI := still.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen
	moving := motionFixture(t, true, 3, 0)
	frame := pressKeys(t, moving, "a")
	if frame.PlainScreen != want.PlainScreen {
		t.Fatalf("the rise changed the frame's text:\n%s\nwant:\n%s", frame.PlainScreen, want.PlainScreen)
	}
	if moving.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen == wantANSI {
		t.Fatal("capture opened without rising")
	}
	waitFor(t, moving, "300ms")
	if got := moving.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen; got != wantANSI {
		t.Fatalf("the rise did not land on the settled panel:\n%s\nwant:\n%s", got, wantANSI)
	}
}

// A count that jumps by several steps counts its way there before it lights,
// and every in-between value is drawn at the settled count's width.
func TestAPulseCountThatJumpsTicksBeforeItLights(t *testing.T) {
	driver := motionFixture(t, true, 3, 0)
	app := driver.App()
	before := pulse.WorkspacePulse{Inbox: 12, Today: 1}
	app.workspacePulse = pulse.WorkspacePulse{Inbox: 18, Today: 1}
	if app.markPulseChange(before, false) == nil || app.pulseMotion.change.Ticking == 0 {
		t.Fatalf("a jump of six drew no ticker: %+v", app.pulseMotion.change)
	}
	frames := 0
	for app.pulseMotion.change.Ticking != 0 {
		frames++
		if frames > 10 {
			t.Fatal("the ticker never settled")
		}
		app.advancePulseMotion(pulseMotionTickMsg{Seq: app.pulseMotion.seq})
	}
	if frames != pulse.TickerFrames(before, app.workspacePulse) {
		t.Fatalf("ticked %d frames, want %d", frames, pulse.TickerFrames(before, app.workspacePulse))
	}
	if change := app.pulseMotion.change; !change.Any() || change.Settling {
		t.Fatalf("after the ticker the count should be lit fresh, got %+v", change)
	}
}

// Motion off draws none of the new runs: the fold closes, the dock opens, and
// capture opens exactly as a session without motion draws them.
func TestMotionOffDisablesCollapseDockAndRise(t *testing.T) {
	for _, tc := range []struct {
		name    string
		backlog int
		setup   []string
		keys    []string
	}{
		{name: "collapse", backlog: 6, setup: []string{"s"}, keys: []string{"s"}},
		{name: "dock", keys: []string{"b"}},
		{name: "rise", keys: []string{"a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			still := motionFixture(t, false, 3, tc.backlog)
			pressKeys(t, still, tc.setup...)
			pressKeys(t, still, tc.keys...)
			want := still.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen
			driver := motionFixture(t, true, 3, tc.backlog)
			reduceMotion(driver)
			pressKeys(t, driver, tc.setup...)
			pressKeys(t, driver, tc.keys...)
			if got := driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen; got != want {
				t.Fatalf("motion off still animated:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
