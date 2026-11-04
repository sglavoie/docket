package tui

import "testing"

// Planning a task for Today lights the count that moved, settles it, then
// leaves the title exactly as a session without motion draws it. The launch's
// own first reading lights nothing.
func TestPulseCountChangeLightsThenSettles(t *testing.T) {
	still := launchReadmeFixture(t)
	pressKeys(t, still, "j", "j", "j", "j", "w", "t")
	settledTitle := titleRowANSI(still)

	driver := launchReadmeFixture(t)
	WithMotion()(driver.App())
	if driver.App().pulseMotion.change.Any() {
		t.Fatal("the launch lit the pulse")
	}
	pressKeys(t, driver, "j", "j", "j", "j", "w", "t")
	if !driver.App().pulseMotion.change.Any() {
		t.Fatal("planning for Today lit no count")
	}
	fresh := titleRowANSI(driver)
	if fresh == settledTitle {
		t.Fatal("the changed count drew nothing")
	}
	waitFor(t, driver, "300ms")
	settling := titleRowANSI(driver)
	if settling == fresh || settling == settledTitle {
		t.Fatal("the count skipped its settling phase")
	}
	waitFor(t, driver, "500ms")
	if got := titleRowANSI(driver); got != settledTitle {
		t.Fatalf("the lit count outlived its phases:\n%q\nwant\n%q", got, settledTitle)
	}
}
