package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerUsesFullWidthAndPeekFollowsCursorAtEveryWidth(t *testing.T) {
	first, _ := models.NewGenericTask("first", 1, "First task")
	first.Description = "First description"
	first.Tags = []string{"backend", "security"}
	second, _ := models.NewGenericTask("second", 2, "Second task")
	second.Description = "Second description"
	second.Tags = []string{"frontend"}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{first, second})
	outliner := NewOutlinerModel(project, nil, newTestConfig())

	for _, width := range []int{60, 99, 100, 160} {
		outliner.SetSize(width, 22)
		outliner.rebuildRows()
		outliner.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: first.ID})
		outliner.peekMode = false
		closed := ansi.Strip(outliner.View().Content)
		if strings.Contains(closed, "Description: First description") || strings.Contains(closed, "Inspector") {
			t.Fatalf("width %d rendered detail while Peek was off:\n%s", width, closed)
		}

		outliner.togglePeek()
		firstView := ansi.Strip(outliner.View().Content)
		if !strings.Contains(firstView, "Description: First description") ||
			!strings.Contains(firstView, "Tags: #backend #security") {
			t.Fatalf("width %d Peek missing first task:\n%s", width, firstView)
		}
		outliner.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: second.ID})
		secondView := ansi.Strip(outliner.View().Content)
		if !strings.Contains(secondView, "Description: Second description") ||
			!strings.Contains(secondView, "Tags: #frontend") ||
			strings.Contains(secondView, "Tags: #backend #security") ||
			strings.Contains(secondView, "Description: First description") {
			t.Fatalf("width %d Peek did not follow cursor:\n%s", width, secondView)
		}
		for lineNumber, line := range strings.Split(outliner.View().Content, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d overflowed to %d", width, lineNumber, got)
			}
		}
	}
}

func TestPeekIndentDoesNotRenderTreeGlyphs(t *testing.T) {
	task := &models.Task{
		ID:          "nested",
		Title:       "Nested task",
		Description: "Additional detail",
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.peekMode = true
	outliner.cursor = 0

	row := outlinerRow{
		task:   task,
		prefix: "│   ├── ",
	}
	rendered := ansi.Strip(outliner.renderOutlinerTaskRow(row, true, 80, 80, 0, nil))
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatalf("rendered row has no Peek content:\n%s", rendered)
	}
	for _, peekLine := range lines[1:] {
		if strings.ContainsAny(peekLine, "│├└─") {
			t.Fatalf("Peek line contains tree glyphs: %q", peekLine)
		}
	}
	if !strings.HasPrefix(lines[1], strings.Repeat(" ", lipgloss.Width(row.prefix)+4)) {
		t.Fatalf("Peek line lost nesting alignment: %q", lines[1])
	}
}

// TestPeekContinuationCompletesTheRowTitleAtEveryWidth is the reconciliation
// test: the row's visible title minus its ellipsis, plus the Peek continuation,
// must equal the whole title. A cut the two paths disagree about shows up here
// as a duplicated or missing character.
func TestPeekContinuationCompletesTheRowTitleAtEveryWidth(t *testing.T) {
	title := "Investigate intermittent timeout when syncing very large project archives " +
		"over slow connections and report which retry policy the transport eventually settled on"
	task := &models.Task{ID: "task", Title: title, WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.peekMode = true
	outliner.cursor = 0

	for _, width := range []int{60, 100} {
		row := outlinerRow{task: task, ProjectID: project.ID}
		colLayout := worstCaseColumnLayout(width)
		overflow := outliner.taskRowTitleOverflow(row, width, colLayout)
		if overflow == "" || !strings.HasSuffix(title, overflow) {
			t.Fatalf("width %d overflow %q is not the title's own tail", width, overflow)
		}
		head := strings.TrimSuffix(title, overflow)

		lines := strings.Split(ansi.Strip(outliner.renderOutlinerTaskRow(row, true, width, width, 0, colLayout)), "\n")
		if len(lines) < 2 {
			t.Fatalf("width %d rendered no Peek continuation:\n%s", width, strings.Join(lines, "\n"))
		}
		if !strings.Contains(lines[0], head+"…") {
			t.Fatalf("width %d row did not render exactly the head %q:\n%s", width, head, lines[0])
		}
		// The wrap drops the space it breaks on, so the continuation is compared
		// word by word; the cut itself is already exact through head+overflow.
		continuation := strings.Join(strings.Fields(strings.Join(lines[1:], " ")), " ")
		if want := strings.Join(strings.Fields(overflow), " "); continuation != want {
			t.Fatalf("width %d continuation = %q, want %q", width, continuation, want)
		}
		if strings.Contains(strings.Join(lines[1:], "\n"), strings.TrimSpace(head)) {
			t.Fatalf("width %d Peek restated the head:\n%s", width, strings.Join(lines[1:], "\n"))
		}
		for lineNumber, line := range lines {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d overflowed to %d", width, lineNumber, got)
			}
		}
	}
}

