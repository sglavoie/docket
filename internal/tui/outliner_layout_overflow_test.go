package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// TestOutlinerMultiSelectNestedRowsFitHeight guards the multi-select height
// overflow: the "[ ] " checkbox prefix plus a deep tree indent
// (│   └── ▾ ...) pushes a nested row past the render width. renderTaskTitle
// floors the title budget at 10 cols, so the assembled row exceeded rowWidth and
// the Width(rowWidth) row style *wrapped* it to a second terminal row — but the
// viewport budgets one line per row, so the wrapped rows overflowed pane height.
// Every rendered row must stay one line (no line wider than the render width) and
// the whole render must fit the pane height.
func TestOutlinerMultiSelectNestedRowsFitHeight(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	cfg := newTestConfig()
	cfg.Settings().SectionLimitNext = intPtr(0)

	// A deep hierarchy: epic → story → several leaf tasks. The leaves carry the
	// longest tree prefix, which is exactly where the checkbox tips the row into a
	// wrap at a narrow width.
	epic := renderFixtureTask(t, "epic", 1, models.TaskTypeEpic,
		"Launch the platform with a fairly long epic title", models.StatusTodo, 1, models.SectionNext)
	story := renderFixtureTask(t, "story", 2, models.TaskTypeStory,
		"User authentication and account onboarding flow", models.StatusTodo, 2, models.SectionNext)
	story.ParentID = &epic.ID
	tasks := []*models.Task{epic, story}
	for i := 0; i < 6; i++ {
		leaf := renderFixtureTask(t, fmt.Sprintf("leaf-%02d", i), i+3, models.TaskTypeTask,
			fmt.Sprintf("Build subsystem %02d with a comfortably long descriptive title", i),
			models.StatusTodo, 3, models.SectionNext)
		leaf.ParentID = &story.ID
		tasks = append(tasks, leaf)
	}
	project := renderFixtureProject(t, "proj-ms", "ms", "Multi Select", tasks)

	m := NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	const paneHeight = 18
	m.SetSize(72, paneHeight)
	m.rebuildRows()
	m.multiSelectMode = true
	m.setStatus("Multi-select: Space=toggle, a=all, A=deselect, D=delete, v/Esc=exit")

	// A narrow tree width where the deepest rows would wrap without the clamp.
	const treeWidth = 34
	rendered := m.renderOutlinerTreeContent(treeWidth)

	wrapped := lipgloss.NewStyle().Width(treeWidth).Render(rendered)
	lines := strings.Split(strings.TrimRight(wrapped, "\n"), "\n")

	if len(lines) > paneHeight {
		t.Fatalf("multi-select nested tree rendered %d lines, exceeds pane height %d:\n%s",
			len(lines), paneHeight, ansi.Strip(wrapped))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > treeWidth {
			t.Fatalf("row %d exceeds tree width %d: width=%d stripped=%q", i, treeWidth, w, ansi.Strip(line))
		}
	}

	// Multi-select must remain functional: every leaf row keeps its checkbox.
	if got := strings.Count(ansi.Strip(rendered), "[ ]"); got < 6 {
		t.Errorf("expected a checkbox on each of 6 leaf rows, found %d:\n%s", got, ansi.Strip(rendered))
	}
}

func TestOutlinerNarrowDeepRowKeepsTitleVisible(t *testing.T) {
	cfg := newTestConfig()
	task := renderFixtureTask(t, "deep", 9, models.TaskTypeTask,
		"Build authentication flow", models.StatusTodo, 3, models.SectionNext)
	project := renderFixtureProject(t, "proj-narrow", "narrow", "Narrow", []*models.Task{task})
	m := NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.multiSelectMode = true

	row := outlinerRow{task: task, prefix: "│   │   │   │   └── "}
	const width = 28
	rendered := m.renderOutlinerTaskRow(row, true, width, width, 0, nil)
	plain := ansi.Strip(rendered)
	if got := lipgloss.Width(rendered); got > width {
		t.Fatalf("narrow outliner row width = %d, want <= %d: %q", got, width, plain)
	}
	if !strings.Contains(plain, "Buil") {
		t.Fatalf("narrow outliner row spent its width before the task title: %q", plain)
	}
}

// TestOutlinerFullWidthHeaderFitsWidth guards the full-width header overflow:
// renderWithSidebar clamps content via Width(treeWidth), but renderFullWidth
// passed the tree through with no enclosing width style, so a long project name
// plus the perspective breadcrumb overflowed terminal width. renderFullWidth must
// now clamp to width so no rendered line exceeds it.
func TestOutlinerFullWidthHeaderFitsWidth(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	cfg := newTestConfig()

	task := renderFixtureTask(t, "t1", 1, models.TaskTypeTask,
		"A task", models.StatusTodo, 3, models.SectionNext)
	project := renderFixtureProject(t, "proj-hdr", "hdr",
		"A deliberately very long project name that will not fit a narrow terminal",
		[]*models.Task{task})

	m := NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	const width = 60
	const paneHeight = 20
	m.SetSize(width, paneHeight)
	m.rebuildRows()

	// Non-vacuous precondition: the assembled header genuinely overflows width, so
	// the clamp is what keeps the render in bounds.
	perspective := m.currentPerspective()
	perspectiveLabel := m.buildPerspectiveLabel(perspective)
	searchIndicator := m.buildOutlinerSearchIndicator()
	header := m.buildOutlinerTitleLine(width, perspectiveLabel, searchIndicator)
	if w := lipgloss.Width(header); w <= width {
		t.Fatalf("precondition: header width %d does not exceed %d; test would not guard the bug", w, width)
	}

	rendered := m.renderFullWidth()
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(lines) > paneHeight {
		t.Fatalf("full-width render produced %d lines, exceeds pane height %d:\n%s",
			len(lines), paneHeight, ansi.Strip(rendered))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("full-width line %d exceeds width %d: width=%d stripped=%q", i, width, w, ansi.Strip(line))
		}
	}
}

func TestOutlinerFullWidthContextNotesHeaderFitsWidth(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	cfg := newTestConfig()

	parent := renderFixtureTask(t, "story", 1, models.TaskTypeStory,
		"focus on shipping new pipeline with same set of permissions", models.StatusInProgress, 1, "")
	child := renderFixtureTask(t, "task", 2, models.TaskTypeTask,
		"Resume session to handle migration to Cloud Functions", models.StatusTodo, 3, "")
	child.ParentID = &parent.ID
	project := renderFixtureProject(t, "proj-real-title", "data-pipelines", "Data pipelines", []*models.Task{parent, child})
	project.Context = "tradition"
	notesDir := filepath.Join(project.Path, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatalf("create notes dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(notesDir, "note.md"), []byte("# Note\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}

	m := NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	const width = 60
	m.SetSize(width, 20)
	m.rebuildRows()

	rendered := m.renderFullWidth()
	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, "Data pipelines - Tasks [tradition]") || !strings.Contains(plain, "Notes") {
		t.Fatalf("test setup did not render context+notes title pressure:\n%s", plain)
	}
	for i, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("full-width context+notes line %d exceeds width %d: width=%d stripped=%q", i, width, w, ansi.Strip(line))
		}
	}
}
