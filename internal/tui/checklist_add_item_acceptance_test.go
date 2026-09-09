package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// checklistFixtureApp launches the checklist fixture and returns the driver, the
// workspace behind it, and the Outliner the assertions read. The checklist
// fixture is the one that carries every owner shape `c` has to answer from: a
// task with a populated checklist, a task that also has a child, that child, and
// a task with no checklist at all.
func checklistFixtureApp(t *testing.T) (*UITestDriver, *uitestWorkspace, *outliner.OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatal("fixture app has no Outliner")
	}
	return driver, workspace, view
}

// seatCursorOnTaskTitled parks the Outliner cursor on the task row naming title.
// Walking the real projection rather than counting key presses is what keeps the
// assertion about the row kind instead of about how many `j`s the fixture
// happens to need.
func seatCursorOnTaskTitled(t *testing.T, view *outliner.OutlinerModel, title string) {
	t.Helper()
	for index, row := range view.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.Title == title {
			view.Cursor = index
			return
		}
	}
	t.Fatalf("no task row titled %q in the projection", title)
}

// TestChecklistAddItemAppliesFromEveryOwnerRowAndNoOther is the applicability
// half of the acceptance: `c` has to answer from a task whose checklist is
// empty, from one whose checklist is populated, and from a nested child, while
// still refusing every row that names no single task. The refusals are the point
// — `c` is a root key, so a fallback to a nearby task would append an item to
// work the cursor was not on.
func TestChecklistAddItemAppliesFromEveryOwnerRowAndNoOther(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	app := driver.App()
	descriptor, ok := app.GetOperationRegistry().Descriptor(operation.OperationChecklistAddItem)
	if !ok {
		t.Fatal("Add Item is not registered")
	}

	for _, title := range []string{
		checklistFixturePlainTitle,  // no checklist at all
		checklistFixtureFoldTitle,   // a populated checklist
		checklistFixtureParentTitle, // a populated checklist and a child
		"Confirm the staging soak",  // a nested child with no checklist
	} {
		seatCursorOnTaskTitled(t, view, title)
		if !descriptor.Applicable(app.operationCommandContext()) {
			t.Errorf("c refused the owner row %q", title)
		}
	}

	// One expanded item, which is the row the rest of the family needs and the
	// row `c` has always answered from.
	seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)
	view.Folds.SetChecklist(view.TaskIdentityForRow(view.Rows[view.Cursor]), true)
	view.RebuildRows()
	item := -1
	for index, row := range view.Rows {
		if row.IsChecklistItem {
			item = index
			break
		}
	}
	if item < 0 {
		t.Fatal("expanding the checklist produced no item row")
	}
	view.Cursor = item
	if !descriptor.Applicable(app.operationCommandContext()) {
		t.Error("c refused a checklist item row")
	}

	for _, row := range []struct {
		name  string
		match func(outliner.OutlinerRow) bool
	}{
		{"section header", func(r outliner.OutlinerRow) bool { return r.IsSectionHeader }},
		{"project header", func(r outliner.OutlinerRow) bool { return r.IsProjectHeader }},
	} {
		seated := false
		for index, candidate := range view.Rows {
			if row.match(candidate) {
				view.Cursor, seated = index, true
				break
			}
		}
		if !seated {
			t.Fatalf("the projection carries no %s row", row.name)
		}
		if descriptor.Applicable(app.operationCommandContext()) {
			t.Errorf("c applied on a %s, which names no task", row.name)
		}
	}

	// A visual selection is the last refusal: one item appended to one of N
	// selected tasks is not a bulk act, so `c` steps aside rather than picking
	// the cursor out of the selection.
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
	context := app.operationCommandContext()
	context.Selection.Visual = true
	if descriptor.Applicable(context) {
		t.Error("c applied under a visual selection")
	}
}

// TestChecklistAddItemReachesACompletedTask keeps the key off the lifecycle
// axis. A finished task still collects the follow-up notes its checklist holds,
// and nothing in the append reads its state, so completing the owner first must
// change nothing about what `c` does.
func TestChecklistAddItemReachesACompletedTask(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "$"},
		{Type: UITestActionKey, Key: "d"},
		{Type: UITestActionWait},
		// Recent admits terminal work, so it is where a completed task still has
		// a row to press a key on.
		{Type: UITestActionKey, Key: "g"},
		{Type: UITestActionKey, Key: "r"},
		{Type: UITestActionWait},
	})

	view = driver.App().crossOutliner
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
	if task := projectedTaskTitled(t, view, checklistFixturePlainTitle); !task.IsInState(models.StateDone) {
		t.Fatalf("$d left %q in state %q", task.Title, task.State())
	}

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "c"},
		{Type: UITestActionText, Text: "File the completion note"},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})

	task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
	if len(task.Checklist) != 1 || task.Checklist[0].Text != "File the completion note" {
		t.Fatalf("completed task checklist = %+v, want the one appended item", task.Checklist)
	}
	if !strings.Contains(driver.Render(), "[ ] File the completion note") {
		t.Errorf("the committed item was not revealed:\n%s", driver.Render())
	}
}

// checklistFixtureProjectDirectory is the one project every checklist fixture
// task lives in, which is the directory a forced write failure has to seal.
func checklistFixtureProjectDirectory(workspace *uitestWorkspace) string {
	return filepath.Join(workspace.DataDir, "projects", "checklist-delivery")
}

// checklistItemTexts is what the projection currently says the task's checklist
// holds. Reading the committed items rather than the screen is what separates an
// item that reached the outline from one that is only sitting in a prompt.
func checklistItemTexts(t *testing.T, view *outliner.OutlinerModel, title string) []string {
	t.Helper()
	task := projectedTaskTitled(t, view, title)
	texts := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		texts = append(texts, item.Text)
	}
	return texts
}

