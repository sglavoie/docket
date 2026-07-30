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

func TestTaskInspectorSummaryCoversCanonicalFieldsAndBoundsUnicode(t *testing.T) {
	project, err := models.NewProject("project", "Résumé 🚀", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	parent, err := models.NewGenericTask("parent", 1, "Parent with 日本語")
	if err != nil {
		t.Fatalf("NewGenericTask parent: %v", err)
	}
	task, err := models.NewGenericTask("task", 2, "Ship the Unicode-safe inspector ✨")
	if err != nil {
		t.Fatalf("NewGenericTask task: %v", err)
	}
	task.ParentID = &parent.ID
	task.Description = strings.Repeat("A long description with naïve café and 界 characters. ", 8)
	task.WorkflowState = models.StateWaiting
	task.Priority = 2
	task.Tags = []string{"backend", "i18n"}
	task.DependsOn = []string{parent.ID}
	task.Recurrence = &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly}
	task.Checklist = []models.ChecklistItem{{Text: "one", Done: true}, {Text: "two"}}
	now := time.Date(2026, time.July, 29, 12, 30, 0, 0, time.UTC)
	task.CreatedAt = now.Add(-time.Hour)
	task.UpdatedAt = now
	project.Tasks = []*models.Task{parent, task}
	project.BuildTaskTree()

	const width = 31
	rendered := renderTaskInspectorSummary(task, project, width, 40)
	plain := ansi.Strip(rendered)
	for _, want := range []string{
		"Task: #2", "Path: Résumé 🚀", "State: waiting",
		"Priority: P2 high", "Due: none", "Snooze: none", "Tags: backend, i18n",
		"Dependencies: #1", "Recurrence: every 2 weeks", "Checklist: 1/2 complete",
		"Lifecycle", "Created:", "Updated:", "Completed: none", "Description",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("summary missing %q:\n%s", want, plain)
		}
	}
	for lineNumber, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", lineNumber, got, width, ansi.Strip(line))
		}
	}
}

func TestTaskInspectorSummaryTruncatesLongContentToHeight(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		ShortID:     1,
		Title:       strings.Repeat("界", 80),
		Description: strings.Repeat("description ", 100),
	}
	rendered := renderTaskInspectorSummary(task, nil, 18, 8)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 8 {
		t.Fatalf("summary height = %d, want 8:\n%s", len(lines), ansi.Strip(rendered))
	}
	if !strings.Contains(ansi.Strip(lines[len(lines)-1]), "…") {
		t.Fatalf("truncated summary missing ellipsis:\n%s", ansi.Strip(rendered))
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > 18 {
			t.Fatalf("line width = %d, want <= 18: %q", got, ansi.Strip(line))
		}
	}
}

func TestOutlinerInspectorBreakpointAndCursorFollowing(t *testing.T) {
	first, _ := models.NewGenericTask("first", 1, "First task")
	second, _ := models.NewGenericTask("second", 2, "Second task")
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{first, second})
	outliner := NewOutlinerModel(project, nil, newTestConfig())

	outliner.SetSize(inspectorWideBreakpoint-1, 22)
	outliner.rebuildRows()
	outliner.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: first.ID})
	narrow := ansi.Strip(outliner.View().Content)
	if strings.Contains(narrow, "Task: #1 First task") {
		t.Fatalf("narrow Outliner unexpectedly rendered the persistent inspector:\n%s", narrow)
	}

	outliner.SetSize(inspectorWideBreakpoint, 22)
	outliner.rebuildRows()
	outliner.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: first.ID})
	wideFirst := ansi.Strip(outliner.View().Content)
	if !strings.Contains(wideFirst, "Task: #1 First task") {
		t.Fatalf("wide inspector did not follow first task:\n%s", wideFirst)
	}
	outliner.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: second.ID})
	wideSecond := ansi.Strip(outliner.View().Content)
	if !strings.Contains(wideSecond, "Task: #2 Second task") {
		t.Fatalf("wide inspector did not follow second task:\n%s", wideSecond)
	}
	for lineNumber, line := range strings.Split(outliner.View().Content, "\n") {
		if got := lipgloss.Width(line); got > inspectorWideBreakpoint {
			t.Fatalf("wide line %d width = %d, want <= %d", lineNumber, got, inspectorWideBreakpoint)
		}
	}
}

func TestInspectOperationOpensFrozenNarrowInspectorAndIsNoOpWhenWide(t *testing.T) {
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
	task, _ := outliner.getTaskRef()
	if task == nil {
		t.Fatal("test Outliner has no cursor task")
	}
	invokeInspector := func() {
		t.Helper()
		context := model.operationCommandContext()
		descriptor, ok := model.operationRegistry.Descriptor(operationCoreDetail)
		if !ok {
			t.Fatal("inspect operation is not registered")
		}
		targets, err := (OperationTargetResolver{}).Resolve(descriptor, context)
		if err != nil {
			t.Fatalf("resolve inspect target: %v", err)
		}
		result := model.executeCoreInputOperation(OperationInvocation{
			Descriptor: descriptor,
			Context:    context,
			Targets:    targets,
			Source:     OperationSourceDirectKey,
		})
		if result.Message != nil {
			_, _ = model.Update(result.Message)
		}
	}

	model.handleWindowSize(tea.WindowSizeMsg{Width: inspectorWideBreakpoint - 1, Height: 30})
	beforeCursor := outliner.cursor
	invokeInspector()
	if !model.peekActive || model.peekTask == nil || model.peekTask.ID != task.ID {
		t.Fatalf("narrow Enter did not open inspector for %q", task.ID)
	}
	_, _ = model.Update(syntheticKey('j'))
	if outliner.cursor != beforeCursor || model.peekTask == nil || model.peekTask.ID != task.ID {
		t.Fatal("narrow inspector did not freeze its target and covered cursor")
	}
	_, _ = model.Update(syntheticSpecialKey(tea.KeyEsc))
	if model.peekActive {
		t.Fatal("Esc did not close narrow inspector")
	}

	model.handleWindowSize(tea.WindowSizeMsg{Width: inspectorWideBreakpoint, Height: 30})
	invokeInspector()
	if model.peekActive {
		t.Fatal("wide Enter opened an inspector overlay")
	}
}
