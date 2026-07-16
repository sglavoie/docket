package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
	assertContains(t, plain, "‹ Completed", "previous perspective label")
	assertContains(t, plain, "@ All", "current perspective label")
	assertContains(t, plain, "Active ›", "next perspective label")
	assertContains(t, plain, IconExpanded, "expanded section indicator")
	assertContains(t, plain, "@ Inbox", "Inbox section label")
	assertContains(t, plain, "(1)", "Inbox section count")
	assertContains(t, plain, "@ Next", "Next section label")
	assertContains(t, plain, "@ Active", "Active section label")
	assertContains(t, rendered, MutedStyle.Render("‹ Completed"), "styled previous perspective")
	assertContains(t, rendered, GetViewModeStyleForSection("All").Render("@ All"), "styled current perspective")
	assertContains(t, rendered, MutedStyle.Render("Active ›"), "styled next perspective")
	assertContains(t, rendered, GetViewModeStyleForSection(models.SectionInbox).Bold(true).Render("@ Inbox"), "styled Inbox section")
	assertContains(t, rendered, GetViewModeStyleForSection(models.SectionNext).Bold(true).Render("@ Next"), "styled Next section")
	assertContains(t, rendered, MutedStyle.Render("(1)"), "styled Inbox count")
	assertContains(t, rendered, MutedStyle.Render("(1)"), "styled per-section counts")
	assertContains(t, rendered, renderedProjectHeaderBadge("Alpha Project", "alpha"), "styled Alpha project sub-header badge")
	assertContains(t, rendered, renderedProjectHeaderBadge("Beta Project", "beta"), "styled Beta project sub-header badge")

	selectedLine := findRenderedLineContaining(t, rendered, "#42")
	selectedPlain := ansi.Strip(selectedLine)
	assertNotContains(t, selectedPlain, "[alpha]", "per-task project badge suppressed in cross-project All perspective")
	assertContains(t, selectedPlain, "Selected child carries rendered metadata clearly", "selected title")
	assertContains(t, selectedPlain, "#42", "selected short ID metadata")
	assertNotContains(t, selectedPlain, "#ops #today", "tag metadata moves out of dense row into the metadata strip")
	if selectedLine == selectedPlain {
		t.Fatalf("selected row has no ANSI styling applied: %q", selectedLine)
	}

	stripPlain := ansi.Strip(findRenderedLineContaining(t, rendered, "#ops #today"))
	assertContains(t, stripPlain, "#ops #today", "selected metadata strip carries the moved tag metadata")

	for i, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("rendered line %d exceeds width %d: width=%d stripped=%q", i, width, got, ansi.Strip(line))
		}
	}
}

