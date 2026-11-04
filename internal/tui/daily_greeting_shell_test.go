package tui

import (
	"strings"
	"testing"
	"time"
)

// The first interactive launch of a local day opens with the greeting, records
// the day machine-locally, and the next launch that day says nothing.
func TestDailyGreetingOpensTheFirstLaunchOfADayOnly(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	if greeted(driver.Observe(ObserveOptions{}).PlainScreen) {
		t.Fatal("a launch without the greeting option greeted")
	}

	driver.launchOptions = append(driver.launchOptions, WithDailyGreeting())
	if err := driver.Restart(); err != nil {
		t.Fatal(err)
	}
	screen := driver.Observe(ObserveOptions{}).PlainScreen
	if !greeted(screen) {
		t.Fatalf("first launch of the day did not greet:\n%s", screen)
	}
	if got, want := driver.App().domain.runtime.Machine().GreetedOn, time.Now().Format(time.DateOnly); got != want {
		t.Fatalf("greeted_on = %q, want %q", got, want)
	}

	if err := driver.Restart(); err != nil {
		t.Fatal(err)
	}
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; greeted(screen) {
		t.Fatalf("second launch the same day greeted again:\n%s", screen)
	}
}

func greeted(screen string) bool {
	for _, salutation := range []string{"Good morning · ", "Good afternoon · ", "Good evening · ", "Working late · "} {
		if strings.Contains(screen, salutation) {
			return true
		}
	}
	return false
}
