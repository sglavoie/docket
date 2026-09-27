package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// outlinerEditOperations is `e`'s whole family in Outliner declaration order,
// which is also the order the strategy resolves them in: the first applicable
// binding wins, so the set is only honest if at most one of them ever applies.
var outlinerEditOperations = []operation.OperationID{
	operation.OperationChecklistEditItem,
	operation.OperationScopeRenameProject,
	operation.OperationCoreEdit,
}

// applicableEditOperations names every member of that family the given context
// admits. The count is the assertion in every case below: one on a row that
// names an editable entity, zero on a row that names none.
func applicableEditOperations(t *testing.T, app *AppModel) []operation.OperationID {
	t.Helper()
	var applicable []operation.OperationID
	context := app.operationCommandContext()
	for _, id := range outlinerEditOperations {
		descriptor, ok := app.GetOperationRegistry().Descriptor(id)
		if !ok {
			t.Fatalf("%s is not registered", id)
		}
		if descriptor.Applicable(context) {
			applicable = append(applicable, id)
		}
	}
	return applicable
}

// TestOutlinerEditKeyNamesExactlyOneEntityPerRow is the gate session 2 rests on.
// Three operations answer `e` in the Outliner, and nothing but their
// applicability keeps them apart, so every row shape is checked for exactly one
// winner and named. The checklist fixture holds a single project on purpose:
// Rename Project carries a sole-scope fallback for the Project Hub, where a row
// already named its project, and here that fallback would let `e` rename the
// workspace's only project from a section header or an empty screen.
func TestOutlinerEditKeyNamesExactlyOneEntityPerRow(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)

	cases := []struct {
		name    string
		seat    func(t *testing.T)
		want    operation.OperationID
		refuses bool
	}{
		{
			name: "a task row edits the task",
			seat: func(t *testing.T) {
				seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)
			},
			want: operation.OperationCoreEdit,
		},
		{
			name: "a checklist item row edits the item",
			seat: func(t *testing.T) {
				seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)
				view.Folds.SetChecklist(view.TaskIdentityForRow(view.Rows[view.Cursor]), true)
				view.RebuildRows()
				seatCursorOnRow(t, view, "checklist item", func(row outliner.OutlinerRow) bool {
					return row.IsChecklistItem
				})
			},
			want: operation.OperationChecklistEditItem,
		},
		{
			name: "a project header edits the project",
			seat: func(t *testing.T) {
				seatCursorOnProjectHeader(t, view)
			},
			want: operation.OperationScopeRenameProject,
		},
		{
			// A section header names no entity at all, so `e` falls to Edit Task
			// and refuses there, the way every other task key on a header does.
			// What matters is which operation it is not: Rename Project would
			// have this workspace's only project to fall back on.
			name: "a section header falls to Edit Task and refuses",
			seat: func(t *testing.T) {
				seatCursorOnRow(t, view, "section header", func(row outliner.OutlinerRow) bool {
					return row.IsSectionHeader
				})
			},
			want:    operation.OperationCoreEdit,
			refuses: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.seat(t)
			assertEditKeyResolvesTo(t, driver, testCase.want)
			if testCase.refuses {
				assertEditKeyOpensNoEditor(t, driver)
			}
		})
	}

	// An empty projection is the sharpest version of that refusal: no row means
	// no entity, and the sole project is still one keystroke from being renamed
	// if anything in the family reaches past the cursor for a target.
	// A fresh app, because the refusal above already put its status in the strip
	// and this case has to see its own.
	t.Run("an empty projection falls to Edit Task and refuses", func(t *testing.T) {
		driver, _, _ := checklistFixtureApp(t)
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "/"},
			{Type: UITestActionText, Text: "zzzzz no task carries this"},
			{Type: UITestActionKey, Key: "enter"},
			{Type: UITestActionWait},
		})
		for _, row := range driver.App().crossOutliner.Rows {
			if row.IsTaskRow() {
				t.Fatalf("the search still matched %q, so this is not the empty case", row.Task.Title)
			}
		}
		assertEditKeyResolvesTo(t, driver, operation.OperationCoreEdit)
		assertEditKeyOpensNoEditor(t, driver)
	})
}

// assertEditKeyResolvesTo holds the family to exactly one applicable member and
// names it, which is the whole of the disjointness contract.
func assertEditKeyResolvesTo(t *testing.T, driver *UITestDriver, want operation.OperationID) {
	t.Helper()
	applicable := applicableEditOperations(t, driver.App())
	if len(applicable) != 1 {
		t.Fatalf("e applied as %v, want exactly one operation", applicable)
	}
	if applicable[0] != want {
		t.Fatalf("e resolved to %s, want %s", applicable[0], want)
	}
}

// assertEditKeyOpensNoEditor presses the key and checks that the Outliner kept
// the screen. A project editor here would mean the sole-scope fallback fired.
func assertEditKeyOpensNoEditor(t *testing.T, driver *UITestDriver) {
	t.Helper()
	before := driver.App().crossOutliner.Cursor
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "e"},
		{Type: UITestActionWait},
		{Type: UITestActionAssert, Assert: "view", Value: "CrossProjectOutliner"},
		{Type: UITestActionAssert, Assert: "contains", Value: "Move the cursor to a task to use this action"},
	})
	if got := driver.App().crossOutliner.Cursor; got != before {
		t.Fatalf("a refused e moved the cursor from %d to %d", before, got)
	}
}
