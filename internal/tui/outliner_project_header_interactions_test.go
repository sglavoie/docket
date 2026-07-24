package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
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
	alphaTask.WorkflowState = models.StateInProgress
	alpha := &models.Project{ID: "proj-alpha", Name: "Alpha", Slug: "alpha", Tasks: []*models.Task{alphaTask}}
	alpha.BuildTaskTree()

	betaTask := makeTask("beta-active", "Beta active task", "")
	betaTask.WorkflowState = models.StateInProgress
	beta := &models.Project{ID: "proj-beta", Name: "Beta", Slug: "beta", Tasks: []*models.Task{betaTask}}
	beta.BuildTaskTree()

	m = &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		activePerspective: PerspectiveAll + 1, // Active
		loadedProjects:    []*models.Project{alpha, beta},
	}
	m.Dispatcher = keymap.NewDispatcher(m.buildKeyMap())
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

// navigateCursorTo drives j/k through the model's real keymap dispatcher until
// the cursor rests on target, mirroring the production key path. It fails the
// test if movement stalls before reaching target — this is how header rows are
// reached without assigning m.cursor directly.
func navigateCursorTo(t *testing.T, m *OutlinerModel, target int) {
	t.Helper()
	for guard := 0; m.cursor != target; guard++ {
		key := 'j'
		if m.cursor > target {
			key = 'k'
		}
		before := m.cursor
		m.Dispatcher.Dispatch(syntheticKey(key))
		if m.cursor == before {
			t.Fatalf("navigation stalled at cursor=%d trying to reach %d", m.cursor, target)
		}
		if guard > len(m.rows)+2 {
			t.Fatalf("navigation failed to converge on cursor=%d (stuck at %d)", target, m.cursor)
		}
	}
}

func TestOutlinerProjectHeadersAreNavigable(t *testing.T) {
	// Separators stay non-navigable while project headers become navigable.
	if (outlinerRow{IsSeparator: true}).isNavigable() {
		t.Error("separator rows must remain non-navigable")
	}
	if !(outlinerRow{IsProjectHeader: true}).isNavigable() {
		t.Error("project header rows must be navigable so Enter/Shift+Enter are reachable")
	}

	m, alphaHeader, betaHeader, alphaTaskRow, betaTaskRow := crossProjectHeaderFixture(t)

	// After a rebuild the clamp is allowed to settle the cursor on the first
	// project header rather than skipping past it to the task row.
	if !m.rows[m.cursor].IsProjectHeader {
		t.Fatalf("expected clamp to keep cursor on a project header, got row %d (%+v)", m.cursor, m.rows[m.cursor])
	}

	// Walk downward with j to the last row; record every cursor stop so we can
	// assert that both project headers are landed on (not skipped) during normal
	// movement. Walking to the last row index keeps the assertion independent of
	// project group ordering.
	lastRow := len(m.rows) - 1
	visited := map[int]bool{m.cursor: true}
	for guard := 0; m.cursor < lastRow; guard++ {
		before := m.cursor
		m.Dispatcher.Dispatch(syntheticKey('j'))
		if m.cursor == before {
			t.Fatalf("j stalled at cursor=%d", m.cursor)
		}
		visited[m.cursor] = true
		if guard > len(m.rows)+2 {
			t.Fatal("j navigation did not converge on the last row")
		}
	}
	for _, want := range []struct {
		name string
		idx  int
	}{
		{"alpha header", alphaHeader},
		{"alpha task", alphaTaskRow},
		{"beta header", betaHeader},
		{"beta task", betaTaskRow},
	} {
		if !visited[want.idx] {
			t.Errorf("downward j movement skipped %s (row %d); every visible header and task must be reachable", want.name, want.idx)
		}
	}

	// Walk back up with k and confirm each project header is landed on again.
	navigateCursorTo(t, m, betaHeader)
	if !m.rows[m.cursor].IsProjectHeader || m.rows[m.cursor].ProjectID != m.rows[betaHeader].ProjectID {
		t.Fatalf("k movement failed to rest on the beta project header, got row %d (%+v)", m.cursor, m.rows[m.cursor])
	}
	navigateCursorTo(t, m, alphaHeader)
	if !m.rows[m.cursor].IsProjectHeader || m.rows[m.cursor].ProjectID != m.rows[alphaHeader].ProjectID {
		t.Fatalf("k movement failed to rest on the alpha project header, got row %d (%+v)", m.cursor, m.rows[m.cursor])
	}
}

func TestOutlinerProjectHeaderEnterPushesScope(t *testing.T) {
	m, alphaHeader, _, _, _ := crossProjectHeaderFixture(t)
	alphaID := m.rows[alphaHeader].ProjectID

	// Reach the header through real j/k navigation rather than a direct cursor
	// assignment, proving the header is a legitimate cursor destination.
	navigateCursorTo(t, m, alphaHeader)
	if !m.rows[m.cursor].IsProjectHeader || m.rows[m.cursor].ProjectID != alphaID {
		t.Fatalf("navigation did not land on the alpha project header, got row %d (%+v)", m.cursor, m.rows[m.cursor])
	}
	zoomBefore := len(m.crossZoomStack)

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
	// Enter now scopes the global Outliner to the header's project in place.
	if m.scopedProjectID != alphaID {
		t.Fatalf("expected Enter on project header to scope to %q, got %q", alphaID, m.scopedProjectID)
	}
	if len(m.crossZoomStack) != zoomBefore {
		t.Errorf("project header Enter must scope, not push zoom: before=%d after=%d", zoomBefore, len(m.crossZoomStack))
	}
	// The scoped view shows only alpha's rows.
	for _, row := range m.rows {
		if row.isTaskRow() && row.task != nil && row.ProjectID != alphaID {
			t.Fatalf("scoped rows must belong to %q, found %q", alphaID, row.ProjectID)
		}
	}
}

