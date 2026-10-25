package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/status"
)

// relationDriverWithReference seeds one cross-project reference into the running
// fixture and hands back the driver, the referring task, and the target it
// names. The reference is written in memory rather than through the grammar
// because this file is about `g~`, not about how a reference is created.
func relationDriverWithReference(t *testing.T) (*UITestDriver, *models.Task, *models.Task, *models.Project) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("fixture %s: %v", FixtureMultiProject, err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	outlinerView := driver.App().crossOutliner
	if outlinerView == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	// The launch folds Backlog, which holds part of the fixture's work.
	outlinerView.ClearAllFolds()
	// Both ends have to be rows the launch view already draws, or the hop would
	// be proving something about revelation rather than about the keys. A row
	// carries a copy of its task, so the pair is named by ID here and the
	// reference is written into the population the index reads.
	var source, target *models.Task
	var sourceProjectID, targetProjectID string
	for _, row := range outlinerView.Rows {
		if !row.IsTaskRow() || row.Task == nil || row.ProjectID == "" {
			continue
		}
		switch {
		case source == nil:
			source, sourceProjectID = row.Task, row.ProjectID
		case target == nil && row.ProjectID != sourceProjectID:
			target, targetProjectID = row.Task, row.ProjectID
		}
	}
	if source == nil || target == nil {
		t.Fatal("the fixture holds no cross-project pair to reference")
	}
	targetProject := driver.App().ProjectByID(targetProjectID)
	if targetProject == nil {
		t.Fatalf("the loaded population has no project %q", targetProjectID)
	}
	stored := relationPopulationTask(t, outlinerView, sourceProjectID, source.ID)
	stored.Dependencies = []models.TaskReference{{ProjectID: targetProjectID, TaskID: target.ID}}
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	return driver, source, target, targetProject
}

// relationPopulationTask resolves the loaded task a row is a copy of.
func relationPopulationTask(t *testing.T, outlinerView *outliner.OutlinerModel, projectID, taskID string) *models.Task {
	t.Helper()
	for _, project := range outlinerView.Population.Projects {
		if project == nil || project.ID != projectID {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.ID == taskID {
				return task
			}
		}
	}
	t.Fatalf("the loaded population has no task %q in project %q", taskID, projectID)
	return nil
}

// relationCursorOnTask parks the cursor on one task's row.
func relationCursorOnTask(t *testing.T, outlinerView *outliner.OutlinerModel, task *models.Task) {
	t.Helper()
	for index, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == task.ID {
			outlinerView.Cursor = index
			return
		}
	}
	t.Fatalf("no row holds the task %q", task.Title)
}

// TestGotoRelationWalksTheReferenceGraphThroughRealKeys is the whole loop as the
// user drives it: `g~` opens the picker over the cursor task, Enter lands on the
// target in its own project, and Esc comes back.
func TestGotoRelationWalksTheReferenceGraphThroughRealKeys(t *testing.T) {
	driver, source, target, targetProject := relationDriverWithReference(t)
	outlinerView := driver.App().crossOutliner
	relationCursorOnTask(t, outlinerView, source)
	originMode := outlinerView.PrimaryMode

	sortPressKeys(t, driver, "g", "~")
	if !outlinerView.RelationPickerOpen() {
		t.Fatal("g~ did not open the relationship picker")
	}
	// The overlay is what the frame is showing, and it names its subject.
	if view := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(view, "Relationships") {
		t.Fatalf("the frame does not show the picker:\n%s", view)
	}

	sortPressKeys(t, driver, "enter")
	if outlinerView.RelationPickerOpen() {
		t.Fatal("Enter left the picker open over its own destination")
	}
	if outlinerView.Scope.ProjectID != targetProject.ID {
		t.Fatalf("scope = %q, want the target's project %q", outlinerView.Scope.ProjectID, targetProject.ID)
	}
	if outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf("mode = %s, want All", outlinerView.PrimaryMode)
	}
	if got := outlinerView.CurrentTaskIdentity(); got.TaskID != target.ID {
		t.Fatalf("cursor task = %+v, want the target %q", got, target.ID)
	}

	sortPressKeys(t, driver, "esc")
	if outlinerView.Scope.ProjectID != "" {
		t.Fatalf("Esc left the scope at %q, want the origin", outlinerView.Scope.ProjectID)
	}
	if outlinerView.PrimaryMode != originMode {
		t.Fatalf("Esc restored mode %s, want the origin %s", outlinerView.PrimaryMode, originMode)
	}
	if got := outlinerView.CurrentTaskIdentity(); got.TaskID != source.ID {
		t.Fatalf("Esc parked on %+v, want the task the hop was taken from", got)
	}
	if outlinerView.RelationPickerOpen() {
		t.Fatal("the return reopened the picker")
	}
}

