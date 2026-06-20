package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectOutlinerRenderDarkThemeVisualElements(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const width = 112
	m, _ := crossProjectRenderFixture(t, width, 18)

	rendered := m.renderOutlinerTreeContent(width)
	plain := ansi.Strip(rendered)

	assertContains(t, plain, "Cross-project Outliner (2 projects)", "title")
	assertContains(t, plain, "‹ Snoozed", "previous perspective label")
	assertContains(t, plain, "@ All", "current perspective label")
	assertContains(t, plain, "Active ›", "next perspective label")
	assertContains(t, plain, IconExpanded, "expanded section indicator")
	assertContains(t, plain, "@ Inbox", "Inbox section label")
	assertContains(t, plain, "(1)", "Inbox section count")
	assertContains(t, plain, "@ Next", "Next section label")
	assertContains(t, plain, "(2)", "Next section count")
	assertContains(t, rendered, MutedStyle.Render("‹ Snoozed"), "styled previous perspective")
	assertContains(t, rendered, GetViewModeStyleForSection("All").Render("@ All"), "styled current perspective")
	assertContains(t, rendered, MutedStyle.Render("Active ›"), "styled next perspective")
	assertContains(t, rendered, GetViewModeStyleForSection(models.SectionInbox).Bold(true).Render("@ Inbox"), "styled Inbox section")
	assertContains(t, rendered, GetViewModeStyleForSection(models.SectionNext).Bold(true).Render("@ Next"), "styled Next section")
	assertContains(t, rendered, MutedStyle.Render("(1)"), "styled Inbox count")
	assertContains(t, rendered, MutedStyle.Render("(2)"), "styled Next count")
	assertContains(t, rendered, renderedProjectBadge("Alpha Project", "alpha"), "styled Alpha project badge")
	assertContains(t, rendered, renderedProjectBadge("Beta Project", "beta"), "styled Beta project badge")

	selectedLine := findRenderedLineContaining(t, rendered, "#42")
	selectedPlain := ansi.Strip(selectedLine)
	assertContains(t, selectedPlain, "[alpha]", "selected project badge text")
	assertContains(t, selectedPlain, "Selected child carries rendered metadata clearly", "selected title")
	assertContains(t, selectedPlain, "#42", "selected short ID metadata")
	assertContains(t, selectedPlain, "#ops #today", "selected tag metadata")
	if selectedLine == selectedPlain {
		t.Fatalf("selected row has no ANSI styling applied: %q", selectedLine)
	}
	if strings.Contains(selectedLine, renderedProjectBadge("Alpha Project", "alpha")) {
		t.Fatalf("selected row retained nested project badge styling instead of selected-row repainting: %q", selectedLine)
	}

	for i, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("rendered line %d exceeds width %d: width=%d stripped=%q", i, width, got, ansi.Strip(line))
		}
	}
}

func TestCrossProjectOutlinerRenderTreePrefixesFoldIndicatorsAndTruncation(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const rowWidth = 72
	m, selectedTask := crossProjectRenderFixture(t, 112, 18)
	parentIdx := findOutlinerRowIndex(t, m.rows, "alpha-parent")
	childIdx := findOutlinerRowIndex(t, m.rows, selectedTask.ID)

	parentLine := m.renderOutlinerTaskRow(m.rows[parentIdx], false, rowWidth, rowWidth, parentIdx, nil)
	parentPlain := ansi.Strip(parentLine)
	assertContains(t, parentPlain, connectorLastItem+IconExpanded, "expanded parent tree prefix")
	assertContains(t, parentPlain, "[alpha]", "parent project badge")

	childLine := m.renderOutlinerTaskRow(m.rows[childIdx], true, rowWidth, rowWidth, childIdx, nil)
	childPlain := ansi.Strip(childLine)
	assertContains(t, childPlain, connectorSpace+"└────", "leaf tree prefix substitute")
	assertContains(t, childPlain, "[alpha]", "child project badge")
	assertContains(t, childPlain, "#42", "child short ID metadata")
	assertContains(t, childPlain, "#ops #today", "child tag metadata")
	assertContains(t, childPlain, "Selected child ca", "truncated child title prefix")
	if strings.Contains(childPlain, selectedTask.Title) {
		t.Fatalf("expected child title to be truncated at width %d, got full title in %q", rowWidth, childPlain)
	}
	if got := lipgloss.Width(childLine); got != rowWidth {
		t.Fatalf("selected child row width = %d, want %d, stripped=%q", got, rowWidth, childPlain)
	}
	if childLine == childPlain {
		t.Fatalf("selected child row has no ANSI styling applied: %q", childLine)
	}
}

