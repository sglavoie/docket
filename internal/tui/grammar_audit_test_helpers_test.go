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
	app.crossOutliner.population.projects = []*models.Project{project}
	app.crossOutliner.population.invalidate()
	app.crossOutliner.rebuildRows()
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
	if len(app.pendingChord) != 0 {
		t.Fatalf("chord %q left pending prefix %v", keys, app.pendingChord)
	}
	return cmd
}

func headerTargetProjectHeaderRow(t *testing.T, outliner *OutlinerModel) int {
	t.Helper()
	for index, row := range outliner.rows {
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

// taskActionParityContext focuses the Outliner on the first live task so the
// sheet projection is evaluated against a real target instead of an empty one.
func taskActionParityContext(t *testing.T, app *AppModel) operation.CommandContext {
	t.Helper()
	outliner := app.crossOutliner
	if outliner == nil {
		t.Fatal("fixture app has no Outliner")
	}
	for index, row := range outliner.rows {
		if !row.isTaskRow() {
			continue
		}
		outliner.Cursor = index
		ctx := outliner.operationCommandContext()
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
	binding, descriptor, resolution := model.operationRegistry.InputBindingForSequence(
		operation.InputStrategySimplified,
		path,
		ctx,
	)
	if resolution != operation.ChordTerminal {
		t.Fatalf("key path %v resolution = %v, want terminal", path, resolution)
	}
	return model.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:       descriptor.ID,
		Context:  ctx,
		Value:    binding.PresetValue,
		HasValue: binding.HasPresetValue,
	})
}
