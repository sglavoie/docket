package tui

import (
	"strings"
	"testing"
)

// Tab draws the mode it reached arriving for one phase, and the phase ends on
// its own. Without motion the title never changes paint.
func TestModeSwitchArrivesThenSettles(t *testing.T) {
	titleOf := func(driver *UITestDriver) string {
		obs := driver.Observe(ObserveOptions{IncludeANSI: true})
		return strings.SplitN(obs.ANSIScreen, "\n", 2)[0]
	}
	still := launchReadmeFixture(t)
	pressKeys(t, still, "tab")
	settledTitle := titleOf(still)

	driver := launchReadmeFixture(t)
	WithMotion()(driver.App())
	pressKeys(t, driver, "tab")
	if arriving := titleOf(driver); arriving == settledTitle {
		t.Fatal("Tab drew no arrival on the title")
	}
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionWait, Duration: "250ms"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	if settled := titleOf(driver); settled != settledTitle {
		t.Fatalf("arrival outlived its phase:\n%q\nwant\n%q", settled, settledTitle)
	}
}
