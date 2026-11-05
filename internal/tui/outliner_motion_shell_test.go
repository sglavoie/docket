package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
)

const (
	motionFixtureWidth  = 80
	motionFixtureHeight = 20
)

// motionFixture launches one project of inbox tasks and a folded Backlog lane,
// with motion armed or not. The two launches are otherwise the same, so a frame
// with motion can be compared against the frame a session without it draws.
func motionFixture(t *testing.T, armed bool, inbox, backlog int) *UITestDriver {
	t.Helper()
	w, err := newUITestWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Cleanup() })
	_, err = w.addProject("Bulk", func(p *models.Project) {
		for index := range inbox {
			newFixtureTask(p, fmt.Sprintf("Inbox task %02d", index+1)).ApplyWorkflowState(models.StateInbox)
		}
		for index := range backlog {
			newFixtureTask(p, fmt.Sprintf("Later task %02d", index+1)).ApplyWorkflowState(models.StateBacklog)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	driver, err := w.launch(motionFixtureWidth, motionFixtureHeight)
	if err != nil {
		t.Fatal(err)
	}
	if armed {
		WithMotion()(driver.App())
	}
	// A real session has drawn a frame before its first key; the scroll measures
	// from the window that frame recorded.
	driver.Observe(ObserveOptions{})
	return driver
}

// motionFrame presses keys and holds the frame they leave to the layout rules:
// no issue, and no line wider than the terminal.
func motionFrame(t *testing.T, driver *UITestDriver, keys ...string) UITestObservation {
	t.Helper()
	obs := pressKeys(t, driver, keys...)
	assertMotionFrameLayout(t, obs)
	return obs
}

func assertMotionFrameLayout(t *testing.T, obs UITestObservation) {
	t.Helper()
	if len(obs.LayoutIssues) != 0 {
		t.Fatalf("layout issues: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
	for _, line := range strings.Split(obs.PlainScreen, "\n") {
		if width := lipgloss.Width(line); width > motionFixtureWidth {
			t.Fatalf("line is %d wide, over %d: %q", width, motionFixtureWidth, line)
		}
	}
}

func settledFrame(t *testing.T, driver *UITestDriver) UITestObservation {
	t.Helper()
	waitFor(t, driver, "700ms")
	obs := driver.Observe(ObserveOptions{IncludeANSI: true})
	assertMotionFrameLayout(t, obs)
	return obs
}

func reduceMotion(driver *UITestDriver) {
	app := driver.App()
	app.lifecycle.hostReducesMotion = true
	app.applyMotion()
}

// G jumps the window to the list's end. With motion the frame the key draws is
// an intermediate window, while LastViewport already records the settled one,
// so the cursor's screen row and the pointer read where the list is going; the
// wait then lands on exactly the frame a session without motion drew at once.
func TestSmoothScrollPaintsBetweenWindowsAndLandsOnTheSettledOne(t *testing.T) {
	still := motionFixture(t, false, 40, 0)
	want := motionFrame(t, still, "G")
	moving := motionFixture(t, true, 40, 0)
	frame := motionFrame(t, moving, "G")
	if frame.PlainScreen == want.PlainScreen {
		t.Fatalf("G drew the settled window at once with motion:\n%s", frame.PlainScreen)
	}
	if got, wantRow := cursorRow(t, moving), cursorRow(t, still); got != wantRow {
		t.Fatalf("CursorScreenRow = %d during the scroll, want the settled %d", got, wantRow)
	}
	if moving.App().crossOutliner.LastViewport != still.App().crossOutliner.LastViewport {
		t.Fatal("the scroll moved the recorded window")
	}
	if got := settledFrame(t, moving); got.PlainScreen != want.PlainScreen {
		t.Fatalf("scroll did not land on the settled window:\n%s\nwant:\n%s", got.PlainScreen, want.PlainScreen)
	}
}

func TestSmoothScrollRetiresOnTheNextKeystroke(t *testing.T) {
	still := motionFixture(t, false, 40, 0)
	want := motionFrame(t, still, "G", "k")
	moving := motionFixture(t, true, 40, 0)
	if got := motionFrame(t, moving, "G", "k"); got.PlainScreen != want.PlainScreen {
		t.Fatalf("a keystroke left the scroll painting:\n%s\nwant:\n%s", got.PlainScreen, want.PlainScreen)
	}
}

// A one-row step never animates: k moves the window by at most a row.
func TestSmoothScrollIgnoresAOneRowStep(t *testing.T) {
	still := motionFixture(t, false, 40, 0)
	pressKeys(t, still, "G")
	want := motionFrame(t, still, "k", "k")
	moving := motionFixture(t, true, 40, 0)
	motionFrame(t, moving, "G")
	settledFrame(t, moving)
	if got := motionFrame(t, moving, "k", "k"); got.PlainScreen != want.PlainScreen {
		t.Fatalf("a one-row step animated:\n%s\nwant:\n%s", got.PlainScreen, want.PlainScreen)
	}
}

func cursorRow(t *testing.T, driver *UITestDriver) int {
	t.Helper()
	row, ok := driver.App().crossOutliner.CursorScreenRow()
	if !ok {
		t.Fatal("cursor not drawn")
	}
	return row
}

// s opens the folded Backlog lane. With motion the first frame draws the head
// of the opened block and blanks its tail at the rows it will fill, so no row
// below it moves; the wait draws all of it.
func TestFoldRevealRollsTheOpenedRowsOut(t *testing.T) {
	still := motionFixture(t, false, 3, 6)
	want := motionFrame(t, still, "s")
	moving := motionFixture(t, true, 3, 6)
	frame := motionFrame(t, moving, "s")
	if frame.PlainScreen == want.PlainScreen {
		t.Fatalf("s drew every opened row at once with motion:\n%s", frame.PlainScreen)
	}
	if !strings.Contains(frame.PlainScreen, "Later task 01") || strings.Contains(frame.PlainScreen, "Later task 06") {
		t.Fatalf("first frame should draw the block's head and withhold its tail:\n%s", frame.PlainScreen)
	}
	if lines, wantLines := strings.Split(frame.PlainScreen, "\n"), strings.Split(want.PlainScreen, "\n"); len(lines) != len(wantLines) {
		t.Fatalf("reveal drew %d lines, want %d", len(lines), len(wantLines))
	}
	if got := settledFrame(t, moving); got.PlainScreen != want.PlainScreen {
		t.Fatalf("reveal did not land on the opened rows:\n%s\nwant:\n%s", got.PlainScreen, want.PlainScreen)
	}
}

func TestFoldRevealRetiresOnTheNextKeystroke(t *testing.T) {
	still := motionFixture(t, false, 3, 6)
	want := motionFrame(t, still, "s", "j")
	moving := motionFixture(t, true, 3, 6)
	if got := motionFrame(t, moving, "s", "j"); got.PlainScreen != want.PlainScreen {
		t.Fatalf("a keystroke left the reveal painting:\n%s\nwant:\n%s", got.PlainScreen, want.PlainScreen)
	}
}

// J lands the cursor's task one row down. The trail is paint on the cursor's
// band alone, so the plain frame is the one a session without motion draws and
// only the cursor row's colors differ; it fades out on its own, and a second J
// lights it again.
func TestMoveTrailLightsTheLandedRowThenFades(t *testing.T) {
	still := motionFixture(t, false, 3, 0)
	want := pressKeys(t, still, "J")
	wantANSI := still.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen
	moving := motionFixture(t, true, 3, 0)
	frame := motionFrame(t, moving, "J")
	if frame.PlainScreen != want.PlainScreen {
		t.Fatalf("the trail changed a character:\n%s\nwant:\n%s", frame.PlainScreen, want.PlainScreen)
	}
	row := cursorRow(t, moving)
	if !strings.Contains(strings.Split(frame.PlainScreen, "\n")[row], "Inbox task 01") {
		t.Fatalf("cursor row %d does not hold the moved task:\n%s", row, frame.PlainScreen)
	}
	lit := moving.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen
	if ansiLine(lit, row) == ansiLine(wantANSI, row) {
		t.Fatal("the landed row is painted as if nothing moved")
	}
	if ansiLine(lit, row-1) != ansiLine(wantANSI, row-1) {
		t.Fatal("the trail painted a row other than the landed one")
	}
	// The footer is left out: motion also runs the undo countdown on its divider.
	if got := settledFrame(t, moving); ansiLine(got.ANSIScreen, row) != ansiLine(wantANSI, row) {
		t.Fatal("the trail did not fade to the ordinary cursor")
	}

	pressKeys(t, still, "J")
	pressKeys(t, moving, "J")
	row = cursorRow(t, moving)
	if ansiLine(moving.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, row) ==
		ansiLine(still.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, row) {
		t.Fatal("a repeated J did not light the trail again")
	}
}

func TestMoveTrailRetiresOnTheNextKeystroke(t *testing.T) {
	still := motionFixture(t, false, 3, 0)
	pressKeys(t, still, "J", "d")
	want := still.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen
	moving := motionFixture(t, true, 3, 0)
	pressKeys(t, moving, "J", "d")
	row := cursorRow(t, moving)
	if got := moving.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen; ansiLine(got, row) != ansiLine(want, row) {
		t.Fatal("a keystroke left the trail lit")
	}
}

func ansiLine(screen string, row int) string {
	lines := strings.Split(screen, "\n")
	if row < 0 || row >= len(lines) {
		return ""
	}
	return lines[row]
}

// Reduce Motion, the Motion preference, and a launch that never armed motion
// all leave the three animations off: each key draws its settled frame at once.
func TestMotionOffDisablesScrollRevealAndTrail(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inbox   int
		backlog int
		keys    []string
	}{
		{name: "scroll", inbox: 40, keys: []string{"G"}},
		{name: "reveal", inbox: 3, backlog: 6, keys: []string{"s"}},
		{name: "trail", inbox: 3, keys: []string{"J"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			still := motionFixture(t, false, tc.inbox, tc.backlog)
			pressKeys(t, still, tc.keys...)
			want := still.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen
			for _, off := range []func(*UITestDriver){reduceMotion, func(d *UITestDriver) {
				d.App().lifecycle.motionOff = true
				d.App().applyMotion()
			}} {
				driver := motionFixture(t, true, tc.inbox, tc.backlog)
				off(driver)
				pressKeys(t, driver, tc.keys...)
				if got := driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen; got != want {
					t.Fatalf("motion off still animated:\n%s\nwant:\n%s", got, want)
				}
			}
		})
	}
}
