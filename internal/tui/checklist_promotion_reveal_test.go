package tui

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// This file is the reveal half of `S`. Promotion is the one mutation whose
// gesture destroys the row it was made on: the item is consumed, so the cursor
// has nowhere to stay and the committed child is the only thing left to point
// at. Every case here asks the same two questions of a different projection —
// did the child land where the user is looking, and did anything else move.

// expandChecklistOf opens one task's checklist and returns its owner identity,
// which is the setup every promotion needs: `S` answers from an item row, and an
// item row exists only while its owner's checklist is expanded.
func expandChecklistOf(t *testing.T, view *outliner.OutlinerModel, title string) taskref.TaskIdentity {
	t.Helper()
	seatCursorOnTaskTitled(t, view, title)
	identity := view.TaskIdentityForRow(view.Rows[view.Cursor])
	view.Folds.SetChecklist(identity, true)
	view.RebuildRows()
	return identity
}

// seatCursorOnChecklistItem parks the cursor on the item row carrying text.
// Walking the projection rather than counting rows keeps the assertion about the
// item the user is looking at instead of about how the fixture happens to sort.
func seatCursorOnChecklistItem(t *testing.T, view *outliner.OutlinerModel, text string) {
	t.Helper()
	for index, row := range view.Rows {
		if !row.IsChecklistItem || row.Task == nil {
			continue
		}
		if row.Task.Checklist[row.ChecklistIndex].Text == text {
			view.Cursor = index
			return
		}
	}
	t.Fatalf("no checklist item row carrying %q in the projection", text)
}

// promoteAtCursor presses the one key this file is about and drains what it
// queued, without advancing the clock. A promotion whose child the view hides
// answers in a transient status, and the driver's wait spends exactly the two
// seconds that status lives for, so settling here would throw away the one thing
// several of these cases exist to read.
func promoteAtCursor(t *testing.T, driver *UITestDriver) {
	t.Helper()
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "S"}})
}

// workspaceTaskTitled is the committed child itself, read out of the reloaded
// workspace rather than out of the projection. A Done child is hidden by the
// default view, so a test that could only see rows could not tell a promotion
// that wrote nothing from one whose result the view declines to draw.
func workspaceTaskTitled(t *testing.T, app *AppModel, title string) (*models.Task, string) {
	t.Helper()
	for _, project := range app.domain.projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.Title == title {
				return task, project.ID
			}
		}
	}
	t.Fatalf("no task titled %q in the loaded workspace", title)
	return nil, ""
}

// cursorTaskTitle names the row the cursor rests on, or says what kind of row it
// is when that row names no task. Every reveal assertion ends here.
func cursorTaskTitle(t *testing.T, view *outliner.OutlinerModel) string {
	t.Helper()
	if view.Cursor < 0 || view.Cursor >= len(view.Rows) {
		t.Fatalf("cursor %d is outside the %d projected rows", view.Cursor, len(view.Rows))
	}
	row := view.Rows[view.Cursor]
	switch {
	case row.IsChecklistItem && row.Task != nil:
		return "checklist item " + row.Task.Checklist[row.ChecklistIndex].Text
	case row.IsTaskRow() && row.Task != nil:
		return row.Task.Title
	case row.IsProjectHeader:
		return "<project header>"
	case row.IsSectionHeader:
		return "<section header>"
	default:
		return "<unknown row>"
	}
}

