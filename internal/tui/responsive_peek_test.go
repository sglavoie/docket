package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestOutlinerUsesFullWidthAndPeekFollowsCursorAtEveryWidth(t *testing.T) {
	first, _ := models.NewGenericTask("first", 1, "First task")
	first.Description = "First description"
	first.Tags = []string{"backend", "security"}
	second, _ := models.NewGenericTask("second", 2, "Second task")
	second.Description = "Second description"
	second.Tags = []string{"frontend"}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{first, second})
	outlinerView := outliner.NewOutlinerModel(project, nil, newTestConfig())

	for _, width := range []int{60, 99, 100, 160} {
		outlinerView.SetSize(width, 22)
		outlinerView.RebuildRows()
		outlinerView.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: first.ID})
		outlinerView.PeekMode = false
		closed := ansi.Strip(outlinerView.View().Content)
		if strings.Contains(closed, "Description: First description") || strings.Contains(closed, "Inspector") {
			t.Fatalf("width %d rendered detail while Peek was off:\n%s", width, closed)
		}

		outlinerView.TogglePeek()
		outlinerView.PeekExpanded = true
		firstView := ansi.Strip(outlinerView.View().Content)
		if !strings.Contains(firstView, "Description: First description") ||
			!strings.Contains(firstView, "Tags: #backend #security") {
			t.Fatalf("width %d Peek missing first task:\n%s", width, firstView)
		}
		outlinerView.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: second.ID})
		secondView := ansi.Strip(outlinerView.View().Content)
		if !strings.Contains(secondView, "Description: Second description") ||
			!strings.Contains(secondView, "Tags: #frontend") ||
			strings.Contains(secondView, "Tags: #backend #security") ||
			strings.Contains(secondView, "Description: First description") {
			t.Fatalf("width %d Peek did not follow cursor:\n%s", width, secondView)
		}
		for lineNumber, line := range strings.Split(outlinerView.View().Content, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d overflowed to %d", width, lineNumber, got)
			}
		}
	}
}

// TestPeekDocumentIndentIgnoresTheRowsNesting is the dock's half of the old
// alignment claim. The document is drawn in a block of its own below the list
// rather than beneath the row it describes, so it owns one indent: a deeply
// nested row and a root row produce the same left edge, and neither carries the
// tree glyphs that only mean something in the list.
func TestPeekDocumentIndentIgnoresTheRowsNesting(t *testing.T) {
	task := &models.Task{
		ID:          "nested",
		Title:       "Nested task",
		Description: "Additional detail",
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outlinerView := outliner.NewOutlinerModel(project, nil, newTestConfig())
	outlinerView.SetSize(80, 24)
	outlinerView.PeekMode = true
	outlinerView.Cursor = 0

	nested := outlinerView.PeekDocumentBodyForRow(outliner.OutlinerRow{Task: task, Prefix: "│   ├── "})
	root := outlinerView.PeekDocumentBodyForRow(outliner.OutlinerRow{Task: task})
	if len(nested) == 0 {
		t.Fatal("the nested row produced no Peek document")
	}
	for index, line := range nested {
		plain := ansi.Strip(line)
		if strings.ContainsAny(plain, "│├└─") {
			t.Fatalf("Peek line contains tree glyphs: %q", plain)
		}
		if plain != ansi.Strip(root[index]) {
			t.Fatalf("line %d differs by nesting:\n%q\n%q", index, plain, ansi.Strip(root[index]))
		}
	}
	if got := ansi.Strip(nested[0]); !strings.HasPrefix(got, "  ") || strings.HasPrefix(got, "   ") {
		t.Fatalf("Peek line does not carry the dock's own indent: %q", got)
	}
}

// TestPeekNamesTheWholeTitleAtEveryWidth replaced the continuation
// reconciliation. Nothing is reassembled from a head and a tail any more: the
// row clips its title and Peek leads with the whole of it, wrapped, so a reader
// recovers the title from one place rather than from two measurements that have
// to agree.
func TestPeekNamesTheWholeTitleAtEveryWidth(t *testing.T) {
	title := "Investigate intermittent timeout when syncing very large project archives " +
		"over slow connections and report which retry policy the transport eventually settled on"
	task := &models.Task{ID: "task", Title: title, WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outlinerView := outliner.NewOutlinerModel(project, nil, newTestConfig())
	outlinerView.PeekMode = true
	outlinerView.Cursor = 0

	for _, width := range []int{60, 100} {
		outlinerView.SetSize(width, 24)
		row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}

		rowLine := ansi.Strip(outlinerView.RenderOutlinerTaskRow(row, true, width, 0, nil))
		if !strings.HasSuffix(strings.TrimRight(rowLine, " "), "…") {
			t.Fatalf("width %d row did not clip the title:\n%s", width, rowLine)
		}
		body := outlinerView.PeekDocumentBodyForRow(row)
		if len(body) == 0 {
			t.Fatalf("width %d rendered no Peek body", width)
		}
		// The wrap drops the space it breaks on, so the title is compared word by
		// word rather than byte for byte.
		peek := strings.Join(strings.Fields(ansi.Strip(strings.Join(body, " "))), " ")
		if want := strings.Join(strings.Fields(title), " "); !strings.HasPrefix(peek, want) {
			t.Fatalf("width %d Peek did not lead with the whole title:\n%s", width, peek)
		}
		for lineNumber, line := range append([]string{rowLine}, body...) {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d overflowed to %d", width, lineNumber, got)
			}
		}
	}
}

