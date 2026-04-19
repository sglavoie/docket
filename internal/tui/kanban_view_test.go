package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newRenderKanban builds a minimal KanbanModel ready for rendering tests.
// cols is set directly; FilterMixin is initialised so filteredCards() works.
func newRenderKanban(width, height int, cols []KanbanColumn) *KanbanModel {
	cfg := testutil.NewTestConfig()
	m := &KanbanModel{
		columns:     cols,
		activeCol:   0,
		mode:        KanbanModeSection,
		width:       width,
		height:      height,
		config:      cfg,
		FilterMixin: FilterMixin{filter: newFilterState()},
	}
	return m
}

// makeColN creates a KanbanColumn with n tasks (convenience wrapper).
func makeColN(id string, n int) KanbanColumn {
	proj := testutil.NewTestProject("p")
	col := KanbanColumn{ID: id, Label: id, Icon: "·"}
	for i := 0; i < n; i++ {
		t := testutil.NewTestTask("A title that may be long", models.TaskTypeTask)
		col.Tasks = append(col.Tasks, KanbanCard{Task: t, Project: proj})
	}
	return col
}

// ── basic rendering ───────────────────────────────────────────────────────────

func TestKanbanRender_ZeroSize_ReturnsLoading(t *testing.T) {
	m := newRenderKanban(0, 0, nil)
	got := kanbanRender(m)
	if !strings.Contains(got, "loading") {
		t.Errorf("zero-size render: want loading message, got %q", got)
	}
}

func TestKanbanRender_NoCols_ShowsMessage(t *testing.T) {
	m := newRenderKanban(80, 24, nil)
	got := kanbanRender(m)
	if !strings.Contains(got, "No tasks") {
		t.Errorf("no columns: want 'No tasks' message, got %q", got)
	}
}

func TestKanbanRender_Basic_ContainsColumnHeader(t *testing.T) {
	cols := []KanbanColumn{makeColN("inbox", 2)}
	m := newRenderKanban(80, 24, cols)
	got := kanbanRender(m)
	if !strings.Contains(got, "inbox") {
		t.Errorf("expected column header 'inbox' in output")
	}
}

func TestKanbanRender_ContainsTaskTitle(t *testing.T) {
	col := makeColN("inbox", 1)
	col.Tasks[0].Task.Title = "My special task"
	m := newRenderKanban(80, 24, []KanbanColumn{col})
	got := kanbanRender(m)
	if !strings.Contains(got, "My special task") {
		t.Errorf("expected task title in output")
	}
}

// ── overflow indicator ────────────────────────────────────────────────────────

func TestKanbanRender_OverflowIndicator(t *testing.T) {
	// height=10: fixedOverhead=4 → bodyRows=6
	// 10 tasks in column → first 5 visible, overflow shows ...+5 more
	cols := []KanbanColumn{makeColN("inbox", 10)}
	m := newRenderKanban(80, 10, cols)
	got := kanbanRender(m)
	if !strings.Contains(got, "more") {
		t.Errorf("expected overflow '...+N more' indicator, got:\n%s", got)
	}
}

func TestKanbanRender_NoOverflow_WhenAllFit(t *testing.T) {
	// height=30: bodyRows=26 — plenty for 3 tasks
	cols := []KanbanColumn{makeColN("inbox", 3)}
	m := newRenderKanban(80, 30, cols)
	got := kanbanRender(m)
	if strings.Contains(got, "more") {
		t.Errorf("unexpected overflow indicator when tasks fit: %s", got)
	}
}

// ── horizontal scroll indicators ─────────────────────────────────────────────

func TestKanbanRender_RightScrollHint_WhenColumnsExceedWidth(t *testing.T) {
	// width=50: maxVisible = (50+1)/21 = 2. With 5 columns, right side hidden.
	cols := make([]KanbanColumn, 5)
	for i := range cols {
		cols[i] = makeColN("col", 1)
	}
	m := newRenderKanban(50, 24, cols)
	got := kanbanRender(m)
	if !strings.Contains(got, "▶") {
		t.Errorf("expected right-scroll indicator ▶ when columns overflow")
	}
}

func TestKanbanRender_LeftScrollHint_WhenScrolledRight(t *testing.T) {
	cols := make([]KanbanColumn, 5)
	for i := range cols {
		cols[i] = makeColN("col", 1)
	}
	m := newRenderKanban(50, 24, cols)
	m.activeCol = 3
	m.colScrollOff = 2 // scrolled past first two columns
	got := kanbanRender(m)
	if !strings.Contains(got, "◀") {
		t.Errorf("expected left-scroll indicator ◀ when scrolled right")
	}
}

func TestKanbanRender_NoScrollHints_WhenAllColumnsVisible(t *testing.T) {
	// width=200: room for all 3 columns
	cols := []KanbanColumn{makeColN("a", 1), makeColN("b", 1), makeColN("c", 1)}
	m := newRenderKanban(200, 24, cols)
	got := kanbanRender(m)
	if strings.Contains(got, "◀") || strings.Contains(got, "▶") {
		t.Errorf("unexpected scroll indicators when all columns fit")
	}
}

// ── column width calculation ──────────────────────────────────────────────────

func TestKanbanRender_NarrowTerminal_SingleColumnMode(t *testing.T) {
	// Width=30: maxVisible=1 → render must not panic and must show active column.
	cols := []KanbanColumn{makeColN("inbox", 1), makeColN("next", 1)}
	m := newRenderKanban(30, 24, cols)
	got := kanbanRender(m)
	// Basic sanity: output is non-empty and contains the active column name.
	if got == "" {
		t.Error("narrow terminal: expected non-empty render")
	}
	if !strings.Contains(got, "inbox") {
		t.Errorf("narrow terminal: expected active column 'inbox' in output, got:\n%s", got)
	}
}

func TestKanbanRender_WideTerminal_NormalHint(t *testing.T) {
	// Width=200: multiple columns fit → normal ctrl+g hint
	cols := []KanbanColumn{makeColN("inbox", 1), makeColN("next", 1)}
	m := newRenderKanban(200, 24, cols)
	got := kanbanRender(m)
	if !strings.Contains(got, "ctrl+g") {
		t.Errorf("wide terminal: expected ctrl+g hint, got:\n%s", got)
	}
}

// ── detail pane ───────────────────────────────────────────────────────────────

func TestKanbanRender_DetailPane_VisibleWhenEnabled(t *testing.T) {
	col := makeColN("inbox", 1)
	col.Tasks[0].Task.Title = "Detail pane task"
	m := newRenderKanban(80, 24, []KanbanColumn{col})
	m.detailPaneVisible = true
	got := kanbanRender(m)
	// Detail pane separator should be present
	if !strings.Contains(got, "─") {
		t.Errorf("detail pane: expected separator line")
	}
}

// ── long title truncation ─────────────────────────────────────────────────────

func TestKanbanRender_LongTitleTruncated(t *testing.T) {
	longTitle := strings.Repeat("X", 200)
	col := makeColN("inbox", 1)
	col.Tasks[0].Task.Title = longTitle
	m := newRenderKanban(80, 24, []KanbanColumn{col})
	got := kanbanRender(m)
	// The full 200-char title should not appear verbatim
	if strings.Contains(got, longTitle) {
		t.Error("long title should be truncated")
	}
}
