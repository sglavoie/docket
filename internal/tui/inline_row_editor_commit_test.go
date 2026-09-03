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

	result, err := parseQuickEdit(
		"Renamed !2 @0 $today #new -#alpha %destination >7",
		child, source, []*models.Project{source, destination},
	)
	if err != nil {
		t.Fatalf("parse cross-project row edit: %v", err)
	}
	req := taskLineEditRequest(taskref.For(child, source), result)
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
	out := NewOutlinerModel(m.currentProject, m.configManager, m.runtime)
	out.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	out.SetSize(100, 20)
	out.rebuildRows()
	out.expandedNodes[taskref.For(task, m.currentProject)] = false
	out.collapsedSections["unrelated"] = true
	m.crossOutliner = out
	m.navigation.ResetRoot()
	projection := out.jumpProjectionFingerprint()

	out.Update(StartTaskLineEditMsg{Task: task, Project: m.currentProject})
	out.quickEdit.input.SetValue("Renamed atomically")
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()
	_, cmd := out.handleQuickEditMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !out.quickEdit.IsActive() || !out.quickEdit.pending {
		t.Fatalf("commit did not retain a pending inline draft: active=%t pending=%t", out.quickEdit.IsActive(), out.quickEdit.pending)
	}
	committed, ok := cmd().(coordinator.TaskLineEditedMsg)
	if !ok || !committed.Result.Changed || committed.Result.TransactionID == "" {
		t.Fatalf("atomic command result = %#v", committed)
	}
	m.handleTaskLineEdited(committed)
	if out.quickEdit.IsActive() {
		t.Fatal("successful atomic edit left the row editor active")
	}
	if got := m.currentProject.GetTaskByID(task.ID).Title; got != "Renamed atomically" {
		t.Fatalf("reloaded title = %q, want Renamed atomically", got)
	}
	if got := out.jumpProjectionFingerprint(); got != projection {
		t.Fatalf("successful inline edit changed projection state:\n before %s\n after  %s", projection, got)
	}

	live := m.currentProject.GetTaskByID(task.ID)
	out.Update(StartTaskLineEditMsg{Task: live, Project: m.currentProject})
	out.quickEdit.input.SetValue("Draft survives")
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()
	out.quickEdit.BeginCommit()
	projection = out.jumpProjectionFingerprint()
	failure := errors.New("simulated revision conflict")
	m.handleTaskLineEditFailed(coordinator.TaskLineEditFailedMsg{
		Source: taskref.For(live, m.currentProject), Err: failure,
	})
	if !out.quickEdit.IsActive() || out.quickEdit.pending || out.quickEdit.input.Value() != "Draft survives" {
		t.Fatalf("failed commit lost draft: active=%t pending=%t value=%q", out.quickEdit.IsActive(), out.quickEdit.pending, out.quickEdit.input.Value())
	}
	if !strings.Contains(out.quickEdit.GetPreviewError(), failure.Error()) {
		t.Fatalf("inline failure = %q, want %q", out.quickEdit.GetPreviewError(), failure)
	}
	if got := out.jumpProjectionFingerprint(); got != projection {
		t.Fatalf("failed inline edit changed projection state:\n before %s\n after  %s", projection, got)
	}
}

func TestInlineRowEditorUnchangedCommitCreatesNoUndoTransaction(t *testing.T) {
	m, _, task, _ := newPlanningLedgerApp(t, models.StateBacklog)
	out := NewOutlinerModel(m.currentProject, m.configManager, m.runtime)
	out.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	out.SetSize(100, 20)
	out.rebuildRows()
	m.crossOutliner = out
	m.navigation.ResetRoot()

	before, err := m.deps.Ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("undo count before unchanged edit: %v", err)
	}

	out.Update(StartTaskLineEditMsg{Task: task, Project: m.currentProject})
	if !out.quickEdit.IsActive() || out.quickEdit.input.Value() != task.Title {
		t.Fatalf("inline editor did not open with unchanged title: active=%t value=%q", out.quickEdit.IsActive(), out.quickEdit.input.Value())
	}
	_, cmd := out.handleQuickEditMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("unchanged inline commit returned nil command")
	}
	committed, ok := cmd().(coordinator.TaskLineEditedMsg)
	if !ok || committed.Result.Changed || committed.Result.TransactionID != "" {
		t.Fatalf("unchanged inline result = %#v", committed)
	}
	m.handleTaskLineEdited(committed)

	if out.quickEdit.IsActive() {
		t.Fatal("unchanged inline commit left the row editor active")
	}
	if m.statusMessage != "No task changes" {
		t.Fatalf("unchanged inline status = %q, want No task changes", m.statusMessage)
	}
	after, err := m.deps.Ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("undo count after unchanged edit: %v", err)
	}
	if after != before {
		t.Fatalf("unchanged inline edit created an undo entry: before=%d after=%d", before, after)
	}
}