// TestChecklistPromotionRevealsTheChildUnderItsOwner is the ordinary case: one
// unchecked item in the middle of a checklist becomes a Backlog child of the task
// that held it, the checklist keeps the two items around it in order, and the
// cursor lands on the child rather than on whatever row inherited the index the
// item used to occupy.
func TestChecklistPromotionRevealsTheChildUnderItsOwner(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	owner := expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)

	promoteAtCursor(t, driver)

	view = driver.App().crossOutliner
	want := []string{checklistFixtureFirstItem, checklistFixtureThirdItem}
	if got := checklistItemTexts(t, view, checklistFixtureFoldTitle); !slices.Equal(got, want) {
		t.Fatalf("the owner's checklist is %q, want the promoted item gone from %q", got, want)
	}

	child, projectID := workspaceTaskTitled(t, driver.App(), checklistFixtureSecondItem)
	if projectID != owner.ProjectID {
		t.Errorf("the child landed in project %q, want the owner's %q", projectID, owner.ProjectID)
	}
	if child.ParentID == nil || *child.ParentID != owner.TaskID {
		t.Errorf("the child's parent is %v, want the owner %q", child.ParentID, owner.TaskID)
	}
	if !child.IsInState(models.StateBacklog) {
		t.Errorf("an unchecked item became a %s child, want Backlog", child.State())
	}
	if len(child.Checklist) != 0 || len(child.Tags) != 0 || child.Description != "" {
		t.Errorf("the child carried metadata down from its owner: %+v", child)
	}

	if got := cursorTaskTitle(t, view); got != checklistFixtureSecondItem {
		t.Fatalf("the cursor rests on %q, want the committed child %q", got, checklistFixtureSecondItem)
	}
	if !strings.Contains(driver.Render(), checklistFixtureSecondItem) {
		t.Errorf("the committed child is not on screen:\n%s", driver.Render())
	}
}

// TestChecklistPromotionUndoRestoresTheItemAtItsPosition holds promotion to one
// ledger entry. The item and the child are two halves of one transaction, so a
// single `u` has to take both back — the child gone and the item returned to the
// middle of the list, not appended to the end of it — and `Ctrl+R` has to put
// them both back again.
func TestChecklistPromotionUndoRestoresTheItemAtItsPosition(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	before := checklistItemTexts(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)

	promoteAtCursor(t, driver)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "u"},
		{Type: UITestActionWait},
	})

	view = driver.App().crossOutliner
	if got := checklistItemTexts(t, view, checklistFixtureFoldTitle); !slices.Equal(got, before) {
		t.Fatalf("one undo left the checklist at %q, want the %q it started from", got, before)
	}
	if childSurvives(driver.App(), checklistFixtureSecondItem) {
		t.Error("undo took the checklist item back but left the child task behind")
	}

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+r"},
		{Type: UITestActionWait},
	})
	view = driver.App().crossOutliner
	want := []string{checklistFixtureFirstItem, checklistFixtureThirdItem}
	if got := checklistItemTexts(t, view, checklistFixtureFoldTitle); !slices.Equal(got, want) {
		t.Fatalf("redo left the checklist at %q, want %q", got, want)
	}
	if !childSurvives(driver.App(), checklistFixtureSecondItem) {
		t.Error("redo restored the shortened checklist without the child it paid for")
	}
}

// childSurvives reports whether a task with this title is anywhere in the loaded
// workspace, which is how undo is read: a ledger entry that took the item back
// and left the task is two entries pretending to be one.
func childSurvives(app *AppModel, title string) bool {
	for _, project := range app.domain.projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.Title == title {
				return true
			}
		}
	}
	return false
}

// TestChecklistPromotionOpensTheOwningBranchAndNoOther is the fold contract. A
// collapsed owner is the one way a committed child is missing from a projection
// that would otherwise admit it, so the reveal opens that branch — and only that
// branch, leaving every unrelated fold exactly where the user left it.
func TestChecklistPromotionOpensTheOwningBranchAndNoOther(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	// The owner's subtree is collapsed before its checklist is opened, because the
	// fold ladder sheds the checklist first: collapsing afterwards would take away
	// the item row the gesture needs.
	seatCursorOnTaskTitled(t, view, checklistFixtureParentTitle)
	owner := view.TaskIdentityForRow(view.Rows[view.Cursor])
	if !view.CollapseOrMoveToParent() {
		t.Fatal("the fixture parent's subtree could not be collapsed")
	}
	if view.IsExpanded(owner) {
		t.Fatal("collapsing the owner left its branch open")
	}
	expandChecklistOf(t, view, checklistFixtureParentTitle)
	// A second task's open checklist is the unrelated fold the reveal must not
	// spend, on the one axis a promotion has every reason to touch.
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	sections := maps.Clone(view.Folds.CollapsedSections)
	checklists := maps.Clone(view.Folds.ExpandedChecklists)
	if len(checklists) != 2 {
		t.Fatalf("checklist folds = %v, want both fixture checklists open", checklists)
	}
	// The one write the reveal is allowed: the owner's own branch, opened.
	nodes := maps.Clone(view.Folds.ExpandedNodes)
	nodes[owner] = true

	seatCursorOnChecklistItem(t, view, "Book the maintenance window")
	promoteAtCursor(t, driver)

	view = driver.App().crossOutliner
	if !view.IsExpanded(owner) {
		t.Error("the reveal left the owner's branch collapsed, so the child it committed is invisible")
	}
	if got := view.Folds.CollapsedSections; !maps.Equal(got, sections) {
		t.Errorf("the reveal rewrote the section folds to %v, want the untouched %v", got, sections)
	}
	if got := view.Folds.ExpandedChecklists; !maps.Equal(got, checklists) {
		t.Errorf("the reveal rewrote the checklist folds to %v, want the untouched %v", got, checklists)
	}
	if got := view.Folds.ExpandedNodes; !maps.Equal(got, nodes) {
		t.Errorf("the reveal left the subtree folds at %v, want %v — its owner's branch and nothing else",
			got, nodes)
	}
	if got := cursorTaskTitle(t, view); got != "Book the maintenance window" {
		t.Fatalf("the cursor rests on %q, want the committed child", got)
	}
}

