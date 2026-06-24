package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// crossProjectHeaderFixture builds a non-All cross-project outliner with two
// visible projects (alpha, beta), each contributing one in-progress task. The
// returned model has activePerspective set to Active so the row builder emits
// IsProjectHeader rows for each project group. The fixture is rebuilt rows-only
// (no disk I/O); callers may further mutate cursor/state before exercising the
// behavior under test.
func crossProjectHeaderFixture(t *testing.T) (m *OutlinerModel, alphaHeader, betaHeader int, alphaTaskRow, betaTaskRow int) {
	t.Helper()

	alphaTask := makeTask("alpha-active", "Alpha active task", "")
	alphaTask.Status = models.StatusInProgress
	alpha := &models.Project{ID: "proj-alpha", Name: "Alpha", Slug: "alpha", Tasks: []*models.Task{alphaTask}}
	alpha.BuildTaskTree()

	betaTask := makeTask("beta-active", "Beta active task", "")
	betaTask.Status = models.StatusInProgress
	beta := &models.Project{ID: "proj-beta", Name: "Beta", Slug: "beta", Tasks: []*models.Task{betaTask}}
	beta.BuildTaskTree()

	m = &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll + 1, // Active
		loadedProjects:    []*models.Project{alpha, beta},
	}
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	alphaHeader, betaHeader, alphaTaskRow, betaTaskRow = -1, -1, -1, -1
	for i, row := range m.rows {
		switch {
		case row.IsProjectHeader && row.ProjectID == alpha.ID:
			alphaHeader = i
		case row.IsProjectHeader && row.ProjectID == beta.ID:
			betaHeader = i
		case row.isTaskRow() && row.task != nil && row.task.ID == alphaTask.ID:
			alphaTaskRow = i
		case row.isTaskRow() && row.task != nil && row.task.ID == betaTask.ID:
			betaTaskRow = i
		}
	}
	if alphaHeader < 0 || betaHeader < 0 {
		t.Fatalf("project header fixture: expected one IsProjectHeader per project, got alpha=%d beta=%d", alphaHeader, betaHeader)
	}
	if alphaTaskRow < 0 || betaTaskRow < 0 {
		t.Fatalf("project header fixture: expected one task row per project, got alpha=%d beta=%d", alphaTaskRow, betaTaskRow)
	}
	return m, alphaHeader, betaHeader, alphaTaskRow, betaTaskRow
}

func TestOutlinerProjectHeaderEnterIsNoOp(t *testing.T) {
	m, alphaHeader, _, _, _ := crossProjectHeaderFixture(t)

	m.cursor = alphaHeader
	zoomBefore := len(m.crossZoomStack)
	rowsBefore := append([]outlinerRow(nil), m.rows...)

	model, cmd, handled := m.crossHandleEnterKey()
	if !handled {
		t.Fatal("crossHandleEnterKey returned handled=false for project header; cross-project mode must consume the key")
	}
	if cmd != nil {
		t.Fatalf("project header Enter must not emit a cmd, got %T", cmd())
	}
	if model != m {
		t.Fatalf("project header Enter must return the same model, got %T", model)
	}
	if len(m.crossZoomStack) != zoomBefore {
		t.Errorf("project header Enter must not push zoom stack: before=%d after=%d", zoomBefore, len(m.crossZoomStack))
	}
	if len(m.rows) != len(rowsBefore) {
		t.Errorf("project header Enter must not change row count: before=%d after=%d", len(rowsBefore), len(m.rows))
	}
}

func TestOutlinerProjectHeaderDrillDownIgnored(t *testing.T) {
	m, alphaHeader, _, _, _ := crossProjectHeaderFixture(t)

	m.cursor = alphaHeader
	_, cmd := m.crossHandleGoDrillDown()
	if cmd != nil {
		t.Fatalf("crossHandleGoDrillDown must return nil cmd for project header, got %T", cmd())
	}
}