// TestPeekRendersNoContinuationForAVisibleTitle keeps the block conditional: a
// title the row showed in full is never restated beneath it.
func TestPeekRendersNoContinuationForAVisibleTitle(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Short title", WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.peekMode = true
	outliner.cursor = 0

	row := outlinerRow{task: task, ProjectID: project.ID}
	lines := strings.Split(ansi.Strip(outliner.renderOutlinerTaskRow(row, true, 120, 120, 0, nil)), "\n")
	for _, line := range lines[1:] {
		if strings.Contains(line, "title") {
			t.Fatalf("Peek restated a fully visible title: %q", line)
		}
	}
}

func TestPeekOperationTogglesAndEscDismissesFirst(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, ViewCrossProjectOutliner)
	outliner := model.crossOutliner
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	for index, row := range outliner.rows {
		if row.isTaskRow() {
			outliner.cursor = index
			break
		}
	}

	invokePeek := func() {
		t.Helper()
		result := model.operationExecutor.Execute(OperationExecutionRequest{
			ID: operationCorePeek, Context: model.operationCommandContext(),
		})
		if result.Err != nil {
			t.Fatalf("toggle Peek: %v", result.Err)
		}
	}

	invokePeek()
	if !outliner.peekMode {
		t.Fatal("Shift+Enter did not enable Peek")
	}
	invokePeek()
	if outliner.peekMode {
		t.Fatal("Shift+Enter did not disable Peek")
	}
	invokePeek()
	outliner.crossFocusStack = append(outliner.crossFocusStack, CrossProjectFocusEntry{TaskID: "task"})
	_, _ = model.Update(syntheticSpecialKey(tea.KeyEsc))
	if outliner.peekMode {
		t.Fatal("Esc did not disable Peek")
	}
	if len(outliner.crossFocusStack) != 1 {
		t.Fatal("Esc unwound navigation instead of dismissing Peek first")
	}
}

func TestEnteringMultiSelectDisablesPeek(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Task", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Tasks: []*models.Task{task}}
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.rows = []outlinerRow{{task: task}}
	outliner.peekMode = true
	outliner.toggleCursorSelection()
	if outliner.peekMode {
		t.Fatal("entering multi-select left Peek enabled")
	}
	if !outliner.multiSelectMode {
		t.Fatal("multi-select did not activate")
	}
}

func TestPeekModeSurvivesModeAndScopeChanges(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "Inbox task")
	project.Tasks[0].ApplyWorkflowState(models.StateInbox)
	setTestAppProjects(app, project)
	outliner := app.crossOutliner
	outliner.loadedProjects = []*models.Project{project}
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	outliner.peekMode = true

	_, _ = outliner.selectOutlinerMode(ModeInbox)
	if !outliner.peekMode {
		t.Fatal("mode change disabled Peek")
	}
	outliner.pushProjectScope(project.ID)
	if !outliner.peekMode {
		t.Fatal("scope change disabled Peek")
	}
}

func TestOutlinerRowsPromoteOperationalSignalsAndHideShortID(t *testing.T) {
	task := &models.Task{
		ID: "task", ShortID: 73, Title: "Operational task",
		WorkflowState: models.StateBacklog,
		Checklist:     []models.ChecklistItem{{Text: "done", Done: true}, {Text: "open"}},
	}
	due := time.Now().AddDate(0, 0, 5)
	deferUntil := time.Now().AddDate(0, 0, -2)
	task.DueDate = &due
	task.DeferredUntil = &deferUntil
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.SetSize(100, 22)
	outliner.rebuildRows()
	outliner.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: task.ID})

	plain := ansi.Strip(outliner.View().Content)
	for _, want := range []string{"Operational task", due.Format("Jan 2"), "woke", "1/2"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("row missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "#73") {
		t.Fatalf("row leaked short ID:\n%s", plain)
	}
}

func TestNarrowOperationalRowProtectsUrgencyBeforeOptionalSignals(t *testing.T) {
	task := &models.Task{
		ID: "task", ShortID: 9, Title: "Readable title",
		WorkflowState: models.StateBacklog,
		Priority:      1,
		Checklist:     []models.ChecklistItem{{Text: "one"}, {Text: "two"}},
	}
	due := time.Now()
	deferUntil := time.Now().AddDate(0, 0, -1)
	task.DueDate = &due
	task.DeferredUntil = &deferUntil
	project := renderFixtureProject(
		t,
		"project",
		"a-very-long-project-slug",
		"Project",
		[]*models.Task{task},
	)
	outliner := NewOutlinerModel(nil, nil, newTestConfig())
	outliner.loadedProjects = []*models.Project{project}
	row := outlinerRow{
		task: task, ProjectID: project.ID,
		prefix: "│   │   └── ",
	}

	const width = 60
	plain := ansi.Strip(outliner.renderOutlinerTaskRow(row, true, width, width, 0, nil))
	for _, want := range []string{"Readable", "Today", "woke"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("narrow urgency row missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "#9") {
		t.Fatalf("narrow urgency row leaked short ID:\n%s", plain)
	}
}