// TestChecklistPromotionNamesAChildThisViewHides is the honest half of a narrow
// reveal. The query is the user's, so it is not rewritten to make the child
// visible: the cursor returns to the owner the item came from and the status
// names the child, which is the only way to tell a hidden write from no write.
func TestChecklistPromotionNamesAChildThisViewHides(t *testing.T) {
	driver, _, _ := checklistFixtureApp(t)
	// "runbook" matches the owner's title and neither of the items under it, so
	// the projection admits the checklist and refuses whatever it promotes.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "/"},
		{Type: UITestActionText, Text: "runbook"},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	view := driver.App().crossOutliner
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)

	promoteAtCursor(t, driver)

	view = driver.App().crossOutliner
	if child, _ := workspaceTaskTitled(t, driver.App(), checklistFixtureSecondItem); !child.IsInState(models.StateBacklog) {
		t.Fatalf("the hidden child is in state %s, want the Backlog child a narrow view simply excludes", child.State())
	}
	if got := cursorTaskTitle(t, view); got != checklistFixtureFoldTitle {
		t.Errorf("the cursor rests on %q, want the owner the hidden child came from", got)
	}
	if got := view.StatusMessage; !strings.Contains(got, checklistFixtureSecondItem) ||
		!strings.Contains(got, "hidden") {
		t.Errorf("status = %q, want it to name the hidden child %q", got, checklistFixtureSecondItem)
	}
}

// TestChecklistPromotionNamesACompletedChildTerminalWorkHides is the same
// refusal with a worse failure mode. A ticked item becomes a Done task, Done work
// is hidden by default, and a promotion that says nothing would read exactly like
// one that did nothing — so the status says the child was created and completed.
func TestChecklistPromotionNamesACompletedChildTerminalWorkHides(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	owner := expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureFirstItem)

	promoteAtCursor(t, driver)

	child, _ := workspaceTaskTitled(t, driver.App(), checklistFixtureFirstItem)
	if !child.IsInState(models.StateDone) {
		t.Fatalf("a ticked item became a %s child, want Done", child.State())
	}
	if child.ParentID == nil || *child.ParentID != owner.TaskID {
		t.Errorf("the completed child's parent is %v, want the owner %q", child.ParentID, owner.TaskID)
	}

	view = driver.App().crossOutliner
	if got := cursorTaskTitle(t, view); got != checklistFixtureFoldTitle {
		t.Errorf("the cursor rests on %q, want the owner the completed child came from", got)
	}
	status := view.StatusMessage
	if !strings.Contains(status, checklistFixtureFirstItem) {
		t.Errorf("status = %q, want it to name the completed child", status)
	}
	if !strings.Contains(status, "done") {
		t.Errorf("status = %q, want it to say the child was completed", status)
	}
}