// TestGotoRelationEscClosesThePickerWithoutMoving keeps cancellation cheap: the
// picker is put down and nothing else changes.
func TestGotoRelationEscClosesThePickerWithoutMoving(t *testing.T) {
	driver, source, _, _ := relationDriverWithReference(t)
	outlinerView := driver.App().crossOutliner
	relationCursorOnTask(t, outlinerView, source)

	sortPressKeys(t, driver, "g", "~", "esc")
	if outlinerView.RelationPickerOpen() {
		t.Fatal("Esc did not close the picker")
	}
	if outlinerView.Scope.ProjectID != "" || outlinerView.HasRelationTrail() {
		t.Fatalf("canceling moved the view: scope=%q trail=%v",
			outlinerView.Scope.ProjectID, outlinerView.HasRelationTrail())
	}
	if got := outlinerView.CurrentTaskIdentity(); got.TaskID != source.ID {
		t.Fatalf("canceling moved the cursor to %+v", got)
	}
}

func TestGotoRelationRefusesWithoutASingleTaskCursor(t *testing.T) {
	t.Run("project header", func(t *testing.T) {
		driver, _, _, _ := relationDriverWithReference(t)
		outlinerView := driver.App().crossOutliner
		outlinerView.Cursor = headerTargetProjectHeaderRow(t, outlinerView)

		prefix, err := parseUITestKey("g")
		if err != nil {
			t.Fatalf("parse g: %v", err)
		}
		handled, cmd := driver.App().handleSimplifiedInputKey(prefix)
		if !handled || cmd != nil {
			t.Fatalf("g prefix = handled %v cmd %v, want a pending chord", handled, cmd)
		}
		key, err := parseUITestKey("~")
		if err != nil {
			t.Fatalf("parse ~: %v", err)
		}
		handled, cmd = driver.App().handleSimplifiedInputKey(key)
		if !handled || cmd == nil {
			t.Fatalf("header press = handled %v cmd %v, want the shared refusal", handled, cmd)
		}
		failure, ok := cmd().(status.ErrorMsg)
		if !ok {
			t.Fatalf("header press produced %T, not a refusal", cmd())
		}
		if failure.Err.Error() != operation.OperationTargetRejectionMessage(operation.OperationCategoryTask) {
			t.Fatalf("refusal reads %q, not the shared sentence", failure.Err.Error())
		}
		if outlinerView.RelationPickerOpen() {
			t.Fatal("a project header opened the relationship picker")
		}
	})

	t.Run("visual selection", func(t *testing.T) {
		driver, source, _, _ := relationDriverWithReference(t)
		outlinerView := driver.App().crossOutliner
		relationCursorOnTask(t, outlinerView, source)

		sortPressKeys(t, driver, "v", "g", "~")
		if outlinerView.RelationPickerOpen() {
			t.Fatal("a visual selection opened the relationship picker")
		}
	})
}

// TestGotoRelationIsProjectedIntoTheMenuBarAndHelp keeps the one registration
// visible everywhere the registry is projected: the descriptor answers `g~`,
// the Go menu names it, and the generated help lists it.
func TestGotoRelationIsProjectedIntoTheMenuBarAndHelp(t *testing.T) {
	driver, source, _, _ := relationDriverWithReference(t)
	app := driver.App()
	// The descriptor is gated on a single task, so the projection is read from a
	// context that has one: an unresolvable chord here would only mean the
	// cursor was pointing at a header.
	relationCursorOnTask(t, app.crossOutliner, source)
	ctx := app.operationCommandContext()

	binding, descriptor, resolution := app.operations.Registry.InputBindingForSequence(
		operation.InputStrategySimplified,
		[]string{"g", "~"},
		ctx,
	)
	if resolution != operation.ChordTerminal {
		t.Fatalf("g~ resolved %v, want a terminal chord", resolution)
	}
	if descriptor.ID != operation.OperationCoreGotoRelation {
		t.Fatalf("g~ answers %q", descriptor.ID)
	}
	if binding.DisplayKey != "g~" {
		t.Fatalf("display key = %q, want g~", binding.DisplayKey)
	}

	// The bar drops entries their context cannot run, so the Go menu is resolved
	// against the same cursor the chord was: a launch context names no task and
	// would hide the whole task-gated half of that menu.
	strategy, ok := app.GetOperationRegistry().InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	menus, err := outliner.ResolveMenuEntries(
		app.GetOperationRegistry(),
		strategy,
		ctx,
		outliner.OutlinerMenuSpecs(chrome.ModeAll, false, true),
	)
	if err != nil {
		t.Fatalf("resolve menus: %v", err)
	}
	goMenu := menuWithID(t, menus, outliner.MenuIDGo)
	if !menuHasLabel(goMenu, "Related Task") {
		t.Fatalf("the Go menu does not project the relationship jump: %+v", goMenu.Entries)
	}

	found := false
	for _, item := range app.coreInputHelpItems() {
		if item.Key == "g~" {
			found = true
			if item.Description != "Go to Related Task" {
				t.Fatalf("help describes g~ as %q", item.Description)
			}
		}
	}
	if !found {
		t.Fatal("the generated help does not list g~")
	}
}