func TestCrossProjectOutlinerRenderEmojiPreviewScrollbarStaysWithinWidth(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })
	icons.SetUseEmoji(true)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	const width = 80
	m, selectedTask := crossProjectRenderFixture(t, width, 8)
	selectedTask.Description = "Preview details carry emoji-safe width pressure with enough text to fill the reserved content area before the scrollbar column."
	m.rows[m.cursor].task.Description = selectedTask.Description
	m.previewMode = true

	rendered := m.renderOutlinerTreeContent(width)
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	scrollbarLines := 0
	previewLines := 0
	for i, line := range lines {
		got := lipgloss.Width(line)
		if got > width {
			t.Fatalf("rendered line %d exceeds width %d: width=%d stripped=%q", i, width, got, ansi.Strip(line))
		}

		plain := ansi.Strip(line)
		if hasScrollbarGlyph(plain) {
			scrollbarLines++
			if got != width {
				t.Fatalf("scrollbar line %d width = %d, want %d, stripped=%q", i, got, width, plain)
			}
		}
		if strings.Contains(plain, "Preview details") {
			previewLines++
			if got != width-1 {
				t.Fatalf("preview line %d width = %d, want reserved row width %d, stripped=%q", i, got, width-1, plain)
			}
		}
	}
	if scrollbarLines == 0 {
		t.Fatalf("expected forced overflow to render scrollbar, got:\n%s", ansi.Strip(rendered))
	}
	if previewLines == 0 {
		t.Fatalf("expected selected row preview lines, got:\n%s", ansi.Strip(rendered))
	}
}

func TestCrossProjectOutlinerRenderIgnoresAggregateCustomTabs(t *testing.T) {
	const width = 80
	cfg := config.NewConfig()
	m := NewOutlinerModel(nil, cfg, nil)
	m.SetSize(width, 12)

	task := renderFixtureTask(t, "sprint-task", 11, models.TaskTypeTask, "Sprint task", models.StatusTodo, 3, "")
	task.Section = "Sprint-1"
	project := renderFixtureProject(t, "proj-alpha", "alpha", "Alpha Project", []*models.Task{task})
	project.Sections = []string{"Sprint-1"}

	m.loadedProjects = []*models.Project{project}
	m.crossProjectRebuildRows()

	plain := ansi.Strip(m.renderOutlinerTreeContent(width))
	assertNotContains(t, plain, "Sprint-1", "aggregate custom tab")
}

func TestCrossProjectOutlinerRenderIgnoresAggregateCustomTabColors(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const width = 80
	cfg := config.NewConfig()
	m := NewOutlinerModel(nil, cfg, nil)
	m.SetSize(width, 12)

	task := renderFixtureTask(t, "sprint-task", 11, models.TaskTypeTask, "Sprint task", models.StatusTodo, 3, "")
	task.Section = "Sprint-1"
	project := renderFixtureProject(t, "proj-alpha", "alpha", "Alpha Project", []*models.Task{task})
	project.Sections = []string{models.SectionNext, "Sprint-1"}
	project.SectionColors = map[string]string{"Sprint-1": "green"}

	m.loadedProjects = []*models.Project{project}
	m.crossProjectRebuildRows()

	rendered := m.renderOutlinerTreeContent(width)
	assertNotContains(t, ansi.Strip(rendered), "Sprint-1", "colored aggregate custom tab bar")
}

func TestCrossProjectOutlinerRenderIgnoresAggregateCustomTabsInEmptyState(t *testing.T) {
	const width = 80
	cfg := config.NewConfig()
	m := NewOutlinerModel(nil, cfg, nil)
	m.SetSize(width, 12)

	project := renderFixtureProject(t, "proj-alpha", "alpha", "Alpha Project", nil)
	project.Sections = []string{"Sprint-1"}
	m.loadedProjects = []*models.Project{project}

	plain := ansi.Strip(m.renderOutlinerTreeContent(width))
	assertNotContains(t, plain, "Sprint-1", "empty-state aggregate custom tab")
	assertContains(t, plain, "No tasks found.", "empty-state message")
}

