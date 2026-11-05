package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func ansiLines(driver *UITestDriver) []string {
	return strings.Split(driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, "\n")
}

// Tab eases the new mode's body in: its first frame draws the settled text in
// fainter colors, the title and footer stay exactly as settled, and the fade
// ends on its own frames. Without motion nothing fades.
func TestModeSwitchFadesTheBodyIn(t *testing.T) {
	still := launchReadmeFixture(t)
	pressKeys(t, still, "tab")
	settled := ansiLines(still)
	if still.App().viewFade.remaining != 0 {
		t.Fatal("a session without motion started a fade")
	}

	driver := launchReadmeFixture(t)
	WithMotion()(driver.App())
	pressKeys(t, driver, "tab")
	fading := ansiLines(driver)
	if len(fading) != len(settled) {
		t.Fatalf("the fade changed the frame's height: %d rows, want %d", len(fading), len(settled))
	}
	footerTop := surfaceFooterDividerIndex(fading)
	if footerTop < 0 {
		footerTop = len(fading) - 2
	}
	changed := false
	for row := range fading {
		if ansi.Strip(fading[row]) != ansi.Strip(settled[row]) {
			t.Fatalf("row %d changed text while fading:\n%q\nwant\n%q", row, ansi.Strip(fading[row]), ansi.Strip(settled[row]))
		}
		body := row >= viewFadeTitleRows && row < footerTop
		// The title is the mode arrival's to draw, so only the footer is
		// compared byte for byte here.
		if row >= footerTop && fading[row] != settled[row] {
			t.Fatalf("row %d is outside the body but faded", row)
		}
		changed = changed || (body && fading[row] != settled[row])
	}
	if !changed {
		t.Fatal("Tab drew no fade on the body")
	}

	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionWait, Duration: "500ms"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	if driver.App().viewFade.remaining != 0 {
		t.Fatalf("fade outlived its frames: %d left", driver.App().viewFade.remaining)
	}
	for row, line := range ansiLines(driver)[viewFadeTitleRows:footerTop] {
		if line != settled[row+viewFadeTitleRows] {
			t.Fatalf("body row %d did not settle on its own colors", row+viewFadeTitleRows)
		}
	}
}

// Opening another surface fades it in, and so does returning from it; a cursor
// move is not a switch, and neither is capture, whose panel opens under a host
// that stays drawn.
func TestViewSwitchFadesButCursorAndCaptureDoNot(t *testing.T) {
	driver := launchReadmeFixture(t)
	WithMotion()(driver.App())
	pressKeys(t, driver, "j")
	if driver.App().viewFade.remaining != 0 {
		t.Fatal("moving the cursor started a fade")
	}
	pressKeys(t, driver, "P")
	if names := driver.viewStackNames(); names[len(names)-1] != "ProjectHub" || driver.App().viewFade.remaining == 0 {
		t.Fatalf("opening the project hub drew no fade (stack %v)", names)
	}
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionWait, Duration: "500ms"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	pressKeys(t, driver, "esc")
	if driver.App().viewFade.remaining == 0 {
		t.Fatal("returning to the Outliner drew no fade")
	}
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionWait, Duration: "500ms"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	pressKeys(t, driver, "a")
	if driver.App().viewFade.remaining != 0 {
		t.Fatalf("opening capture faded its host (stack %v)", driver.viewStackNames())
	}
}

// The next key finishes the fade at once, as it finishes every other motion;
// a key that switches again starts a fresh one.
func TestNextKeyFinishesTheViewFade(t *testing.T) {
	driver := launchReadmeFixture(t)
	WithMotion()(driver.App())
	pressKeys(t, driver, "tab", "j")
	if driver.App().viewFade.remaining != 0 {
		t.Fatal("a key left the previous switch fading")
	}
	pressKeys(t, driver, "tab")
	if driver.App().viewFade.remaining != viewFadeFrames {
		t.Fatal("a second switch did not start its own fade")
	}
}

// Turning motion off drops a fade already under way.
func TestMotionOffStopsTheViewFade(t *testing.T) {
	driver := launchReadmeFixture(t)
	WithMotion()(driver.App())
	pressKeys(t, driver, "tab")
	if driver.App().viewFade.remaining == 0 {
		t.Fatal("Tab started no fade")
	}
	app := driver.App()
	app.lifecycle.hostReducesMotion = true
	app.applyMotion()
	if app.viewFade.remaining != 0 || app.viewFadeReveal() != 1 {
		t.Fatal("Reduce Motion left the fade running")
	}
}