func TestTaskLineEditRelocationFollowsVisibleResultAndFallsBackAtOrigin(t *testing.T) {
	makeOutliner := func() (*OutlinerModel, *models.Project, *models.Project, *models.Task) {
		task := &models.Task{ID: "moving", ShortID: 1, Title: "Moving", WorkflowState: models.StateBacklog}
		neighbor := &models.Task{ID: "neighbor", ShortID: 2, Title: "Neighbor", WorkflowState: models.StateBacklog}
		source := &models.Project{ID: "alpha-source", Slug: "alpha-source", Name: "Source", Tasks: []*models.Task{task, neighbor}}
		destination := &models.Project{ID: "zeta-destination", Slug: "zeta-destination", Name: "Destination"}
		out := NewOutlinerModel(nil, nil, nil)
		out.loadedProjects = []*models.Project{source, destination}
		out.crossSnapshotReady = true
		out.crossForceRebuild = true
		out.crossProjectRebuildRows()
		for index, row := range out.rows {
			if row.task != nil && row.task.ID == task.ID && row.ProjectID == source.ID {
				out.cursor = index
			}
		}
		out.syncJumpProjection()
		return out, source, destination, task
	}

	out, source, destination, task := makeOutliner()
	capture := out.captureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(task, source)})
	source.Tasks = source.Tasks[1:]
	destination.Tasks = append(destination.Tasks, task)
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	_, recorded := out.finishTaskLineEditRelocation(capture, taskref.For(task, destination))
	if !recorded || out.currentTaskIdentity() != taskref.For(task, destination) || len(out.relocationHistory.back) != 1 {
		t.Fatalf("visible relocation = current %+v recorded=%t history=%d", out.currentTaskIdentity(), recorded, len(out.relocationHistory.back))
	}

	out, source, _, task = makeOutliner()
	capture = out.captureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(task, source)})
	source.Tasks = source.Tasks[1:]
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	_, recorded = out.finishTaskLineEditRelocation(capture, taskref.TaskIdentity{ProjectID: "hidden", TaskID: task.ID})
	if recorded || out.currentTaskIdentity().TaskID != "neighbor" || len(out.relocationHistory.back) != 0 {
		t.Fatalf("hidden relocation = current %+v recorded=%t history=%d", out.currentTaskIdentity(), recorded, len(out.relocationHistory.back))
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
	out.quickEdit.EnterWithContext(child, project, []*models.Project{project}, false)
	row := out.rows[out.cursor]

	// The first frame is what installs the row width, so the value has to land
	// afterwards for the scroll window to be the one an editing session sees.
	out.renderOutlinerQuickEditRow(row, 80)
	value := strings.Repeat("overflowing title ", 8) + "ZZEND"
	out.quickEdit.input.SetValue(value)
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()

	lines := strings.Split(out.renderOutlinerQuickEditRow(row, 80), "\n")
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

	narrow := ansi.Strip(strings.Split(out.renderOutlinerQuickEditRow(row, 40), "\n")[1])
	wide := ansi.Strip(strings.Split(out.renderOutlinerQuickEditRow(row, 120), "\n")[1])
	narrowVisible := longestVisibleSuffix(narrow, value)
	wideVisible := longestVisibleSuffix(wide, value)
	if wideVisible <= narrowVisible {
		t.Errorf("widening the row without a keystroke revealed %d of %d runes, narrow showed %d", wideVisible, len([]rune(value)), narrowVisible)
	}
}