func crossProjectRenderFixture(t *testing.T, width, height int) (*OutlinerModel, *models.Task) {
	t.Helper()

	cfg := config.NewConfig()
	m := NewOutlinerModel(nil, cfg, nil)
	m.SetSize(width, height)
	m.columnConfig = NewColumnConfig(map[string]bool{
		ColDependencies: false,
		ColDueDate:      false,
		ColScore:        false,
		ColSnooze:       false,
	}, nil, DefaultOutlinerColumns, PriorityModeAll)

	parent := renderFixtureTask(t, "alpha-parent", 41, models.TaskTypeStory, "Alpha parent with expanded children", models.StatusTodo, 3, models.SectionNext)
	selectedChild := renderFixtureTask(t, "alpha-child", 42, models.TaskTypeTask, "Selected child carries rendered metadata clearly across the row", models.StatusInProgress, 1, models.SectionNext)
	parentID := parent.ID
	selectedChild.ParentID = &parentID
	selectedChild.Tags = []string{"ops", "today"}
	selectedChild.Description = "Has detail text so the description indicator is rendered."
	selectedChild.Checklist = []models.ChecklistItem{
		{Text: "first", Done: true},
		{Text: "second", Done: false},
	}

	betaTask := renderFixtureTask(t, "beta-inbox", 7, models.TaskTypeBug, "Beta inbox triage item", models.StatusTodo, 2, models.SectionInbox)
	betaTask.Tags = []string{"triage"}

	alphaProject := renderFixtureProject(t, "proj-alpha", "alpha", "Alpha Project", []*models.Task{parent, selectedChild})
	betaProject := renderFixtureProject(t, "proj-beta", "beta", "Beta Project", []*models.Task{betaTask})

	m.loadedProjects = []*models.Project{alphaProject, betaProject}
	m.crossProjectRebuildRows()
	m.cursor = findOutlinerRowIndex(t, m.rows, selectedChild.ID)

	return m, selectedChild
}

func renderFixtureProject(t *testing.T, id, slug, name string, tasks []*models.Task) *models.Project {
	t.Helper()

	project, err := models.NewProject(id, name, t.TempDir())
	if err != nil {
		t.Fatalf("NewProject(%q): %v", id, err)
	}
	project.Slug = slug
	project.Tasks = tasks
	project.BuildTaskTree()
	return project
}

func renderFixtureTask(t *testing.T, id string, shortID int, taskType models.TaskType, title string, status models.TaskStatus, priority int, section string) *models.Task {
	t.Helper()

	task, err := models.NewTask(id, shortID, taskType, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.Status = status
	task.Priority = priority
	task.Section = section
	return task
}

func renderedProjectBadge(projectName, slug string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(projectBadgeColor(projectName))).
		Render("[" + slug + "]")
}

func findOutlinerRowIndex(t *testing.T, rows []outlinerRow, taskID string) int {
	t.Helper()

	for i, row := range rows {
		if row.task != nil && row.task.ID == taskID {
			return i
		}
	}
	t.Fatalf("row for task %q not found", taskID)
	return -1
}

func findRenderedLineContaining(t *testing.T, rendered, needle string) string {
	t.Helper()

	for line := range strings.SplitSeq(rendered, "\n") {
		if strings.Contains(ansi.Strip(line), needle) {
			return line
		}
	}
	t.Fatalf("rendered line containing %q not found in:\n%s", needle, ansi.Strip(rendered))
	return ""
}

func hasScrollbarGlyph(line string) bool {
	return strings.Contains(line, "█") || strings.Contains(line, "░")
}

func assertContains(t *testing.T, got, want, label string) {
	t.Helper()

	if !strings.Contains(got, want) {
		t.Fatalf("%s: expected %q to contain %q", label, got, want)
	}
}

func assertNotContains(t *testing.T, got, want, label string) {
	t.Helper()

	if strings.Contains(got, want) {
		t.Fatalf("%s: expected %q not to contain %q", label, got, want)
	}
}
