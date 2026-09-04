package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TestOutlinerMultiSelectStatusFitsHeight guards Bug 3: entering multi-select
// sets a long status line and appends a "[MULTI-SELECT: n selected]" badge to
// the title. In a narrow tree pane both wrap to extra rows; the height budget
// used to reserve a fixed 2 lines for the status and 1 line for the title, so
// the wrapped rows pushed the render past the pane height. The status now
// renders in the shell-pinned strip, so the tree content plus the strip must
// still fit the pane height, and no rendered line may exceed the render width.
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

	m := NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	// Terminal 72x20 leaves the sub-view 18 rows (2 rows go to the app status
	// bar). Exercise the narrow split-pane tree width where the wrap bites.
	const paneHeight = 18
	m.SetSize(72, paneHeight)
	m.rebuildRows()
	m.MultiSelectMode = true
	m.SetStatus("Multi-select: Space=toggle, a=all, A=deselect, D=delete, v/Esc=exit")

	// 39 mirrors the ~55% tree pane width the split-pane layout allots at 72
	// cols, which forces both the status line and the title badge to wrap.
	const treeWidth = 39
	rendered := m.renderOutlinerTreeContent(treeWidth)
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
