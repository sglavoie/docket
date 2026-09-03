package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// TestOutlineEditorRequestRefusesFixedOrderModes is the mode guard. Today, Next,
// Due, and Recent promote rows across section boundaries, so indentation there
// is not parentage.
func TestOutlineEditorRequestRefusesFixedOrderModes(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	outliner := app.activeOutliner()
	if outliner == nil {
		t.Fatal("fixture app has no Outliner")
	}

	for _, mode := range chrome.OutlinerModeCatalog() {
		outliner.primaryMode = mode.ID
		project, label, refusal := app.outlineEditorRequest()
		if mode.FixedOrder() {
			if refusal != outlineEditorModeRefusal {
				t.Errorf("%s refusal = %q, want %q", mode.Label, refusal, outlineEditorModeRefusal)
			}
			if project != nil {
				t.Errorf("%s named a project despite refusing", mode.Label)
			}
			continue
		}
		if refusal != "" {
			t.Errorf("%s refused with %q", mode.Label, refusal)
		}
		if label != mode.Label {
			t.Errorf("%s reported label %q", mode.Label, label)
		}
	}
}

// TestOutlineEditorIsInapplicableUnderVisualSelection matches the existing
// structural-operation rule: a selection claims a narrower scope than the whole
// project group the buffer would rewrite.
func TestOutlineEditorIsInapplicableUnderVisualSelection(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	descriptor, ok := app.GetOperationRegistry().Descriptor(operationCoreOutlineEditor)
	if !ok {
		t.Fatal("tui.core.outline-editor is not registered")
	}

	ctx := app.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	if !descriptor.Applicable(ctx) {
		t.Fatal("outline edit is inapplicable in the plain Outliner")
	}
	ctx.Selection.Visual = true
	if descriptor.Applicable(ctx) {
		t.Fatal("outline edit stayed applicable under a visual selection")
	}
}

// TestOutlineEditorRequestReadsTheProjectUnderEveryRowKind is why `O` is
// classified target-free rather than rejecting on a project header: the buffer's
// scope is the project the cursor sits in, and a header names one as plainly as
// a task row does.
func TestOutlineEditorRequestReadsTheProjectUnderEveryRowKind(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	outliner := app.activeOutliner()
	if outliner == nil {
		t.Fatal("fixture app has no Outliner")
	}

	header, task := -1, -1
	for index, row := range outliner.rows {
		if row.IsProjectHeader && header < 0 {
			header = index
		}
		if row.isTaskRow() && task < 0 {
			task = index
		}
	}
	if header < 0 || task < 0 {
		t.Fatalf("fixture exposed header=%d task=%d", header, task)
	}

	outliner.cursor = header
	fromHeader, _, refusal := app.outlineEditorRequest()
	if refusal != "" {
		t.Fatalf("a project header refused with %q", refusal)
	}
	if fromHeader == nil {
		t.Fatal("a project header named no project")
	}
	if fromHeader.ID != outliner.rows[header].ProjectID {
		t.Errorf("header resolved %s, want %s", fromHeader.ID, outliner.rows[header].ProjectID)
	}

	outliner.cursor = task
	fromTask, _, refusal := app.outlineEditorRequest()
	if refusal != "" {
		t.Fatalf("a task row refused with %q", refusal)
	}
	if fromTask == nil || fromTask.ID != outliner.rows[task].ProjectID {
		t.Errorf("task row resolved %v, want %s", fromTask, outliner.rows[task].ProjectID)
	}
}

// TestOutlineEditorDumpsOneSnapshotWhenTheCursorProjectIsStale is why `O`
// reloads instead of dumping the row's own project pointer: the shell's copy can
// be minutes old, and pairing it with a freshly read revision would claim the
// buffer describes a tree it never saw.
func TestOutlineEditorDumpsOneSnapshotWhenTheCursorProjectIsStale(t *testing.T) {
	stale := outlineTestProject(t)
	gone, err := models.NewGenericTask("aa11bb22cccc", 5, "Removed upstream")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	stale.Tasks = append(stale.Tasks, gone)

	fresh := outlineTestProject(t)
	fresh.Tasks[0].Title = "Ship auth rewrite, reworded upstream"

	stub := &stubMutator{project: fresh, result: app.TaskResult{Revision: "rev-7"}}
	model := &AppModel{features: coordinator.NewFeatureCoordinators(stub)}

	project, revision, buffer, scope, err := model.outlineDumpSource(stale.ID, "Backlog")
	if err != nil {
		t.Fatalf("outlineDumpSource: %v", err)
	}
	if len(scope) != len(fresh.Tasks) {
		t.Errorf("scope holds %d ids, want the %d tasks the snapshot dumped", len(scope), len(fresh.Tasks))
	}
	if project != fresh {
		t.Fatalf("dump project = %v, want the snapshot project", project)
	}
	if revision != "rev-7" {
		t.Errorf("revision = %q, want the one read beside the project", revision)
	}
	if !strings.Contains(buffer, "Ship auth rewrite, reworded upstream") {
		t.Errorf("buffer missing the current title:\n%s", buffer)
	}
	if strings.Contains(buffer, "Removed upstream") {
		t.Errorf("buffer dumped a task only the stale cursor project held:\n%s", buffer)
	}
}
