package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestInlineRowEditorBuildsAtomicCrossProjectRequest(t *testing.T) {
	_, source, _, child := inlineEditorFixture(t)
	parent := &models.Task{ID: "destination-parent", ShortID: 7, Title: "Destination Parent", WorkflowState: models.StateBacklog}
	destination := &models.Project{
		ID: "destination", Slug: "destination", Name: "Destination",
		Tasks: []*models.Task{parent},
	}
	destination.BuildTaskTree()

	result, err := grammar.ParseQuickEdit(
		"Renamed !2 @0 $today #new -#alpha %destination >7",
		child, source, []*models.Project{source, destination},
	)
	if err != nil {
		t.Fatalf("parse cross-project row edit: %v", err)
	}
	req := outliner.TaskLineEditRequest(taskref.For(child, source), result)
	if req.Source.ProjectID != source.ID || req.Source.TaskID != child.ID || req.DestinationProjectID != destination.ID {
		t.Fatalf("request route = source %+v destination %q", req.Source, req.DestinationProjectID)
	}
	if req.Title != "Renamed" || req.Priority == nil || *req.Priority != models.PriorityMedium {
		t.Fatalf("request content = %+v", req)
	}
	if !req.DueSet || req.Due != nil || req.Destination == nil || *req.Destination != app.CaptureDestinationToday {
		t.Fatalf("request destination/due = destination %v dueSet %t due %v", req.Destination, req.DueSet, req.Due)
	}
	if req.Parent.Mode != app.TaskLineParentSet || req.Parent.Parent.ProjectID != destination.ID || req.Parent.Parent.TaskID != parent.ID {
		t.Fatalf("request parent = %+v", req.Parent)
	}
}

func TestInlineRowEditorAtomicCommitAndFailureLifecycle(t *testing.T) {
	m, _, task, _ := newPlanningLedgerApp(t, models.StateBacklog)
	out := outliner.NewOutlinerModel(m.currentProject, m.domain.manager, m.domain.runtime)
	out.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	out.SetSize(100, 20)
	out.RebuildRows()
	out.Folds.ExpandedNodes[taskref.For(task, m.currentProject)] = false
	out.Folds.CollapsedSections["unrelated"] = true
	m.crossOutliner = out
	m.viewNav.navigation.ResetRoot()
	projection := out.JumpProjectionFingerprint()

	out.Update(outliner.StartTaskLineEditMsg{Task: task, Project: m.currentProject})
	out.QuickEdit.Input.SetValue("Renamed atomically")
	out.QuickEdit.Input.CursorEnd()
	out.QuickEdit.RefreshDraft()
	_, cmd := out.HandleQuickEditMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !out.QuickEdit.IsActive() || !out.QuickEdit.Pending {
		t.Fatalf("commit did not retain a pending inline draft: active=%t pending=%t", out.QuickEdit.IsActive(), out.QuickEdit.Pending)
	}
	committed, ok := cmd().(coordinator.TaskLineEditedMsg)
	if !ok || !committed.Result.Changed || committed.Result.TransactionID == "" {
		t.Fatalf("atomic command result = %#v", committed)
	}
	m.handleTaskLineEdited(committed)
	if out.QuickEdit.IsActive() {
		t.Fatal("successful atomic edit left the row editor active")
	}
	if got := m.currentProject.GetTaskByID(task.ID).Title; got != "Renamed atomically" {
		t.Fatalf("reloaded title = %q, want Renamed atomically", got)
	}
	if got := out.JumpProjectionFingerprint(); got != projection {
		t.Fatalf("successful inline edit changed projection state:\n before %s\n after  %s", projection, got)
	}

	live := m.currentProject.GetTaskByID(task.ID)
	out.Update(outliner.StartTaskLineEditMsg{Task: live, Project: m.currentProject})
	out.QuickEdit.Input.SetValue("Draft survives")
	out.QuickEdit.Input.CursorEnd()
	out.QuickEdit.RefreshDraft()
	out.QuickEdit.BeginCommit()
	projection = out.JumpProjectionFingerprint()
	failure := errors.New("simulated revision conflict")
	m.handleTaskLineEditFailed(coordinator.TaskLineEditFailedMsg{
		Source: taskref.For(live, m.currentProject), Err: failure,
	})
	if !out.QuickEdit.IsActive() || out.QuickEdit.Pending || out.QuickEdit.Input.Value() != "Draft survives" {
		t.Fatalf("failed commit lost draft: active=%t pending=%t value=%q", out.QuickEdit.IsActive(), out.QuickEdit.Pending, out.QuickEdit.Input.Value())
	}
	if !strings.Contains(out.QuickEdit.GetPreviewError(), failure.Error()) {
		t.Fatalf("inline failure = %q, want %q", out.QuickEdit.GetPreviewError(), failure)
	}
	if got := out.JumpProjectionFingerprint(); got != projection {
		t.Fatalf("failed inline edit changed projection state:\n before %s\n after  %s", projection, got)
	}
}

