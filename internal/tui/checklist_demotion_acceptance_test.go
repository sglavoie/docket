package tui

import (
	"slices"
	"testing"
)

func TestChecklistSConvertsBackAndSelectsOriginalItem(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	promoteAtCursor(t, driver)
	view = driver.App().crossOutliner
	if got := cursorTaskTitle(t, view); got != checklistFixtureSecondItem {
		t.Fatalf("promoted cursor = %q", got)
	}
	promoteAtCursor(t, driver)
	view = driver.App().crossOutliner
	if got := checklistItemTexts(t, view, checklistFixtureFoldTitle); !slices.Equal(got, []string{checklistFixtureFirstItem, checklistFixtureSecondItem, checklistFixtureThirdItem}) {
		t.Fatalf("restored checklist = %v", got)
	}
	if got := cursorTaskTitle(t, view); got != "checklist item "+checklistFixtureSecondItem {
		t.Fatalf("converted cursor = %q", got)
	}
	if childSurvives(driver.App(), checklistFixtureSecondItem) {
		t.Fatal("child survived reverse conversion")
	}
	promoteAtCursor(t, driver)
	if !childSurvives(driver.App(), checklistFixtureSecondItem) {
		t.Fatal("repeated S did not promote the selected checklist item")
	}
}

func TestChecklistSRefusesPlannedChildWithoutMovingCursor(t *testing.T) {
	driver, workspace, view := checklistFixtureApp(t)
	seatCursorOnTaskTitled(t, view, "Confirm the staging soak")
	identity := view.TaskIdentityForRow(view.Rows[view.Cursor])
	before := checklistItemTexts(t, view, checklistFixtureParentTitle)
	planning, err := workspace.Manager.DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatal(err)
	}
	planning.AddToNext(identity.TaskID, identity.ProjectID)
	if err := workspace.Manager.CommitPlanning(planning); err != nil {
		t.Fatal(err)
	}
	cursor := view.Cursor
	promoteAtCursor(t, driver)
	view = driver.App().crossOutliner
	if driver.App().feedback.err == nil || view.Cursor != cursor {
		t.Fatalf("planned child refusal=%v cursor=%d want=%d", driver.App().feedback.err, view.Cursor, cursor)
	}
	if got := checklistItemTexts(t, view, checklistFixtureParentTitle); !slices.Equal(got, before) {
		t.Fatalf("checklist changed: %v", got)
	}
	if !childSurvives(driver.App(), "Confirm the staging soak") {
		t.Fatal("planned child was removed")
	}
}
