package tui

// Shared fixtures for the shell's grammar audits: the registered AppModels the
// chord, parity, and header-target enumerations drive, and the two ways a test
// pushes a key path through the simplified strategy. Each audit reaches for
// more than one of these, so no single audit file can own them.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

func newFilterChordTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := newUITestApp()
	project := &models.Project{
		ID:   "project",
		Name: "Project",
		Slug: "project",
		Tasks: []*models.Task{
			{ID: "critical", ShortID: 1, Title: "Critical", WorkflowState: models.StateBacklog, Priority: 1},
			{ID: "medium", ShortID: 2, Title: "Medium", WorkflowState: models.StateBacklog, Priority: 2, Review: true},
			{ID: "low", ShortID: 3, Title: "Low", WorkflowState: models.StateWaiting, Priority: 3, Tags: []string{"deep"}},
		},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.RebuildRows()
	return app
}

func pressFilterChord(t *testing.T, app *AppModel, keys string) tea.Cmd {
	t.Helper()
	var cmd tea.Cmd
	var handled bool
	for _, key := range keys {
		handled, cmd = app.handleGlobalKeyPress(tea.KeyPressMsg{Code: key, Text: string(key)})
		if !handled {
			t.Fatalf("key %q of chord %q was not handled", key, keys)
		}
	}
	if len(app.operations.Chord) != 0 {
		t.Fatalf("chord %q left pending prefix %v", keys, app.operations.Chord)
	}
	return cmd
}

// headerTargetProjectHeaderRow folds the projection's project groups and returns
// the first summary they leave behind. The fold is not incidental setup: an
// expanded group draws no heading, so its summary is the only project header a
// cross-project projection ever has, and it is the row every project operation
// resolves from.
func headerTargetProjectHeaderRow(t *testing.T, outlinerView *outliner.OutlinerModel) int {
	t.Helper()
	foldEveryProjectGroup(t, outlinerView)
	for index, row := range outlinerView.Rows {
		if row.IsProjectHeader {
			return index
		}
	}
	t.Fatal("fixture Outliner exposed no project header row")
	return 0
}

func headerTargetAuditApp(t *testing.T) *AppModel {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	return driver.App()
}

// parityAuditApp builds the production catalog exactly as NewAppModel does, so
// these invariants audit registerOperationCatalog itself rather than a
// test-local subset of it.
func parityAuditApp(t *testing.T) *AppModel {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	return driver.App()
}

// peekPagingGrammar names the operations that exist only while a Peek is open
// over the cursor task. An audit that builds its context from a closed Peek
// would read them as unbound keys, which is the grammar working rather than a
// gap, so it opens that state for them the same way it puts a checklist cursor
// under the checklist row grammar. The half-page pair shares its keys with the
// list cursor, so this map is also what tells the audit which of the two
// meanings each binding is being resolved for.
var peekPagingGrammar = map[operation.OperationID]bool{
	operation.OperationCorePeekHalfPageDown: true,
	operation.OperationCorePeekHalfPageUp:   true,
	operation.OperationCorePeekPageDown:     true,
	operation.OperationCorePeekPageUp:       true,
}

// pagePeekParityContext is that state: a visible Peek that can move either way,
// which is the context all four scrolling keys resolve in at once.
func pagePeekParityContext(ctx operation.CommandContext) operation.CommandContext {
	ctx.Peek = operation.PeekContext{Visible: true, CanPageDown: true, CanPageUp: true}
	return ctx
}

// taskActionParityContext focuses the Outliner on the first live task so the
// sheet projection is evaluated against a real target instead of an empty one.
func taskActionParityContext(t *testing.T, app *AppModel) operation.CommandContext {
	t.Helper()
	outlinerView := app.crossOutliner
	if outlinerView == nil {
		t.Fatal("fixture app has no Outliner")
	}
	for index, row := range outlinerView.Rows {
		if !row.IsTaskRow() {
			continue
		}
		outlinerView.Cursor = index
		ctx := outlinerView.OperationCommandContext()
		if operation.CurrentTaskIdentity(ctx).Valid() {
			return ctx
		}
	}
	t.Fatal("fixture Outliner exposed no task row")
	return operation.CommandContext{}
}

// executeDirectPath resolves one strategy key path in ctx and runs it exactly as
// handleSimplifiedInputKey does, so the preset the grammar declares is the value
// the executor receives.
func executeDirectPath(
	t *testing.T,
	model *AppModel,
	ctx operation.CommandContext,
	path []string,
) operation.OperationExecutionResult {
	t.Helper()
	binding, descriptor, resolution := model.operations.Registry.InputBindingForSequence(
		operation.InputStrategySimplified,
		path,
		ctx,
	)
	if resolution != operation.ChordTerminal {
		t.Fatalf("key path %v resolution = %v, want terminal", path, resolution)
	}
	return model.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:       descriptor.ID,
		Context:  ctx,
		Value:    binding.PresetValue,
		HasValue: binding.HasPresetValue,
	})
}
