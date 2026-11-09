package tui

import (
	"strings"
	"testing"
)

// Capture opened from Today plans the draft for Today, so the new row lands on
// the screen it was typed over; the route row says so before Enter.
func TestCaptureFromTodayPlansTheDraftForToday(t *testing.T) {
	driver := launchReadmeFixture(t)
	obs := pressKeys(t, driver, "g", "t", "a")
	if !strings.Contains(obs.PlainScreen, "Today") {
		t.Fatalf("capture from Today did not name the plan:\n%s", obs.PlainScreen)
	}
	sendText(driver, "Planned from Today")
	pressKeys(t, driver, "enter")
	app := driver.App()
	found := false
	for identity := range app.todayIDs {
		if task, _ := app.taskAndProjectForIdentity(identity); task != nil && task.Title == "Planned from Today" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a task captured from Today was not planned for Today; status %q", app.feedback.message)
	}
}

// A typed destination still replaces the default.
func TestCaptureFromTodayKeepsATypedDestination(t *testing.T) {
	driver := launchReadmeFixture(t)
	pressKeys(t, driver, "g", "t", "a")
	sendText(driver, "Parked for later $paused")
	pressKeys(t, driver, "enter")
	app := driver.App()
	for identity := range app.todayIDs {
		if task, _ := app.taskAndProjectForIdentity(identity); task != nil && task.Title == "Parked for later" {
			t.Fatal("a typed $paused was planned for Today anyway")
		}
	}
}

// Anywhere else capture still lands in Inbox, unplanned.
func TestCaptureFromAllStaysInInbox(t *testing.T) {
	driver := launchReadmeFixture(t)
	pressKeys(t, driver, "a")
	sendText(driver, "Unplanned capture")
	pressKeys(t, driver, "enter")
	app := driver.App()
	for identity := range app.todayIDs {
		if task, _ := app.taskAndProjectForIdentity(identity); task != nil && task.Title == "Unplanned capture" {
			t.Fatal("a capture from All was planned for Today")
		}
	}
}