func TestInlineRowEditorUnchangedCommitCreatesNoUndoTransaction(t *testing.T) {
	m, _, task, _ := newPlanningLedgerApp(t, models.StateBacklog)
	out := outliner.NewOutlinerModel(m.currentProject, m.domain.manager, m.domain.runtime)
	out.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	out.SetSize(100, 20)
	out.RebuildRows()
	m.crossOutliner = out
	m.viewNav.navigation.ResetRoot()

	before, err := m.domain.ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("undo count before unchanged edit: %v", err)
	}

	out.Update(outliner.StartTaskLineEditMsg{Task: task, Project: m.currentProject})
	if !out.QuickEdit.IsActive() || out.QuickEdit.Input.Value() != task.Title {
		t.Fatalf("inline editor did not open with unchanged title: active=%t value=%q", out.QuickEdit.IsActive(), out.QuickEdit.Input.Value())
	}
	_, cmd := out.HandleQuickEditMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("unchanged inline commit returned nil command")
	}
	committed, ok := cmd().(coordinator.TaskLineEditedMsg)
	if !ok || committed.Result.Changed || committed.Result.TransactionID != "" {
		t.Fatalf("unchanged inline result = %#v", committed)
	}
	m.handleTaskLineEdited(committed)

	if out.QuickEdit.IsActive() {
		t.Fatal("unchanged inline commit left the row editor active")
	}
	if m.feedback.message != "No task changes" {
		t.Fatalf("unchanged inline status = %q, want No task changes", m.feedback.message)
	}
	after, err := m.domain.ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("undo count after unchanged edit: %v", err)
	}
	if after != before {
		t.Fatalf("unchanged inline edit created an undo entry: before=%d after=%d", before, after)
	}
}

// TestTaskLineEditRelocationFollowsInGroupResultAndRecoversOtherwise is the row
// editor's half of the one retention rule. The block the edit was typed from is
// what decides: a task that block still holds is followed and records the origin
// `Ctrl+O` returns to, while a task that left it — for another project, or for a
// value the projection no longer draws — is a departure the cursor does not
// chase. Both departures land on the neighbor the edit did not touch.
func TestTaskLineEditRelocationFollowsInGroupResultAndRecoversOtherwise(t *testing.T) {
	makeOutliner := func() (*outliner.OutlinerModel, *models.Project, *models.Project, *models.Task, *models.Task) {
		task := &models.Task{ID: "moving", ShortID: 1, Title: "Moving", WorkflowState: models.StateBacklog}
		neighbor := &models.Task{ID: "neighbor", ShortID: 2, Title: "Neighbor", WorkflowState: models.StateBacklog}
		source := &models.Project{ID: "alpha-source", Slug: "alpha-source", Name: "Source", Tasks: []*models.Task{task, neighbor}}
		destination := &models.Project{ID: "zeta-destination", Slug: "zeta-destination", Name: "Destination"}
		out := outliner.NewOutlinerModel(nil, nil, nil)
		out.Population.Projects = []*models.Project{source, destination}
		out.Population.SnapshotReady = true
		out.Population.Invalidate()
		out.CrossProjectRebuildRows()
		for index, row := range out.Rows {
			if row.Task != nil && row.Task.ID == task.ID && row.ProjectID == source.ID {
				out.Cursor = index
			}
		}
		out.SyncJumpProjection()
		return out, source, destination, task, neighbor
	}

	// A reparent inside the source project: the same block still holds the task,
	// one row further down, which is exactly what an origin is recorded for.
	out, source, _, task, neighbor := makeOutliner()
	capture := out.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(task, source)})
	task.ParentID = &neighbor.ID
	neighbor.Children = []*models.Task{task}
	source.Tasks = []*models.Task{neighbor, task}
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	_, recorded := out.FinishTaskLineEditRelocation(capture, taskref.For(task, source))
	if !recorded || out.CurrentTaskIdentity() != taskref.For(task, source) || len(out.RelocationHistory.Back) != 1 {
		t.Fatalf("in-group relocation = current %+v recorded=%t history=%d", out.CurrentTaskIdentity(), recorded, len(out.RelocationHistory.Back))
	}

	// The same edit spelling another project is a departure, even though the task
	// is still on screen under that project's own header.
	out, source, destination, task, _ := makeOutliner()
	capture = out.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(task, source)})
	source.Tasks = source.Tasks[1:]
	destination.Tasks = append(destination.Tasks, task)
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	if _, visible := out.RowIndexForTask(taskref.For(task, destination)); !visible {
		t.Fatal("the moved task left the projection, so this half proves nothing about following it")
	}
	_, recorded = out.FinishTaskLineEditRelocation(capture, taskref.For(task, destination))
	if recorded || out.CurrentTaskIdentity().TaskID != "neighbor" || len(out.RelocationHistory.Back) != 0 {
		t.Fatalf("cross-project relocation = current %+v recorded=%t history=%d", out.CurrentTaskIdentity(), recorded, len(out.RelocationHistory.Back))
	}

	out, source, _, task, _ = makeOutliner()
	capture = out.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(task, source)})
	source.Tasks = source.Tasks[1:]
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	_, recorded = out.FinishTaskLineEditRelocation(capture, taskref.TaskIdentity{ProjectID: "hidden", TaskID: task.ID})
	if recorded || out.CurrentTaskIdentity().TaskID != "neighbor" || len(out.RelocationHistory.Back) != 0 {
		t.Fatalf("hidden relocation = current %+v recorded=%t history=%d", out.CurrentTaskIdentity(), recorded, len(out.RelocationHistory.Back))
	}
}

