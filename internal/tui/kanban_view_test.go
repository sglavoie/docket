package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/testutil"
)

// newRenderKanban builds a minimal KanbanModel ready for rendering tests.
// cols is set directly; FilterMixin is initialized so filteredCards() works.
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
	for range n {
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

func TestKanbanRender_NonFocusedCardUsesPriorityTitleStyle(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	col := makeColN("inbox", 2)
	col.Tasks[0].Task.Title = "Priority card"
	col.Tasks[0].Task.Priority = 1
	col.Cursor = 1

	m := newRenderKanban(80, 24, []KanbanColumn{col})
	got := kanbanRender(m)
	want := GetTitlePriorityStyle(1).Render("Priority card")
	if !strings.Contains(got, want) {
		t.Fatalf("expected priority-styled card title in:\n%s", ansi.Strip(got))
	}
}

// ── overflow indicator ────────────────────────────────────────────────────────

func TestKanbanRender_OverflowIndicator(t *testing.T) {
	// height=10: fixedOverhead=6 → bodyRows=4
	// 10 tasks in column → first 3 visible, overflow shows ...+N more
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
	// width=50: maxVisible = (50+1)/26 = 1. With 5 columns, right side hidden.
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
	// Width=200: multiple columns fit → normal cycle mode hint
	cols := []KanbanColumn{makeColN("inbox", 1), makeColN("next", 1)}
	m := newRenderKanban(200, 24, cols)
	got := kanbanRender(m)
	if !strings.Contains(got, "cycle mode") {
		t.Errorf("wide terminal: expected cycle mode hint, got:\n%s", got)
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

func TestKanbanRender_DetailPane_ThreeLineLayout(t *testing.T) {
	project := testutil.NewTestProject("Data pipelines")
	task := testutil.NewTestTask("focus on shipping new pipeline with same set of permissions so I won't be blocked", models.TaskTypeTask)
	task.Priority = 2
	task.Tags = []string{"stale", "wip-old"}
	task.UpdatedAt = time.Now().Add(-2 * time.Hour)
	due := time.Now().Add(24 * time.Hour)
	task.DueDate = &due

	profile := scoring.NewWSJFProfile()
	cfg := testutil.NewTestConfig()
	cfg.ScoringProfiles = []scoring.ScoringProfile{profile}
	cfg.SetProjectScoringProfile(project.ID, profile.ID)

	m := newRenderKanban(96, 24, []KanbanColumn{{
		ID:    "inbox",
		Label: "inbox",
		Icon:  "·",
		Tasks: []KanbanCard{{Task: task, Project: project}},
	}})
	m.config = cfg
	m.detailPaneVisible = true

	lines := strippedDetailPaneLines(t, m)
	if len(lines) < 4 {
		t.Fatalf("expected separator plus three detail lines, got %d lines: %#v", len(lines), lines)
	}

	titleLine := lines[1]
	metadataLine := lines[2]
	tagsLine := lines[3]

	if !strings.Contains(titleLine, "focus on shipping new pipeline with same set of permissions") {
		t.Fatalf("title line should use the full pane width, got %q", titleLine)
	}
	if strings.Contains(titleLine, "│") || strings.Contains(titleLine, "Data pipelines") || strings.Contains(titleLine, "Tags:") {
		t.Fatalf("title line should not include metadata or tags, got %q", titleLine)
	}

	wantMetadata := []string{"Data pipelines", "●", "Due:", "[S:", "(WSJF)", "hours ago"}
	for _, want := range wantMetadata {
		if !strings.Contains(metadataLine, want) {
			t.Fatalf("metadata line missing %q: %q", want, metadataLine)
		}
	}
	assertInOrder(t, metadataLine, "Data pipelines", "●", "Due:", "[S:", "(WSJF)", "hours ago")

	if !strings.Contains(tagsLine, "Tags: #stale #wip-old") {
		t.Fatalf("third line should render tags, got %q", tagsLine)
	}
}

func TestKanbanRender_DetailPane_TagsBeforeBlockersAndWidthBounded(t *testing.T) {
	project := testutil.NewTestProject("p")
	blocker := testutil.NewTestTask("blocked by a dependency with a long title", models.TaskTypeTask)
	task := testutil.NewTestTask("this detail pane title is intentionally long enough to require truncation", models.TaskTypeTask)
	task.Tags = []string{"stale", "wip-old"}
	task.DependsOn = []string{blocker.ID}

	col := KanbanColumn{
		ID:    "inbox",
		Label: "inbox",
		Icon:  "·",
		Tasks: []KanbanCard{{Task: task, Project: project}},
	}
	m := newRenderKanban(54, 24, []KanbanColumn{col})
	m.detailPaneVisible = true
	m.allCards = []KanbanCard{
		{Task: task, Project: project},
		{Task: blocker, Project: project},
	}

	lines := strippedDetailPaneLines(t, m)
	if len(lines) < 4 {
		t.Fatalf("expected separator plus three detail lines, got %d lines: %#v", len(lines), lines)
	}
	thirdLine := lines[3]
	if !strings.Contains(lines[1], "…") {
		t.Fatalf("long detail title should truncate at pane width, got %q", lines[1])
	}
	assertInOrder(t, thirdLine, "Tags:", "Blocked by:")

	for i, line := range lines[:4] {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("detail pane line %d exceeds width %d: %q (width=%d)", i, m.width, line, width)
		}
	}
}

func strippedDetailPaneLines(t *testing.T, m *KanbanModel) []string {
	t.Helper()

	pane := m.renderKanbanDetailPane()
	rawLines := strings.Split(strings.TrimRight(pane, "\n"), "\n")
	lines := make([]string, len(rawLines))
	for i, line := range rawLines {
		lines[i] = ansi.Strip(line)
	}
	return lines
}

func assertInOrder(t *testing.T, text string, parts ...string) {
	t.Helper()

	pos := -1
	for _, part := range parts {
		next := strings.Index(text[pos+1:], part)
		if next < 0 {
			t.Fatalf("%q not found after index %d in %q", part, pos, text)
		}
		pos += 1 + next
	}
}

// ── minimal height (bodyRows clamped to 1) ──────────────────────────────────

func TestKanbanRender_MinimalHeight_NoPanic(t *testing.T) {
	// height=5: fixedOverhead=6 → bodyRows clamped to 1 (minimum).
	// Should render without panicking and still show a card row.
	cols := []KanbanColumn{makeColN("inbox", 3)}
	m := newRenderKanban(80, 5, cols)
	got := kanbanRender(m)
	if got == "" {
		t.Error("minimal height: expected non-empty render")
	}
	if !strings.Contains(got, "inbox") {
		t.Errorf("minimal height: expected column header 'inbox' in output")
	}
}

func TestKanbanRender_TinyHeight_BelowOverhead(t *testing.T) {
	// height=3: below fixedOverhead(6) → bodyRows clamped to 1.
	// Must not panic.
	cols := []KanbanColumn{makeColN("inbox", 5)}
	m := newRenderKanban(80, 3, cols)
	got := kanbanRender(m)
	if got == "" {
		t.Error("tiny height: expected non-empty render")
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

// TestKanbanVisibleBodyRows_MatchesRender_Overhead guards against the
// fixedOverhead drift between visibleBodyRows() (used by scroll/clamp logic)
// and kanbanRender() (the actual render budget). When the two diverge the
// cursor can fall outside the visible card area without scroll adjustment.
//
// Render computes: bodyRows = height - 6 (title+blank+header+divider+2 footer).
// visibleBodyRows() must agree.
func TestKanbanVisibleBodyRows_MatchesRender_Overhead(t *testing.T) {
	cols := []KanbanColumn{makeColN("inbox", 1)}
	m := newRenderKanban(80, 24, cols)
	// height=24 → expected bodyRows = 24-6 = 18.
	got := m.visibleBodyRows()
	want := 18
	if got != want {
		t.Errorf("visibleBodyRows() = %d, want %d (must equal render bodyRows = height - fixedOverhead(6))", got, want)
	}
}

// TestKanbanClampScrollToVisible_KeepsCursorInRenderedArea verifies that after
// clampScrollToVisible runs, the cursor actually falls within the rows the
// renderer will paint (not past the overflow indicator). This was broken when
// visibleBodyRows() reported 2 more rows than the renderer used.
func TestKanbanClampScrollToVisible_KeepsCursorInRenderedArea(t *testing.T) {
	cols := []KanbanColumn{makeColN("inbox", 7)}
	m := newRenderKanban(80, 10, cols) // bodyRows in render = 10-6 = 4
	m.columns[0].Cursor = 6            // last task
	m.clampScrollToVisible(0)

	// In the renderer, bodyRows=4 and the last row is reserved for the
	// "...+N more" indicator when content overflows. So at most 3 actual
	// card rows are visible, starting at ScrollOffset.
	const renderBodyRows = 4
	scrollOff := m.columns[0].ScrollOffset
	cursor := m.columns[0].Cursor
	// Overflow fires when len(tasks)-ScrollOffset > bodyRows, i.e. > 4.
	hasOverflow := len(m.columns[0].Tasks)-scrollOff > renderBodyRows
	visibleCards := renderBodyRows
	if hasOverflow {
		visibleCards = renderBodyRows - 1 // reserve last row for overflow indicator
	}
	if cursor < scrollOff || cursor >= scrollOff+visibleCards {
		t.Errorf("cursor %d not in rendered range [%d, %d) (visibleCards=%d, overflow=%v)",
			cursor, scrollOff, scrollOff+visibleCards, visibleCards, hasOverflow)
	}
}

// TestKanbanRender_NarrowMultiCol_HintAtTrueBoundary verifies the position
// indicator vs. cycle-mode hint switches at the width where the *actual*
// renderer drops to one visible column. The narrow-hint width math previously
// used minColWidth=20 (21 = 20+1) while the column layout uses minColWidth=25;
// at width=46 the renderer fits only one column (since (46+1)/26 = 1) but the
// hint logic believed two columns were visible and showed "cycle mode".
func TestKanbanRender_NarrowMultiCol_HintAtTrueBoundary(t *testing.T) {
	// At width=46 the renderer's maxVisible = (46+1)/(25+1) = 1.
	// With 3 columns total, the user sees only the active column, so the hint
	// must be the "col X/Y h/l" position indicator, not "cycle mode".
	cols := []KanbanColumn{makeColN("a", 1), makeColN("b", 1), makeColN("c", 1)}
	m := newRenderKanban(46, 24, cols)
	got := kanbanRender(m)
	if strings.Contains(got, "cycle mode") {
		t.Errorf("width=46 with 3 cols: expected position indicator (single col visible), got cycle-mode hint:\n%s", got)
	}
	if !strings.Contains(got, "col ") {
		t.Errorf("width=46 with 3 cols: expected 'col N/M' position indicator in output:\n%s", got)
	}
}
