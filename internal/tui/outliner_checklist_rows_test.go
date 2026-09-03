package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// A checklist row carries its owner's task pointer so it can render, which is
// exactly why it must fail isTaskRow: every task operation resolves its target
// through that predicate and would otherwise silently act on the owner. It stays
// navigable because the cursor has to be able to land on it.
func TestChecklistRowIsNotATaskRowButStaysNavigable(t *testing.T) {
	owner := outlinerRow{task: &models.Task{}}
	if !owner.isTaskRow() || !owner.isNavigable() {
		t.Fatalf("owner row: isTaskRow=%v isNavigable=%v, want true/true",
			owner.isTaskRow(), owner.isNavigable())
	}

	item := owner
	item.IsChecklistItem = true
	item.ChecklistIndex = 1
	item.ChecklistDone = true
	if item.isTaskRow() {
		t.Fatal("checklist row reports isTaskRow, so task operations would target its owner")
	}
	if !item.isNavigable() {
		t.Fatal("checklist row is not navigable, so the cursor could never reach it")
	}
}

func TestChecklistFoldLadderExpandsAndCollapses(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("NewFixtureDriver(checklist) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	outliner := driver.App().crossOutliner
	focusChecklistOwner(t, driver, checklistFixtureFoldTitle)

	if screen := driver.Observe(ObserveOptions{}).PlainScreen; strings.Contains(screen, checklistFixtureFirstItem) {
		t.Fatalf("checklist items render before any expansion:\n%s", screen)
	}

	if !outliner.expandAtCursor() {
		t.Fatal("expandAtCursor did not expand a checklist-only task")
	}
	items := checklistRowTexts(outliner)
	want := []string{checklistFixtureFirstItem, checklistFixtureSecondItem, checklistFixtureThirdItem}
	if len(items) != len(want) {
		t.Fatalf("checklist rows = %v, want %v", items, want)
	}
	for index, text := range want {
		if items[index] != text {
			t.Fatalf("checklist row %d = %q, want %q", index, items[index], text)
		}
	}
	if !outliner.rows[outliner.cursor].isTaskRow() {
		t.Fatal("expanding moved the cursor off the owner task row")
	}

	if !outliner.collapseOrMoveToParent() {
		t.Fatal("collapseOrMoveToParent did not collapse the expanded checklist")
	}
	if got := checklistRowTexts(outliner); len(got) != 0 {
		t.Fatalf("checklist rows survive collapse: %v", got)
	}
}

// h on a checklist row returns to its owner rather than collapsing, so the
// checklist is one rung below the task in the same ladder.
func TestChecklistRowCollapseReturnsToOwner(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("NewFixtureDriver(checklist) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	outliner := driver.App().crossOutliner
	owner := focusChecklistOwner(t, driver, checklistFixtureFoldTitle)
	if !outliner.expandAtCursor() {
		t.Fatal("expandAtCursor did not expand the checklist")
	}

	outliner.cursor++
	if !outliner.rows[outliner.cursor].IsChecklistItem {
		t.Fatalf("row after the owner is not a checklist item: %+v", outliner.rows[outliner.cursor])
	}
	if !outliner.collapseOrMoveToParent() {
		t.Fatal("collapseOrMoveToParent on a checklist row did not move the cursor")
	}
	if got := outliner.taskIdentityForRow(outliner.rows[outliner.cursor]); got != owner {
		t.Fatalf("cursor identity = %+v, want the owner %+v", got, owner)
	}
	if len(checklistRowTexts(outliner)) == 0 {
		t.Fatal("returning to the owner also collapsed the checklist")
	}
}

// Children come before the checklist on the way out and the checklist folds
// first on the way back, so l/h are exact inverses on a task holding both.
func TestChecklistFoldOrdersChildrenBeforeItems(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("NewFixtureDriver(checklist) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	outliner := driver.App().crossOutliner
	identity := focusChecklistOwner(t, driver, checklistFixtureParentTitle)
	if outliner.isExpanded(identity) {
		outliner.collapseOrMoveToParent()
		focusChecklistOwner(t, driver, checklistFixtureParentTitle)
	}

	outliner.expandAtCursor()
	if !outliner.isExpanded(identity) {
		t.Fatal("the first expansion did not open the child subtree")
	}
	if outliner.isChecklistExpanded(identity) {
		t.Fatal("the first expansion opened the checklist before the children")
	}
	outliner.expandAtCursor()
	if !outliner.isChecklistExpanded(identity) {
		t.Fatal("the second expansion did not open the checklist")
	}

	outliner.collapseOrMoveToParent()
	if !outliner.isExpanded(identity) || outliner.isChecklistExpanded(identity) {
		t.Fatal("the first collapse did not shed the checklist first")
	}
	outliner.collapseOrMoveToParent()
	if outliner.isExpanded(identity) {
		t.Fatal("the second collapse did not close the child subtree")
	}
}

// Visual selection names task rows only, so entering it hides checklist rows and
// re-seats the cursor on the owner. The fold state itself survives, which is
// what lets Esc restore the exact rows the user was reading.
func TestChecklistRowsSuppressedDuringVisualSelection(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("NewFixtureDriver(checklist) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	outliner := driver.App().crossOutliner
	owner := focusChecklistOwner(t, driver, checklistFixtureFoldTitle)
	outliner.expandAtCursor()
	outliner.cursor++
	if !outliner.rows[outliner.cursor].IsChecklistItem {
		t.Fatal("the cursor is not on a checklist row before entering selection")
	}

	outliner.toggleMultiSelectMode()
	if got := checklistRowTexts(outliner); len(got) != 0 {
		t.Fatalf("checklist rows survive visual selection: %v", got)
	}
	if !outliner.isChecklistExpanded(owner) {
		t.Fatal("visual selection discarded the checklist fold state")
	}
	if got := outliner.taskIdentityForRow(outliner.rows[outliner.cursor]); got != owner {
		t.Fatalf("cursor identity = %+v, want the re-seated owner %+v", got, owner)
	}

	outliner.toggleMultiSelectMode()
	if got := checklistRowTexts(outliner); len(got) != 3 {
		t.Fatalf("leaving visual selection restored %v, want the three checklist rows", got)
	}
}

func focusChecklistOwner(t *testing.T, driver *UITestDriver, title string) taskref.TaskIdentity {
	t.Helper()
	outliner := driver.App().crossOutliner
	for index, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil && row.task.Title == title {
			outliner.cursor = index
			return outliner.taskIdentityForRow(row)
		}
	}
	t.Fatalf("checklist fixture has no visible task row titled %q", title)
	return taskref.TaskIdentity{}
}

func checklistRowTexts(outliner *OutlinerModel) []string {
	texts := make([]string, 0, len(outliner.rows))
	for _, row := range outliner.rows {
		if row.IsChecklistItem && row.task != nil {
			texts = append(texts, row.task.Checklist[row.ChecklistIndex].Text)
		}
	}
	return texts
}
