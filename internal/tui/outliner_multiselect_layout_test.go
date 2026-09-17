package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TestOutlinerMultiSelectStatusFitsHeight guards Bug 3: a long status line and a
// "[MULTI-SELECT: n selected]" badge on the title both wrap in a narrow tree
// pane, and the height budget used to reserve a fixed 2 lines for the status and
// 1 for the title, so the wrapped rows pushed the render past the pane height.
// The status renders in the shell-pinned strip instead, so the tree content plus
// the strip must still fit the pane height and no rendered line may exceed the
// render width.
//
// Entering multi-select no longer writes that status — see
// TestMultiSelectStatesItselfOnceOnTheContextRow — so the wrapping message is
// set here directly. That is the point rather than a workaround: what this test
// measures is the pane arithmetic under a wrapped strip, and any wrapped strip
// will do.
func TestOutlinerMultiSelectStatusFitsHeight(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	cfg := newTestConfig()
	// Disable the Next section limit so a full viewport of rows does not emit a
	// wrapping "over section limit" warning (orthogonal to the status/badge
	// height accounting under test).

	// Enough task rows to fill the viewport so footer/header accounting matters.
	tasks := make([]*models.Task, 0, 12)
	for i := 0; i < 12; i++ {
		tasks = append(tasks, renderFixtureTask(t,
			fmt.Sprintf("row-%02d", i),
			i+1,
			fmt.Sprintf("Fill row %02d with a fairly long title", i),
			models.StateBacklog,
			3,
			models.StateBacklog))
	}
	project := renderFixtureProject(t, "proj-ms", "ms", "Multi Select", tasks)

	m := outliner.NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	// Terminal 72x20 leaves the sub-view 18 rows (2 rows go to the app status
	// bar). Exercise the narrow split-pane tree width where the wrap bites.
	const paneHeight = 18
	m.SetSize(72, paneHeight)
	m.RebuildRows()
	m.MultiSelectMode = true
	m.SetStatus("Multi-select: Space=toggle, a=all, A=deselect, D=delete, v/Esc=exit")

	// 39 mirrors the ~55% tree pane width the split-pane layout allots at 72
	// cols, which forces both the status line and the title badge to wrap.
	const treeWidth = 39
	rendered := m.RenderOutlinerTreeContent(treeWidth)
	strip := m.ViewStatusStrip(treeWidth)

	// The status line must actually wrap here, otherwise the test would pass
	// vacuously and not guard the regression.
	if got := lipgloss.Height(strip); got < 2 {
		t.Fatalf("precondition: status did not wrap at width %d (got %d line(s)); test would not guard the bug", treeWidth, got)
	}

	// Measure the render as the enclosing split-pane style does: constrained to
	// the tree width, so any over-wide line wraps into extra rows.
	wrapped := lipgloss.NewStyle().Width(treeWidth).Render(rendered)
	lines := strings.Split(strings.TrimRight(wrapped, "\n"), "\n")

	// The strip and its divider come out of the same pane budget.
	stripHeight := 1 + lipgloss.Height(strip)
	if len(lines)+stripHeight > paneHeight {
		t.Fatalf("multi-select tree content rendered %d lines plus a %d-line strip, exceeds pane height %d:\n%s",
			len(lines), stripHeight, paneHeight, ansi.Strip(wrapped))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > treeWidth {
			t.Fatalf("line %d exceeds tree width %d: width=%d stripped=%q", i, treeWidth, w, ansi.Strip(line))
		}
	}

	// Multi-select must remain functional: the badge and status hints render.
	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, "MULTI-SELECT") {
		t.Errorf("multi-select badge missing from render:\n%s", plain)
	}
	if !strings.Contains(ansi.Strip(strip), "Space=toggle") {
		t.Errorf("multi-select status hint missing from strip:\n%s", ansi.Strip(strip))
	}
}

