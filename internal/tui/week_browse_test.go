package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

func launchWeekBrowseFixture(t *testing.T) (*UITestDriver, string) {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	var dueID string
	if _, err = workspace.addProject("Week Fixture", func(project *models.Project) {
		task := newFixtureTask(project, "Ship the quarterly letter")
		task.ApplyWorkflowState(models.StateBacklog)
		due := time.Now().AddDate(0, 0, 3)
		due = time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.Local)
		task.DueDate = &due
		dueID = task.ID
	}); err != nil {
		t.Fatalf("add project: %v", err)
	}
	driver, err := workspace.launch(100, 35)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	return driver, dueID
}

// gw over Today selects the first dated day of the strip and lists what it
// holds; the day keys move the selection and Esc puts it down.
func TestWeekBrowseSelectsADayAndListsItsWork(t *testing.T) {
	driver, _ := launchWeekBrowseFixture(t)
	obs := pressKeys(t, driver, "g", "t", "g", "w")
	if !driver.App().crossOutliner.Week.Active {
		t.Fatalf("gw did not open the week browse; status %q\n%s", driver.App().feedback.message, obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "● Ship the quarterly letter %week-fixture") {
		t.Fatalf("the selected day does not list its deadline:\n%s", obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "←/→ day") {
		t.Fatalf("the footer does not name the browse's keys:\n%s", obs.PlainScreen)
	}
	obs = pressKeys(t, driver, "left")
	if strings.Contains(obs.PlainScreen, "● Ship the quarterly letter") {
		t.Fatalf("moving off the day kept its list:\n%s", obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "Nothing is dated on") {
		t.Fatalf("an empty day says nothing:\n%s", obs.PlainScreen)
	}
	pressKeys(t, driver, "esc")
	if driver.App().crossOutliner.Week.Active {
		t.Fatal("Esc did not close the week browse")
	}
}

// Enter leaves for Due with the cursor on the selected day's first task.
func TestWeekBrowseEnterOpensTheDayInDue(t *testing.T) {
	driver, dueID := launchWeekBrowseFixture(t)
	pressKeys(t, driver, "g", "t", "g", "w", "enter")
	out := driver.App().crossOutliner
	if out.PrimaryMode != chrome.ModeDue {
		t.Fatalf("Enter left the browse in %v, want Due", out.PrimaryMode)
	}
	if task, _ := driver.App().getCurrentCursorTaskAndProject(); task == nil || task.ID != dueID {
		t.Fatalf("the cursor is not on the selected day's task: %+v", task)
	}
}

// Outside Today the browse refuses by naming where it lives.
func TestWeekBrowseRefusesOutsideToday(t *testing.T) {
	driver, _ := launchWeekBrowseFixture(t)
	pressKeys(t, driver, "g", "w")
	if driver.App().crossOutliner.Week.Active {
		t.Fatal("gw opened the browse outside Today")
	}
	if !strings.Contains(driver.App().feedback.message, "drawn in Today") {
		t.Fatalf("refusal = %q", driver.App().feedback.message)
	}
}
