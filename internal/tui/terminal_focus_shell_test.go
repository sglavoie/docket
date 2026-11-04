package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// A window the terminal reports unfocused rests its cursor: the paint changes,
// no character or width does, and focusing it again draws exactly what it drew
// before the blur.
func TestUnfocusedTerminalRestsTheCursor(t *testing.T) {
	driver := launchReadmeFixture(t)
	pressKeys(t, driver, "j", "j")
	focused := driver.Observe(ObserveOptions{IncludeANSI: true})

	driver.dispatch(tea.BlurMsg{})
	driver.drain()
	blurred := driver.Observe(ObserveOptions{IncludeANSI: true})
	if blurred.PlainScreen != focused.PlainScreen {
		t.Fatalf("resting the cursor changed the text:\n%s\nwant\n%s", blurred.PlainScreen, focused.PlainScreen)
	}
	if blurred.ANSIScreen == focused.ANSIScreen {
		t.Fatal("an unfocused window drew its cursor at full strength")
	}

	driver.dispatch(tea.FocusMsg{})
	driver.drain()
	if got := driver.Observe(ObserveOptions{IncludeANSI: true}); got.ANSIScreen != focused.ANSIScreen {
		t.Fatal("focusing the window again did not restore the cursor")
	}
}

// A count that moves while the window is in the background is not lit where
// nobody sees it: the change waits, and the focus that brings the user back
// lights it, measured from the counts as they stood at the blur.
func TestPulseChangeWhileUnfocusedLightsOnReturn(t *testing.T) {
	driver := launchReadmeFixture(t)
	WithMotion()(driver.App())
	pressKeys(t, driver, "j", "j", "j", "j")

	driver.dispatch(tea.BlurMsg{})
	driver.drain()
	pressKeys(t, driver, "w", "t")
	if driver.App().pulseMotion.change.Any() {
		t.Fatal("a count lit while the window was unfocused")
	}

	driver.dispatch(tea.FocusMsg{})
	driver.drain()
	if !driver.App().pulseMotion.change.Any() {
		t.Fatal("returning to the window lit nothing that moved while away")
	}
	if driver.App().pulseMotion.away != nil {
		t.Fatal("the return did not spend the held counts")
	}
}