// longestVisibleSuffix reports how many trailing runes of value the rendered line
// still shows, which is what distinguishes a horizontally scrolled window from a
// window computed for a stale width.
func longestVisibleSuffix(line, value string) int {
	runes := []rune(value)
	for index := range runes {
		if strings.Contains(line, string(runes[index:])) {
			return len(runes) - index
		}
	}
	return 0
}

func TestInlineRowEditorKeepsTypedTailAndCursorVisibleWhenTitleOverflows(t *testing.T) {
	out, project, _, child := inlineEditorFixture(t)
	out.QuickEdit.EnterWithContext(child, project, []*models.Project{project}, false)
	row := out.Rows[out.Cursor]

	// The first frame is what installs the row width, so the value has to land
	// afterwards for the scroll window to be the one an editing session sees.
	out.RenderOutlinerQuickEditRow(row, 80, nil)
	value := strings.Repeat("overflowing title ", 8) + "ZZEND"
	out.QuickEdit.Input.SetValue(value)
	out.QuickEdit.Input.CursorEnd()
	out.QuickEdit.RefreshDraft()

	lines := strings.Split(out.RenderOutlinerQuickEditRow(row, 80, nil), "\n")
	if len(lines) != 2 {
		t.Fatalf("quick edit rendered %d lines, want the metadata and input lines only", len(lines))
	}
	for _, line := range lines {
		if got := lipgloss.Width(ansi.Strip(line)); got > 80 {
			t.Fatalf("quick edit line is %d columns, want at most 80: %q", got, ansi.Strip(line))
		}
	}

	inputLine := ansi.Strip(lines[1])
	if !strings.Contains(inputLine, "ZZEND") {
		t.Errorf("tail at the cursor is not visible: %q", inputLine)
	}
	if strings.Contains(inputLine, "…") {
		t.Errorf("input line is ellipsized instead of horizontally scrolled: %q", inputLine)
	}
	if !strings.Contains(lines[1], "\x1b[7;") {
		t.Errorf("overflowing input line has no reverse-video cursor block: %q", lines[1])
	}

	narrow := ansi.Strip(strings.Split(out.RenderOutlinerQuickEditRow(row, 40, nil), "\n")[1])
	wide := ansi.Strip(strings.Split(out.RenderOutlinerQuickEditRow(row, 120, nil), "\n")[1])
	narrowVisible := longestVisibleSuffix(narrow, value)
	wideVisible := longestVisibleSuffix(wide, value)
	if wideVisible <= narrowVisible {
		t.Errorf("widening the row without a keystroke revealed %d of %d runes, narrow showed %d", wideVisible, len([]rune(value)), narrowVisible)
	}
}