func TestOutlinerProjectHeaderMutationIgnored(t *testing.T) {
	m, alphaHeader, _, _, _ := crossProjectHeaderFixture(t)

	// Snapshot the alpha task so we can detect any accidental mutation.
	alpha := m.loadedProjects[0]
	alphaTaskBefore := *alpha.Tasks[0]

	m.cursor = alphaHeader

	// Priority mutation: with cursor on a non-task row, crossGetMutationTarget
	// reports valid=false and the handler returns a nil cmd without touching
	// any project. Without persisting we cannot reload here; we directly
	// assert the in-memory task is untouched.
	if cmd, consumed := m.crossHandlePriorityMutation("1"); !consumed {
		t.Error("crossHandlePriorityMutation should consume known priority key even on project header")
	} else if cmd != nil {
		t.Errorf("crossHandlePriorityMutation on project header must return nil cmd, got %T", cmd())
	}
	if got := *alpha.Tasks[0]; got.Priority != alphaTaskBefore.Priority || got.Status != alphaTaskBefore.Status {
		t.Errorf("project header priority mutation leaked into task state: before=%+v after=%+v", alphaTaskBefore, got)
	}

	// Status mutation: same defensive contract.
	model, cmd, handled := m.crossHandleStatusMutation(models.StatusDone)
	if !handled {
		t.Error("crossHandleStatusMutation must report handled=true in cross-project mode even when cursor is on project header")
	}
	if cmd != nil {
		t.Errorf("crossHandleStatusMutation on project header must return nil cmd, got %T", cmd())
	}
	if model != m {
		t.Errorf("crossHandleStatusMutation on project header must return the same model, got %T", model)
	}
	if got := *alpha.Tasks[0]; got.Status != alphaTaskBefore.Status {
		t.Errorf("project header status mutation leaked into task state: before=%q after=%q", alphaTaskBefore.Status, got.Status)
	}
}

func TestOutlinerProjectHeaderSelectAllSkipped(t *testing.T) {
	m, _, _, alphaTaskRow, betaTaskRow := crossProjectHeaderFixture(t)

	m.multiSelectMode = true
	m.selectAllVisible()

	if len(m.selectedTasks) != 2 {
		t.Errorf("expected exactly 2 task selections (one per visible task), got %d: %v", len(m.selectedTasks), m.selectedTasks)
	}
	alphaTaskID := m.rows[alphaTaskRow].task.ID
	betaTaskID := m.rows[betaTaskRow].task.ID
	if !m.selectedTasks[alphaTaskID] {
		t.Errorf("alpha task %q should be selected", alphaTaskID)
	}
	if !m.selectedTasks[betaTaskID] {
		t.Errorf("beta task %q should be selected", betaTaskID)
	}

	// No project-header row should leak into the selection set under any key.
	for _, row := range m.rows {
		if row.IsProjectHeader && m.selectedTasks[row.ProjectID] {
			t.Errorf("project header ProjectID=%q leaked into selectedTasks", row.ProjectID)
		}
	}
}

func TestOutlinerProjectHeaderCursorSkip(t *testing.T) {
	m, alphaHeader, betaHeader, alphaTaskRow, betaTaskRow := crossProjectHeaderFixture(t)

	// Simulate a forward navigation that lands on the beta project header.
	// crossSkipNonNavigableRows should walk forward to the next navigable row
	// (the beta task row beneath the header).
	m.cursor = betaHeader
	prev := alphaTaskRow
	m.crossSkipNonNavigableRows(prev, nil)
	if m.cursor != betaTaskRow {
		t.Errorf("forward skip from project header: want cursor=%d (beta task), got %d", betaTaskRow, m.cursor)
	}

	// Simulate a backward navigation that lands on the alpha project header
	// from a position past it. The cursor should walk backward to a navigable
	// row, and since the alpha header is the very first row, the recovery
	// branch should then walk forward to the alpha task row.
	m.cursor = alphaHeader
	prev = betaTaskRow
	m.crossSkipNonNavigableRows(prev, nil)
	if m.cursor != alphaTaskRow {
		t.Errorf("backward skip from project header at list start: want cursor=%d (alpha task), got %d", alphaTaskRow, m.cursor)
	}
}
