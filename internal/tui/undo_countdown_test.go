package tui

import "testing"

// The undo window drains across the footer divider: whole when the toast
// arrives, about half way through its window at half, and gone when `u` no
// longer answers.
func TestUndoCountdownDrainsWithTheUndoWindow(t *testing.T) {
	driver := launchReadmeFixture(t)
	WithMotion()(driver.App())
	pressKeys(t, driver, "j", "j", "j", "j", "w", "d")
	app := driver.App()
	const width = 100
	if got := app.undoCountdownCells(width); got != width {
		t.Fatalf("fresh undo window holds %d of %d cells", got, width)
	}
	wait := func(duration string) {
		t.Helper()
		if _, err := driver.ApplyAction(UITestAction{Type: UITestActionWait, Duration: duration}, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	wait("3s")
	if got := app.undoCountdownCells(width); got < 40 || got > 60 {
		t.Fatalf("half-way undo window holds %d of %d cells", got, width)
	}
	wait("3100ms")
	if got := app.undoCountdownCells(width); got != 0 {
		t.Fatalf("closed undo window still holds %d cells", got)
	}
}

func TestUndoCountdownNeedsMotion(t *testing.T) {
	driver := launchReadmeFixture(t)
	pressKeys(t, driver, "j", "j", "j", "j", "w", "d")
	if got := driver.App().undoCountdownCells(100); got != 0 {
		t.Fatalf("a session without motion drew a countdown of %d cells", got)
	}
}