// TestChecklistPromotionInsideFocusStaysInsideFocus keeps the reveal off the
// scope axis. Focus is a projection the user chose; the child of a focused task's
// descendant belongs inside it, so the frame must still be standing afterwards
// and the cursor must be on the child rather than back at the top.
func TestChecklistPromotionInsideFocusStaysInsideFocus(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	const soakTitle = "Confirm the staging soak"
	const item = "Replay the smoke suite"

	seatCursorOnTaskTitled(t, view, checklistFixtureParentTitle)
	if !view.FocusSubtreeAtCursor() {
		t.Fatal("the fixture parent could not be focused")
	}
	root, focused := view.FocusRootIdentity()
	if !focused {
		t.Fatal("focusing pushed no frame")
	}

	// The focused subtree's own task needs a checklist before it can have an item
	// row, and `c` is the ordinary way to give it one.
	seatCursorOnTaskTitled(t, view, soakTitle)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "c"},
		{Type: UITestActionText, Text: item},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})

	view = driver.App().crossOutliner
	seatCursorOnChecklistItem(t, view, item)
	promoteAtCursor(t, driver)

	view = driver.App().crossOutliner
	if got, stillFocused := view.FocusRootIdentity(); !stillFocused || got != root {
		t.Fatalf("focus root = %+v (focused=%v), want the untouched %+v", got, stillFocused, root)
	}
	if got := cursorTaskTitle(t, view); got != item {
		t.Fatalf("the cursor rests on %q, want the child %q committed inside Focus", got, item)
	}
	grandchild, _ := workspaceTaskTitled(t, driver.App(), item)
	parent, _ := workspaceTaskTitled(t, driver.App(), soakTitle)
	if grandchild.ParentID == nil || *grandchild.ParentID != parent.ID {
		t.Errorf("the child's parent is %v, want the focused descendant %q", grandchild.ParentID, parent.ID)
	}
}

// TestChecklistPromotionInACrossProjectProjection promotes from the second of
// four project groups. The cross-project outliner resolves a row's owner through
// its ProjectID rather than through a current project, so this is where a reveal
// that assumed one project would seat the cursor in the wrong group — or refuse
// to find the child at all.
func TestChecklistPromotionInACrossProjectProjection(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	const ownerTitle = "Beta rollout checklist"
	const item = "Draft the rollout note"

	seatCursorOnTaskTitled(t, view, ownerTitle)
	owner := view.TaskIdentityForRow(view.Rows[view.Cursor])
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "c"},
		{Type: UITestActionText, Text: item},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})

	view = driver.App().crossOutliner
	seatCursorOnChecklistItem(t, view, item)
	promoteAtCursor(t, driver)

	view = driver.App().crossOutliner
	if got := cursorTaskTitle(t, view); got != item {
		t.Fatalf("the cursor rests on %q, want the committed child %q", got, item)
	}
	if got := view.Rows[view.Cursor].ProjectID; got != owner.ProjectID {
		t.Errorf("the cursor landed in project %q, want the owner's %q", got, owner.ProjectID)
	}
	child, projectID := workspaceTaskTitled(t, driver.App(), item)
	if projectID != owner.ProjectID {
		t.Errorf("the child was written into project %q, want %q", projectID, owner.ProjectID)
	}
	if child.ParentID == nil || *child.ParentID != owner.TaskID {
		t.Errorf("the child's parent is %v, want the owner %q", child.ParentID, owner.TaskID)
	}
}

// TestChecklistPromotionFitsASmallTerminal drives the same gesture at a size
// where the reveal has to scroll rather than simply move an index. The child is
// still committed, the cursor is still on it, and the screen the reveal produced
// still fits the terminal it was drawn for.
func TestChecklistPromotionFitsASmallTerminal(t *testing.T) {
	driver, _, _ := checklistFixtureApp(t)
	driver.Resize(60, 14)
	view := driver.App().crossOutliner
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)

	promoteAtCursor(t, driver)

	view = driver.App().crossOutliner
	if got := cursorTaskTitle(t, view); got != checklistFixtureSecondItem {
		t.Fatalf("the cursor rests on %q, want the committed child", got)
	}
	if observation := driver.Observe(ObserveOptions{}); len(observation.LayoutIssues) > 0 {
		t.Errorf("the revealed child broke the 60x14 layout: %v\n%s",
			observation.LayoutIssues, observation.PlainScreen)
	}
}