// TestPeekNamesATitleTheRowAlreadyShowed keeps the title unconditional. Peek is
// the complete document, not the row's remainder, so a title the row showed in
// full is still the document's first line.
func TestPeekNamesATitleTheRowAlreadyShowed(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Short title", WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outlinerView := outliner.NewOutlinerModel(project, nil, newTestConfig())
	outlinerView.PeekMode = true
	outlinerView.Cursor = 0

	outlinerView.SetSize(120, 24)
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}
	body := outlinerView.PeekDocumentBodyForRow(row)
	if len(body) == 0 {
		t.Fatal("the row produced no Peek document")
	}
	if got := strings.TrimSpace(ansi.Strip(body[0])); got != "Short title" {
		t.Fatalf("Peek did not lead with the title: %q", got)
	}
}

func TestPeekOperationTogglesAndEscDismissesFirst(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	outlinerView := model.crossOutliner
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	for index, row := range outlinerView.Rows {
		if row.IsTaskRow() {
			outlinerView.Cursor = index
			break
		}
	}

	invokePeek := func() {
		t.Helper()
		result := model.operations.Executor.Execute(operation.OperationExecutionRequest{
			ID: operation.OperationCorePeek, Context: model.operationCommandContext(),
		})
		if result.Err != nil {
			t.Fatalf("toggle Peek: %v", result.Err)
		}
	}

	invokePeek()
	if !outlinerView.PeekMode {
		t.Fatal("b did not enable Peek")
	}
	invokePeek()
	if outlinerView.PeekMode {
		t.Fatal("b did not disable Peek")
	}
	invokePeek()
	outlinerView.Focus.CrossStack = append(outlinerView.Focus.CrossStack, outliner.CrossProjectFocusEntry{TaskID: "task"})
	_, _ = model.Update(syntheticSpecialKey(tea.KeyEsc))
	if outlinerView.PeekMode {
		t.Fatal("Esc did not disable Peek")
	}
	if len(outlinerView.Focus.CrossStack) != 1 {
		t.Fatal("Esc unwound navigation instead of dismissing Peek first")
	}
}

func TestEnteringMultiSelectDisablesPeek(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Task", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Tasks: []*models.Task{task}}
	outlinerView := outliner.NewOutlinerModel(project, nil, newTestConfig())
	outlinerView.Rows = []outliner.OutlinerRow{{Task: task}}
	outlinerView.PeekMode = true
	outlinerView.ToggleCursorSelection()
	if outlinerView.PeekMode {
		t.Fatal("entering multi-select left Peek enabled")
	}
	if !outlinerView.MultiSelectMode {
		t.Fatal("multi-select did not activate")
	}
}