// TestChecklistAddItemLeavesTheOutlineAloneWhenTheWriteFails is the failure half
// of the reveal contract. Both the fold and the cursor move off the success
// message, so a refused write has to leave a collapsed checklist collapsed and
// the cursor where it stood. A read-only project directory is the honest
// refusal: it stops the transaction at the lock, before anything is written, so
// tasks.md is still exactly what the fixture wrote.
//
// The item text is the one thing a refusal must not take with it. It is on
// screen afterwards and it must be, because it is in the reopened prompt; what
// the outline may not carry is a committed item. So the two are asserted apart:
// the projection by its checklist, the buffer by the prompt's own value.
func TestChecklistAddItemLeavesTheOutlineAloneWhenTheWriteFails(t *testing.T) {
	driver, workspace, view := checklistFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)
	before := view.Cursor
	identity := view.TaskIdentityForRow(view.Rows[before])
	committed := checklistItemTexts(t, view, checklistFixtureFoldTitle)

	directory := checklistFixtureProjectDirectory(workspace)
	document := filepath.Join(directory, "tasks.md")
	written, err := os.ReadFile(document)
	if err != nil {
		t.Fatalf("read the fixture document: %v", err)
	}
	sealProjectDirectory(t, directory)

	const draft = "Never committed"
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "c"},
		{Type: UITestActionText, Text: draft},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})

	prompt := driver.App().operations.TextPrompt
	if prompt == nil {
		t.Fatalf("a refused append threw the buffer away:\n%s", driver.Render())
	}
	if prompt.Value() != draft {
		t.Errorf("the reopened buffer holds %q, want the refused item %q", prompt.Value(), draft)
	}
	if prompt.Notice() == "" {
		t.Error("the reopened buffer says nothing about why the save was refused")
	}
	if !strings.Contains(driver.Render(), draft) {
		t.Errorf("the retained item is not on screen:\n%s", driver.Render())
	}

	view = driver.App().crossOutliner
	if got := checklistItemTexts(t, view, checklistFixtureFoldTitle); !slices.Equal(got, committed) {
		t.Errorf("a refused append changed the committed checklist to %q, want %q", got, committed)
	}
	if after, err := os.ReadFile(document); err != nil || string(after) != string(written) {
		t.Errorf("a refused append rewrote tasks.md (read error %v)", err)
	}
	if view.Folds.ExpandedChecklists[identity] {
		t.Error("a refused append expanded the owner's checklist")
	}
	if view.Cursor != before {
		t.Errorf("cursor = %d after a refused append, want the row it was on (%d)", view.Cursor, before)
	}
}

// TestChecklistAddItemRetriesTheRetainedItem is the other half of retaining the
// draft: keeping the text is only worth anything if pressing the submit key
// again lands it. Both owner shapes are covered because they reach the same
// operation by different routes — an empty checklist has no item row, so `c`
// answers from the task itself, while a populated one is the shape the checklist
// row grammar owns — and a retry has to append exactly one item either way.
func TestChecklistAddItemRetriesTheRetainedItem(t *testing.T) {
	for _, owner := range []struct {
		name  string
		title string
	}{
		{"empty checklist", checklistFixturePlainTitle},
		{"populated checklist", checklistFixtureFoldTitle},
	} {
		t.Run(owner.name, func(t *testing.T) {
			driver, workspace, view := checklistFixtureApp(t)
			seatCursorOnTaskTitled(t, view, owner.title)
			committed := checklistItemTexts(t, view, owner.title)

			unseal := sealProjectDirectory(t, checklistFixtureProjectDirectory(workspace))

			const draft = "Keep this item after a refused save"
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "c"},
				{Type: UITestActionText, Text: draft},
				{Type: UITestActionKey, Key: "enter"},
				{Type: UITestActionWait},
			})
			if prompt := driver.App().operations.TextPrompt; prompt == nil || prompt.Value() != draft {
				t.Fatalf("the refusal did not leave the item to retry:\n%s", driver.Render())
			}

			unseal()
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "enter"},
				{Type: UITestActionWait},
			})

			if driver.App().operations.TextPrompt != nil {
				t.Errorf("a committed retry left the prompt open:\n%s", driver.Render())
			}
			want := append(slices.Clone(committed), draft)
			if got := checklistItemTexts(t, driver.App().crossOutliner, owner.title); !slices.Equal(got, want) {
				t.Fatalf("after the retry the checklist is %q, want %q", got, want)
			}

			// One append is one undo entry, so the refusal that preceded it recorded
			// nothing: a single u has to reach the fixture's own checklist.
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "u"},
				{Type: UITestActionWait},
			})
			if got := checklistItemTexts(t, driver.App().crossOutliner, owner.title); !slices.Equal(got, committed) {
				t.Fatalf("one undo left the checklist at %q, want the %q it started from", got, committed)
			}
		})
	}
}

// runChecklistActions replays actions through the production driver and fails
// the test on the first one the app refuses to accept.
func runChecklistActions(t *testing.T, driver *UITestDriver, actions []UITestAction) {
	t.Helper()
	for index, action := range actions {
		if _, err := driver.ApplyAction(action, ObserveOptions{}); err != nil {
			t.Fatalf("action %d (%s %q): %v", index, action.Type, action.Key+action.Text, err)
		}
	}
}

// projectedTaskTitled returns the task the projection currently carries under
// title. A row's Task is the reloaded workspace copy, so reading it is what
// separates a committed append from one that only reached the screen.
func projectedTaskTitled(t *testing.T, view *outliner.OutlinerModel, title string) *models.Task {
	t.Helper()
	for _, row := range view.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.Title == title {
			return row.Task
		}
	}
	t.Fatalf("no task titled %q in the projection", title)
	return nil
}
