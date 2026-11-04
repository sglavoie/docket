package tui

import "testing"

func roomyLaunch(t *testing.T, fixture string, width, height int) *UITestDriver {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(fixture, width, height)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	driver.launchOptions = append(driver.launchOptions, WithRoomyLaunchPeek())
	if err := driver.Restart(); err != nil {
		t.Fatal(err)
	}
	return driver
}

// A launch whose projection leaves the dock its whole height opens Peek on the
// task the cursor landed on, and `b` closes it as ever.
func TestRoomyLaunchOpensPeek(t *testing.T) {
	driver := roomyLaunch(t, FixtureRich, 100, 60)
	view := driver.App().crossOutliner
	if !view.PeekDockVisible() {
		t.Fatalf("a roomy launch left Peek shut:\n%s", driver.Observe(ObserveOptions{}).PlainScreen)
	}
	if obs := driver.Observe(ObserveOptions{}); len(obs.LayoutIssues) != 0 {
		t.Fatalf("layout issues: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
	pressKeys(t, driver, "b")
	if view.PeekDockVisible() {
		t.Fatal("b did not close the launch's Peek")
	}
}

// A projection that would have to scroll under the dock keeps its rows.
func TestCrowdedLaunchKeepsPeekShut(t *testing.T) {
	driver := roomyLaunch(t, FixtureBusy, 100, 20)
	view := driver.App().crossOutliner
	if len(view.Rows) <= 12 {
		t.Fatalf("the busy fixture draws only %d rows, which proves nothing about crowding", len(view.Rows))
	}
	if view.PeekDockVisible() {
		t.Fatalf("a crowded launch opened Peek:\n%s", driver.Observe(ObserveOptions{}).PlainScreen)
	}
}