// TestMultiSelectStatesItselfOnceOnTheContextRow is the selection summary's own
// contract, measured on the model rather than on a frame.
//
// Three claims. Entering and leaving multi-select writes nothing to the status
// channel, so a piece of operation feedback already standing there survives the
// gesture that is about to replace it — which is the whole reason the summary
// left that channel. Once members are folded out of view, both row spellings
// carry the hidden count: the compact one sheds the word `selected`, never the
// number, since a gesture reaching rows you cannot see is the one thing a narrow
// terminal must not quietly stop saying. And the `Value` `Q` reads is the whole
// breakdown exactly where there is a difference to read — with nothing hidden,
// the visible half is the total restated and the readout keeps its row.
func TestMultiSelectStatesItselfOnceOnTheContextRow(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	parent := renderFixtureTask(t, "ms-parent", 1, "Parent holding two children",
		models.StateBacklog, 3, models.StateBacklog)
	children := make([]*models.Task, 0, 2)
	for i := range 2 {
		child := renderFixtureTask(t, fmt.Sprintf("ms-child-%d", i), i+2,
			fmt.Sprintf("Child %d", i), models.StateBacklog, 3, models.StateBacklog)
		child.ParentID = &parent.ID
		children = append(children, child)
	}
	project := renderFixtureProject(t, "proj-ms-entry", "mse", "Multi Select Entry",
		append([]*models.Task{parent}, children...))

	cfg := newTestConfig()
	m := outliner.NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(100, 35)
	m.RebuildRows()

	const feedback = "Set Due Date: due today"
	m.SetStatus(feedback)
	m.ToggleMultiSelectMode()
	if m.StatusMessage != feedback {
		t.Fatalf("entering multi-select displaced the feedback: %q", m.StatusMessage)
	}

	m.SelectAllVisible()
	if counts := m.SelectionCounts(); counts.Total != 3 || counts.Hidden != 0 {
		t.Fatalf("the whole projection is not selected and visible: %+v", counts)
	}
	entry := multiSelectContextEntry(t, m, 100)
	if entry.Value != "3 selected" {
		t.Fatalf("the readout spells a breakdown with no difference in it: %q", entry.Value)
	}
	if strings.Contains(ansi.Strip(entry.Row), "hidden") {
		t.Fatalf("nothing is hidden, so the row spends no word on it: %q", ansi.Strip(entry.Row))
	}

	// Fold the parent and its two children leave the projection while staying in
	// the selection, which is the only state where the hidden half means anything.
	m.Cursor = rowIndexForFixtureTask(t, m, parent.ID)
	if !m.CollapseOrMoveToParent() {
		t.Fatal("the parent row did not fold")
	}
	m.RebuildRows()

	counts := m.SelectionCounts()
	if counts.Total != 3 || counts.Visible != 1 || counts.Hidden != 2 {
		t.Fatalf("folding did not move two members out of view: %+v", counts)
	}
	entry = multiSelectContextEntry(t, m, 100)
	if entry.Value != "3 total · 1 visible · 2 hidden" {
		t.Fatalf("the readout value does not carry all three numbers: %q", entry.Value)
	}
	full, compact := ansi.Strip(entry.Row), ansi.Strip(entry.RowCompact)
	if full != "MULTI-SELECT: 3 selected (2 hidden)" {
		t.Fatalf("the full spelling is %q", full)
	}
	if compact != "MULTI-SELECT: 3 (2 hidden)" {
		t.Fatalf("the compact spelling sheds the hidden count: %q", compact)
	}

	m.SetStatus(feedback)
	m.ToggleMultiSelectMode()
	if m.StatusMessage != feedback {
		t.Fatalf("leaving multi-select displaced the feedback: %q", m.StatusMessage)
	}
	if entries := m.BuildViewContextEntries(100); slices.ContainsFunc(entries,
		func(e outliner.ViewContextEntry) bool { return e.Label == "selection" }) {
		t.Fatal("the selection entry outlived the selection")
	}
}

// multiSelectContextEntry is the selection axis as the context row and the `Q`
// readout both read it, taken from the model's one description of its live axes.
func multiSelectContextEntry(t *testing.T, m *outliner.OutlinerModel, width int) outliner.ViewContextEntry {
	t.Helper()
	for _, entry := range m.BuildViewContextEntries(width) {
		if entry.Label == "selection" {
			return entry
		}
	}
	t.Fatalf("the live axes carry no selection entry at width %d", width)
	return outliner.ViewContextEntry{}
}

// rowIndexForFixtureTask seats the cursor by task identity rather than by a
// literal index, so an extra header row in the projection does not silently make
// the fold land somewhere else.
func rowIndexForFixtureTask(t *testing.T, m *outliner.OutlinerModel, taskID string) int {
	t.Helper()
	for index, row := range m.Rows {
		if row.Task != nil && row.Task.ID == taskID {
			return index
		}
	}
	t.Fatalf("no row draws task %q", taskID)
	return -1
}