func TestOutlinerProjectHeaderMutationIgnored(t *testing.T) {
	m, alphaHeader, _, _, _ := crossProjectHeaderFixture(t)

	// Snapshot the alpha task so we can detect any accidental mutation.
	alpha := m.loadedProjects[0]
	alphaTaskBefore := *alpha.Tasks[0]

	navigateCursorTo(t, m, alphaHeader)

	// Priority mutation: with cursor on a non-task row, crossGetMutationTarget
	// reports valid=false and the handler returns a nil cmd without touching
	// any project. Without persisting we cannot reload here; we directly
	// assert the in-memory task is untouched.
	if cmd, consumed := m.crossHandlePriorityMutation("1"); !consumed {
		t.Error("crossHandlePriorityMutation should consume known priority key even on project header")
	} else if cmd != nil {
		t.Errorf("crossHandlePriorityMutation on project header must return nil cmd, got %T", cmd())
	}
	if got := *alpha.Tasks[0]; got.Priority != alphaTaskBefore.Priority || got.State() != alphaTaskBefore.State() {
		t.Errorf("project header priority mutation leaked into task state: before=%+v after=%+v", alphaTaskBefore, got)
	}

	// Status mutation: same defensive contract.
	model, cmd, handled := m.crossHandleStateMutation(models.StateDone)
	if !handled {
		t.Error("crossHandleStateMutation must report handled=true in cross-project mode even when cursor is on project header")
	}
	if cmd != nil {
		t.Errorf("crossHandleStateMutation on project header must return nil cmd, got %T", cmd())
	}
	if model != m {
		t.Errorf("crossHandleStateMutation on project header must return the same model, got %T", model)
	}
	if got := *alpha.Tasks[0]; got.State() != alphaTaskBefore.State() {
		t.Errorf("project header status mutation leaked into task state: before=%q after=%q", alphaTaskBefore.State(), got.State())
	}
}

func TestOutlinerProjectHeaderSelectAllSkipped(t *testing.T) {
	m, _, _, alphaTaskRow, betaTaskRow := crossProjectHeaderFixture(t)

	m.multiSelectMode = true
	m.selectAllVisible()

	if m.SelectedCount() != 2 {
		t.Errorf("expected exactly 2 task selections (one per visible task), got %d: %v", m.SelectedCount(), m.selectedTaskIdentities)
	}
	if !m.isRowSelected(m.rows[alphaTaskRow]) {
		t.Errorf("alpha task %q should be selected", m.rows[alphaTaskRow].task.ID)
	}
	if !m.isRowSelected(m.rows[betaTaskRow]) {
		t.Errorf("beta task %q should be selected", m.rows[betaTaskRow].task.ID)
	}

	// No project-header row should leak into the selection set under any key.
	for _, row := range m.rows {
		if row.IsProjectHeader && m.IsTaskIdentitySelected(TaskIdentity{ProjectID: row.ProjectID, TaskID: row.ProjectID}) {
			t.Errorf("project header ProjectID=%q leaked into selectedTaskIdentities", row.ProjectID)
		}
	}
}

// TestOutlinerProjectHeaderSeparatorStillSkipped confirms that the
// skip-non-navigable path still steps over separators even though project
// headers are now valid cursor destinations. Project headers are no longer
// skipped (see TestOutlinerProjectHeadersAreNavigable); only decorative
// separators remain non-navigable.
func TestOutlinerProjectHeaderSeparatorStillSkipped(t *testing.T) {
	m, alphaHeader, _, alphaTaskRow, _ := crossProjectHeaderFixture(t)

	// Splice a separator immediately after the alpha task row.
	sepIdx := alphaTaskRow + 1
	m.rows = append(m.rows[:sepIdx], append([]outlinerRow{{IsSeparator: true}}, m.rows[sepIdx:]...)...)

	// Landing on the separator via a forward move must skip to the next
	// navigable row (which is now a project header — itself navigable).
	m.cursor = sepIdx
	m.crossSkipNonNavigableRows(alphaTaskRow, nil)
	if m.rows[m.cursor].IsSeparator {
		t.Fatalf("forward skip left cursor on a separator at row %d", m.cursor)
	}
	if m.cursor <= sepIdx {
		t.Fatalf("forward skip from separator should advance past row %d, got %d", sepIdx, m.cursor)
	}

	// The alpha project header (before the separator) is a valid rest position:
	// a backward move onto it must NOT skip away.
	m.cursor = alphaHeader
	m.crossSkipNonNavigableRows(alphaTaskRow, nil)
	if m.cursor != alphaHeader {
		t.Fatalf("backward move onto navigable project header should rest there, got %d", m.cursor)
	}
}