func TestPeekModeSurvivesModeAndScopeChanges(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "Inbox task")
	project.Tasks[0].ApplyWorkflowState(models.StateInbox)
	setTestAppProjects(app, project)
	outlinerView := app.crossOutliner
	outlinerView.Population.Projects = []*models.Project{project}
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	outlinerView.PeekMode = true

	_, _ = outlinerView.SelectOutlinerMode(chrome.ModeInbox)
	if !outlinerView.PeekMode {
		t.Fatal("mode change disabled Peek")
	}
	outlinerView.PushProjectScope(project.ID)
	if !outlinerView.PeekMode {
		t.Fatal("scope change disabled Peek")
	}
}

// TestOutlinerRowsKeepTheDeadlineAndSendSupportingSignalsToPeek is the row
// budget stated as a pair. The deadline is required interaction context, so it
// stays on the row beside the title; the checklist tally and the wake a
// deferral left behind are supporting detail, so the same task names them in
// Peek and nowhere else. The short ID was never on either surface.
func TestOutlinerRowsKeepTheDeadlineAndSendSupportingSignalsToPeek(t *testing.T) {
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
	outlinerView := outliner.NewOutlinerModel(project, nil, newTestConfig())
	outlinerView.SetSize(100, 22)
	outlinerView.RebuildRows()
	outlinerView.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID})

	plain := ansi.Strip(outlinerView.View().Content)
	for _, want := range []string{"Operational task", due.Format("Jan 2")} {
		if !strings.Contains(plain, want) {
			t.Fatalf("row missing %q:\n%s", want, plain)
		}
	}
	for _, unwanted := range []string{"woke", "1/2", "#73"} {
		if strings.Contains(strings.ToLower(plain), unwanted) {
			t.Fatalf("row spent a column on %q:\n%s", unwanted, plain)
		}
	}

	outlinerView.PeekMode = true
	peek := ansi.Strip(strings.Join(
		outlinerView.PeekDocumentBodyForRow(outlinerView.Rows[outlinerView.Cursor]), "\n"))
	for _, want := range []string{"Woke: ", "Checklist: 1/2 done"} {
		if !strings.Contains(peek, want) {
			t.Fatalf("Peek did not carry %q the row stopped naming:\n%s", want, peek)
		}
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
	// Four levels of connector mean a parent, and the row carries one, so the
	// compact project label stands down the way it does for any row seated under
	// its own parent: the connectors have already said which project this is.
	parent := &models.Task{
		ID: "parent", ShortID: 8, Title: "Parent",
		WorkflowState: models.StateBacklog,
	}
	task.ParentID = &parent.ID
	project := renderFixtureProject(
		t,
		"project",
		"a-very-long-project-slug",
		"Project",
		[]*models.Task{parent, task},
	)
	outlinerView := outliner.NewOutlinerModel(nil, nil, newTestConfig())
	outlinerView.Population.Projects = []*models.Project{project}
	row := outliner.OutlinerRow{
		Task: task, ProjectID: project.ID,
		Prefix: "│   │   └── ",
	}

	const width = 60
	plain := ansi.Strip(outlinerView.RenderOutlinerTaskRow(row, true, width, 0, nil))
	for _, want := range []string{"Readable title", "Today"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("narrow urgency row missing %q:\n%s", want, plain)
		}
	}
	// Four levels of tree prefix and a deadline used to leave this title
	// competing with a wake marker and a tally for the same columns; on the
	// reclaimed budget it arrives whole instead.
	if strings.Contains(plain, "…") {
		t.Fatalf("narrow urgency row clipped a title the budget now fits:\n%s", plain)
	}
	for _, unwanted := range []string{"woke", "0/2", "#9"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("narrow urgency row spent a column on %q:\n%s", unwanted, plain)
		}
	}
}
