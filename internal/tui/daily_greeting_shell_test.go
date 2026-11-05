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

// The first greeting of a local week recaps the week before from the completion
// history; a later greeting the same week, even on a new day, does not.
func TestDailyGreetingRecapsTheWeekBeforeOnTheFirstGreetingOfAWeek(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	wednesday := time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local)
	lastWeek := func(offset int) time.Time { return time.Date(2026, 10, 26+offset, 0, 0, 0, 0, time.Local) }
	app.completionActivity.DailyCompletions = map[time.Time]int{lastWeek(0): 2, lastWeek(3): 5}

	greetAt := func(greetedOn string, now time.Time) string {
		machine := *app.domain.runtime.Machine()
		machine.GreetedOn = greetedOn
		if err := app.domain.runtime.CommitMachine(&machine); err != nil {
			t.Fatal(err)
		}
		app.lifecycle.greets = true
		app.greetLocalDay(now)
		return driver.Observe(ObserveOptions{}).PlainScreen
	}

	screen := greetAt("2026-10-30", wednesday)
	if !greeted(screen) || !strings.Contains(screen, "last week 7 done, most on Thursday") {
		t.Fatalf("first greeting of the week did not recap the week before:\n%s", screen)
	}
	screen = greetAt("2026-11-03", wednesday)
	if !greeted(screen) || strings.Contains(screen, "last week") {
		t.Fatalf("a later greeting in the same week recapped again:\n%s", screen)
	}
}