func TestCrossProjectOutlinerViewMultiSelectTitleFitsNarrowWidth(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	const width = 72
	m.SetSize(width, 20)
	m.sidebar.SetEnabled(false)

	projects := make([]*models.Project, 0, 34)
	for i := 0; i < 34; i++ {
		task := renderFixtureTask(t, fmt.Sprintf("task-%02d", i), i+1, models.TaskTypeTask,
			fmt.Sprintf("Cross-project task %02d with enough title text to render rows", i),
			models.StatusTodo, 3, models.SectionNext)
		project := renderFixtureProject(t, fmt.Sprintf("proj-%02d", i), fmt.Sprintf("project-%02d", i),
			fmt.Sprintf("Project %02d", i), []*models.Task{task})
		projects = append(projects, project)
	}
	m.loadedProjects = projects
	m.crossProjectRebuildRows()
	m.multiSelectMode = true

	rendered := m.View().Content
	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, "Cross-project Outliner (34 projects)") || !strings.Contains(plain, "[MULTI-SELECT: 0 selected]") {
		t.Fatalf("test setup did not render aggregate multi-select title pressure:\n%s", plain)
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

	m, selectedTask := crossProjectRenderFixture(t, 112, 18)
	parentIdx := findOutlinerRowIndex(t, m.rows, "alpha-parent")
	childIdx := findOutlinerRowIndex(t, m.rows, selectedTask.ID)
	if got := m.rows[childIdx].task.ID; got != selectedTask.ID {
		t.Fatalf("promoted row task ID = %q, want child ID %q", got, selectedTask.ID)
	}

	const wideRowWidth = 112
	parentLine := m.renderOutlinerTaskRow(m.rows[parentIdx], false, wideRowWidth, wideRowWidth, parentIdx, nil)
	parentPlain := ansi.Strip(parentLine)
	assertContains(t, parentPlain, "└────", "section-local parent tree prefix")
	assertNotContains(t, parentPlain, IconExpanded, "cross-section child does not create a fold indicator")
	assertNotContains(t, parentPlain, "[alpha]", "per-task project badge suppressed in cross-project All perspective")

	wideChildLine := m.renderOutlinerTaskRow(m.rows[childIdx], true, wideRowWidth, wideRowWidth, childIdx, nil)
	wideChildPlain := ansi.Strip(wideChildLine)
	assertContains(t, wideChildPlain, "└────", "promoted section-root prefix")
	assertContains(t, wideChildPlain, selectedTask.Title, "promoted child primary title")
	assertContains(t, wideChildPlain, "Alpha parent", "promoted child ancestor suffix")
	if childPos, parentPos := strings.Index(wideChildPlain, selectedTask.Title), strings.Index(wideChildPlain, "Alpha parent"); childPos < 0 || parentPos <= childPos {
		t.Fatalf("promoted row labels are not child-first: %q", wideChildPlain)
	}
	assertNotContains(t, wideChildPlain, "[alpha]", "per-task project badge suppressed in cross-project All perspective")
	assertContains(t, wideChildPlain, "#42", "child short ID metadata")
	assertNotContains(t, wideChildPlain, "#ops #today", "dense rows no longer carry tag metadata inline")
	if got := lipgloss.Width(wideChildLine); got != wideRowWidth {
		t.Fatalf("wide selected child row width = %d, want %d, stripped=%q", got, wideRowWidth, wideChildPlain)
	}

	const narrowRowWidth = 72
	narrowChildLine := m.renderOutlinerTaskRow(m.rows[childIdx], true, narrowRowWidth, narrowRowWidth, childIdx, nil)
	narrowChildPlain := ansi.Strip(narrowChildLine)
	assertContains(t, narrowChildPlain, "Selected chi", "truncated child title prefix")
	assertNotContains(t, narrowChildPlain, "Alpha parent", "ancestor context omitted when the child title is truncated")
	if strings.Contains(narrowChildPlain, selectedTask.Title) {
		t.Fatalf("expected child title to be truncated at width %d, got full title in %q", narrowRowWidth, narrowChildPlain)
	}
	if got := lipgloss.Width(narrowChildLine); got != narrowRowWidth {
		t.Fatalf("narrow selected child row width = %d, want %d, stripped=%q", got, narrowRowWidth, narrowChildPlain)
	}
	if narrowChildLine == narrowChildPlain {
		t.Fatalf("selected child row has no ANSI styling applied: %q", narrowChildLine)
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
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(width, 12)

	task := renderFixtureTask(t, "sprint-task", 11, models.TaskTypeTask, "Sprint task", models.StatusTodo, 3, "")
	// Section "Sprint-1" is a retired custom section; grouping now lives in tags.
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
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(width, 12)

	task := renderFixtureTask(t, "sprint-task", 11, models.TaskTypeTask, "Sprint task", models.StatusTodo, 3, "")
	// Section "Sprint-1" is a retired custom section; grouping now lives in tags.
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
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(width, 12)

	project := renderFixtureProject(t, "proj-alpha", "alpha", "Alpha Project", nil)
	project.Sections = []string{"Sprint-1"}
	m.loadedProjects = []*models.Project{project}

	plain := ansi.Strip(m.renderOutlinerTreeContent(width))
	assertNotContains(t, plain, "Sprint-1", "empty-state aggregate custom tab")
	assertContains(t, plain, "No tasks found.", "empty-state message")
}

// TestOutlinerLongStageDoesNotShiftSiblingRows guards the plan's primary
// regression for the outliner: the row format has no stage column (stage was
// retired from models.Task entirely), so all task rows of the same width
// category must render to the same width.
func TestOutlinerLongStageDoesNotShiftSiblingRows(t *testing.T) {
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	const width = 100
	m.SetSize(width, 18)

	tasks := make([]*models.Task, 0, 8)
	for i := 0; i < 8; i++ {
		task := renderFixtureTask(t,
			fmt.Sprintf("sibling-%02d", i),
			i+1,
			models.TaskTypeTask,
			fmt.Sprintf("Sibling task %02d", i),
			models.StatusTodo,
			3,
			models.SectionNext)
		tasks = append(tasks, task)
	}

	project := renderFixtureProject(t, "proj-long-stage", "lng", "Long Stage", tasks)
	m.project = project
	m.rebuildRows()

	rowWidths := make(map[int]int)
	for i, row := range m.rows {
		if row.task == nil {
			continue
		}
		line := m.renderOutlinerTaskRow(row, false, width, width, i, nil)
		rowWidths[lipgloss.Width(ansi.Strip(line))]++
	}
	if len(rowWidths) == 0 {
		t.Fatalf("expected at least one task row, got none")
	}
	if len(rowWidths) != 1 {
		t.Fatalf("expected uniform outliner row width across long-stage and empty-stage siblings, got %v", rowWidths)
	}
}

func crossProjectRenderFixture(t *testing.T, width, height int) (*OutlinerModel, *models.Task) {
	t.Helper()

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
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
	task.WorkflowState = legacyStateForStatusSection(status, section)
	task.Priority = priority
	return task
}

func renderedProjectHeaderBadge(projectName, slug string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(projectBadgeColor(projectName))).
		Bold(true).
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

// TestCrossProjectNonAllPerspectiveEmitsProjectSubHeaders verifies that
// non-All cross-project perspectives emit one synthetic project sub-header
// per visible project group (mirroring the All-perspective shape) and no
// GTD section headers. Inline `[slug]` badge suppression on task rows is a
// rendering concern covered separately.
func TestCrossProjectNonAllPerspectiveEmitsProjectSubHeaders(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const width = 100
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(width, 18)

	alphaTask := renderFixtureTask(t, "alpha-active", 1, models.TaskTypeTask, "Alpha active item", models.StatusInProgress, 2, "")
	betaTask := renderFixtureTask(t, "beta-active", 2, models.TaskTypeTask, "Beta active item", models.StatusInProgress, 2, "")
	alpha := renderFixtureProject(t, "proj-alpha", "alpha", "Alpha Project", []*models.Task{alphaTask})
	beta := renderFixtureProject(t, "proj-beta", "beta", "Beta Project", []*models.Task{betaTask})

	m.loadedProjects = []*models.Project{alpha, beta}
	m.activePerspective = 2 // Active
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	projectHeaderCount := 0
	for i, row := range m.rows {
		if row.IsSectionHeader {
			t.Fatalf("row[%d] is IsSectionHeader; non-All perspective must not emit GTD section headers", i)
		}
		if row.IsSeparator {
			t.Fatalf("row[%d] is IsSeparator; non-All perspective must use project headers instead of separators", i)
		}
		if row.IsProjectHeader {
			projectHeaderCount++
		}
	}
	if projectHeaderCount != 2 {
		t.Fatalf("expected 2 project sub-headers (one per visible project), got %d", projectHeaderCount)
	}

	rendered := m.renderOutlinerTreeContent(width)
	assertContains(t, rendered, renderedProjectHeaderBadge("Alpha Project", "alpha"), "non-All perspective renders bold project sub-header badge")
	assertContains(t, rendered, renderedProjectHeaderBadge("Beta Project", "beta"), "non-All perspective renders bold project sub-header badge")

	// The inline `[slug]` task-row badge must be suppressed in normal
	// cross-project perspectives: project identity now lives on the bold
	// project sub-header above each group, so repeating it on every task row
	// would be visual noise.
	alphaTaskLine := ansi.Strip(findRenderedLineContaining(t, rendered, "Alpha active item"))
	assertNotContains(t, alphaTaskLine, "[alpha]", "non-All perspective suppresses inline alpha task badge in favor of project sub-header")
	betaTaskLine := ansi.Strip(findRenderedLineContaining(t, rendered, "Beta active item"))
	assertNotContains(t, betaTaskLine, "[beta]", "non-All perspective suppresses inline beta task badge in favor of project sub-header")
}

// TestCrossProjectPriorityModeOmitsProjectSubHeaders verifies that the
// cross-project Priority mode never emits IsProjectHeader rows and keeps the
// per-task inline `[slug]` badge visible (Priority mode buckets tasks
// individually and has no per-project visual grouping).
func TestCrossProjectPriorityModeOmitsProjectSubHeaders(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const width = 100
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(width, 18)

	alphaTask := makePriorityTask("alpha-prio", "Alpha priority item", intPtr(0))
	alphaTask.Priority = 1
	betaTask := makePriorityTask("beta-prio", "Beta priority item", intPtr(0))
	betaTask.Priority = 1
	alpha := renderFixtureProject(t, "proj-alpha", "alpha", "Alpha Project", []*models.Task{alphaTask})
	beta := renderFixtureProject(t, "proj-beta", "beta", "Beta Project", []*models.Task{betaTask})

	m.loadedProjects = []*models.Project{alpha, beta}
	m.enterCrossPriorityMode()
	m.crossProjectRebuildRows()

	for i, row := range m.rows {
		if row.IsProjectHeader {
			t.Fatalf("row[%d] is IsProjectHeader; Priority mode must not emit project sub-headers", i)
		}
	}

	rendered := m.renderOutlinerTreeContent(width)
	plain := ansi.Strip(rendered)
	assertContains(t, plain, "[alpha]", "Priority mode preserves inline alpha project badge")
	assertContains(t, plain, "[beta]", "Priority mode preserves inline beta project badge")
	assertNotContains(t, rendered, renderedProjectHeaderBadge("Alpha Project", "alpha"), "Priority mode must not render bold project sub-header badge")
	assertNotContains(t, rendered, renderedProjectHeaderBadge("Beta Project", "beta"), "Priority mode must not render bold project sub-header badge")
}

// TestCrossProjectAllPerspectiveCollapsedSectionHidesProjectSubHeaders
// verifies that collapsing a shared GTD section header in the All perspective
// also hides every project sub-header beneath that section: the section
// header remains visible (and collapsed), but no IsProjectHeader rows are
// emitted for the collapsed section.
func TestCrossProjectAllPerspectiveCollapsedSectionHidesProjectSubHeaders(t *testing.T) {
	const width = 100
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(width, 18)

	alphaTask := renderFixtureTask(t, "alpha-next", 1, models.TaskTypeTask, "Alpha next item", models.StatusTodo, 2, models.SectionNext)
	betaTask := renderFixtureTask(t, "beta-next", 2, models.TaskTypeTask, "Beta next item", models.StatusTodo, 2, models.SectionNext)
	alpha := renderFixtureProject(t, "proj-alpha", "alpha", "Alpha Project", []*models.Task{alphaTask})
	beta := renderFixtureProject(t, "proj-beta", "beta", "Beta Project", []*models.Task{betaTask})

	m.loadedProjects = []*models.Project{alpha, beta}
	m.activePerspective = PerspectiveAll
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	projectHeadersWhenExpanded := 0
	for _, row := range m.rows {
		if row.IsProjectHeader {
			projectHeadersWhenExpanded++
		}
	}
	if projectHeadersWhenExpanded != 2 {
		t.Fatalf("expected 2 project sub-headers under expanded @ Next section, got %d", projectHeadersWhenExpanded)
	}

	m.collapsedSections["_xp:"+models.SectionNext] = true
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	sawNextHeader := false
	for i, row := range m.rows {
		if row.IsProjectHeader {
			t.Fatalf("row[%d] is IsProjectHeader; collapsed section must hide all project sub-headers", i)
		}
		if row.IsSectionHeader && row.SectionName == models.SectionNext {
			sawNextHeader = true
		}
	}
	if !sawNextHeader {
		t.Fatal("collapsed Next section header should still render even when collapsed")
	}
}
