package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func inlineEditorFixture(t *testing.T) (*OutlinerModel, *models.Project, *models.Task, *models.Task) {
	t.Helper()
	parent := &models.Task{ID: "parent", ShortID: 1, Title: "Parent", WorkflowState: models.StateBacklog}
	child := &models.Task{
		ID: "child", ShortID: 2, Title: "Child", ParentID: &parent.ID,
		WorkflowState: models.StateBacklog, Priority: models.PriorityHigh,
		Tags: []string{"alpha", "beta"}, Pinned: true,
	}
	due := time.Date(2030, time.January, 2, 0, 0, 0, 0, time.UTC)
	deferUntil := due.Add(-24 * time.Hour)
	child.DueDate, child.DeferredUntil = &due, &deferUntil
	project := &models.Project{ID: "project", Slug: "project", Name: "Project", Tasks: []*models.Task{parent, child}}
	project.BuildTaskTree()
	out := NewOutlinerModel(project, nil, nil)
	out.SetSize(120, 20)
	out.rebuildRows()
	for index, row := range out.rows {
		if row.task != nil && row.task.ID == child.ID && row.isTaskRow() {
			out.cursor = index
			break
		}
	}
	return out, project, parent, child
}

func TestInlineRowEditorStateMachineAcceptsAtCursorAndRetainsTokens(t *testing.T) {
	out, project, _, child := inlineEditorFixture(t)
	child.Tags = append(child.Tags, "urgent")
	q := out.quickEdit
	q.EnterWithContext(child, project, []*models.Project{project}, false)
	q.input.SetValue("Renamed #ur $today")
	q.input.SetCursor(len([]rune("Renamed #ur")))
	q.refreshDraft()
	if !q.ac.IsActive() || q.ac.Mode != acModeTag {
		t.Fatalf("tag autocomplete inactive: %+v", q.ac)
	}

	transition, _, _ := q.Handle(tea.KeyPressMsg{Code: tea.KeyEnter})
	if transition != quickEditContinue || !q.IsActive() {
		t.Fatalf("accept transition=%v active=%t, want continue and active", transition, q.IsActive())
	}
	if got := q.input.Value(); !strings.Contains(got, "#urgent") || !strings.Contains(got, "$today") {
		t.Fatalf("accepted input lost token or suffix: %q", got)
	}
	if q.ac.IsActive() {
		t.Fatal("autocomplete remained open after acceptance")
	}

	transition, result, _ := q.Handle(tea.KeyPressMsg{Code: tea.KeyEnter})
	if transition != quickEditCommit || result == nil {
		t.Fatalf("commit transition=%v result=%v", transition, result)
	}
	if result.title != "Renamed" || !result.destinationSet || string(result.destination) != "today" {
		t.Fatalf("commit result = %+v", result)
	}
}

func TestInlineRowEditorCompletesCanonicalTagRemoval(t *testing.T) {
	_, project, _, child := inlineEditorFixture(t)
	q := NewQuickEditComponent()
	q.EnterWithContext(child, project, []*models.Project{project}, false)
	q.input.SetValue("Child -#al")
	q.input.CursorEnd()
	q.refreshDraft()
	if !q.ac.IsActive() || !q.removeTag {
		t.Fatalf("remove-tag autocomplete inactive: %+v", q.ac)
	}
	q.Handle(tea.KeyPressMsg{Code: tea.KeyTab})
	if got := q.input.Value(); !strings.Contains(got, "-#alpha ") {
		t.Fatalf("remove-tag acceptance = %q", got)
	}
}

func TestInlineRowEditorAutocompleteUsesEditGrammarAndEffectiveProject(t *testing.T) {
	_, source, _, child := inlineEditorFixture(t)
	destinationParent := &models.Task{ID: "destination-parent", ShortID: 7, Title: "Destination Parent", WorkflowState: models.StateBacklog}
	destination := &models.Project{
		ID: "destination", Slug: "destination", Name: "Destination",
		Tasks: []*models.Task{destinationParent},
	}
	destination.BuildTaskTree()
	projects := []*models.Project{source, destination}
	q := NewQuickEditComponent()
	q.EnterWithContext(child, source, projects, false)

	q.input.SetValue("Child !")
	q.input.CursorEnd()
	q.refreshDraft()
	if !autocompleteContains(q.ac.Items, "0") {
		t.Fatalf("edit priority suggestions omit canonical clear value: %+v", q.ac.Items)
	}

	q.input.SetValue("Child %dest")
	q.input.CursorEnd()
	q.refreshDraft()
	if !autocompleteContains(q.ac.Items, destination.Slug) {
		t.Fatalf("project suggestions omit exact destination slug: %+v", q.ac.Items)
	}

	q.input.SetValue("Child %destination >")
	q.input.CursorEnd()
	q.refreshDraft()
	if !autocompleteContains(q.ac.Items, fmt.Sprint(destinationParent.ShortID)) {
		t.Fatalf("parent suggestions did not switch to effective project: %+v", q.ac.Items)
	}
}

func autocompleteContains(items []ACSuggestion, value string) bool {
	for _, item := range items {
		if item.Value == value {
			return true
		}
	}
	return false
}

func TestInlineRowEditorCancelPasteAndValidationTransitions(t *testing.T) {
	_, project, _, child := inlineEditorFixture(t)
	q := NewQuickEditComponent()
	q.EnterWithContext(child, project, []*models.Project{project}, false)
	q.input.CursorEnd()
	transition, _, _ := q.Handle(tea.PasteMsg{Content: "\nnew\r\ntext"})
	if transition != quickEditContinue || strings.ContainsAny(q.input.Value(), "\r\n") {
		t.Fatalf("paste transition=%v value=%q", transition, q.input.Value())
	}
	if !strings.Contains(q.input.Value(), " new text") {
		t.Fatalf("paste was not normalized to spaces: %q", q.input.Value())
	}

	q.input.SetValue("")
	q.input.CursorEnd()
	q.refreshDraft()
	transition, _, _ = q.Handle(tea.KeyPressMsg{Code: tea.KeyEnter})
	if transition != quickEditContinue || !q.IsActive() || q.GetPreviewError() == "" {
		t.Fatalf("invalid commit transition=%v active=%t error=%q", transition, q.IsActive(), q.GetPreviewError())
	}

	q.input.SetValue("Child #")
	q.input.CursorEnd()
	q.refreshDraft()
	if !q.ac.IsActive() {
		t.Fatal("expected open suggestions before immediate cancel")
	}
	transition, _, _ = q.Handle(tea.KeyPressMsg{Code: tea.KeyEsc})
	if transition != quickEditCancel || q.IsActive() {
		t.Fatalf("cancel transition=%v active=%t", transition, q.IsActive())
	}
}

func TestInlineRowEditorWideNarrowNestedAndErrorRendering(t *testing.T) {
	out, project, _, child := inlineEditorFixture(t)
	out.quickEdit.EnterWithContext(child, project, []*models.Project{project}, false)
	out.quickEdit.input.SetValue("Child revised $today #gamma")
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()
	row := out.rows[out.cursor]

	wide := ansi.Strip(out.quickEditMetadataLine(row, project, 120))
	for _, want := range []string{"EDIT TASK", "[project] › Parent", "today", "!1", "@Jan 2", "pin", "#gamma"} {
		if !strings.Contains(wide, want) {
			t.Errorf("wide metadata missing %q: %q", want, wide)
		}
	}
	narrow := ansi.Strip(out.quickEditMetadataLine(row, project, 46))
	for _, want := range []string{"EDIT TASK", "[project]", "Parent", "today"} {
		if !strings.Contains(narrow, want) {
			t.Errorf("narrow metadata missing %q: %q", want, narrow)
		}
	}
	for _, omitted := range []string{"#gamma", "pin", "zzz"} {
		if strings.Contains(narrow, omitted) {
			t.Errorf("narrow metadata retained low-priority %q: %q", omitted, narrow)
		}
	}

	parent := project.Tasks[0]
	parent.Title = "Parent marker with a deliberately long descriptive title"
	project.BuildTaskTree()
	minimum := ansi.Strip(out.quickEditMetadataLine(row, project, minTerminalWidth))
	for _, want := range []string{"EDIT TASK", "[project]", "Parent", "today"} {
		if !strings.Contains(minimum, want) {
			t.Errorf("minimum-width metadata missing mandatory segment %q: %q", want, minimum)
		}
	}
	if got := lipgloss.Width(minimum); got > minTerminalWidth {
		t.Errorf("minimum-width metadata is %d columns: %q", got, minimum)
	}

	out.quickEdit.input.SetValue("Child !9")
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()
	errorLine := ansi.Strip(out.quickEditMetadataLine(row, project, 72))
	for _, want := range []string{"EDIT TASK", "invalid priority", "[project]", "backlog"} {
		if !strings.Contains(errorLine, want) {
			t.Errorf("error metadata missing %q: %q", want, errorLine)
		}
	}
}

func TestInlineRowEditorKeepsSelectedCursorVisibleAtEveryPositionAndTheme(t *testing.T) {
	for _, theme := range []string{"dark", "light"} {
		t.Run(theme, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: theme})
			t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

			out, project, _, child := inlineEditorFixture(t)
			out.quickEdit.EnterWithContext(child, project, []*models.Project{project}, false)
			row := out.rows[out.cursor]
			positions := []int{0, len([]rune(child.Title)) / 2, len([]rune(child.Title))}

			for _, position := range positions {
				out.quickEdit.input.SetCursor(position)
				rendered := out.renderOutlinerQuickEditRow(row, 80)
				if !strings.Contains(rendered, "\x1b[7;") {
					t.Errorf("cursor at position %d has no reverse-video block: %q", position, rendered)
				}

				styles := out.quickEdit.input.Styles()
				if styles.Cursor.Blink {
					t.Errorf("cursor at position %d is blinking, want steady", position)
				}
				if got := styles.Focused.Text.GetForeground(); got != ColorSelectedFg {
					t.Errorf("selected input foreground at position %d = %v, want %v", position, got, ColorSelectedFg)
				}
				if got := styles.Focused.Text.GetBackground(); got != ColorSelectedBg {
					t.Errorf("selected input background at position %d = %v, want %v", position, got, ColorSelectedBg)
				}
			}
		})
	}
}

func TestInlineRowEditorAutocompleteIsBoundedAndViewportTracksResize(t *testing.T) {
	tasks := make([]*models.Task, 0, 14)
	for i := 0; i < 14; i++ {
		tasks = append(tasks, &models.Task{
			ID: fmt.Sprintf("task-%02d", i), ShortID: i + 1, Title: fmt.Sprintf("Task %02d", i),
			WorkflowState: models.StateBacklog, Tags: []string{fmt.Sprintf("tag-%02d", i)},
		})
	}
	project := &models.Project{ID: "many", Slug: "many", Name: "Many", Tasks: tasks}
	project.BuildTaskTree()
	out := NewOutlinerModel(project, nil, nil)
	out.SetSize(60, 8)
	out.rebuildRows()
	for index, row := range out.rows {
		if row.task != nil && row.task.ID == tasks[len(tasks)-1].ID {
			out.cursor = index
		}
	}
	out.quickEdit.EnterWithContext(tasks[len(tasks)-1], project, []*models.Project{project}, false)
	out.quickEdit.input.SetValue(tasks[len(tasks)-1].Title + " #")
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()
	if got := out.quickEdit.SuggestionLineCount(out.quickEditSuggestionBudget()); got != 5 {
		t.Fatalf("suggestion rows=%d, want bounded five", got)
	}

	wide := ansi.Strip(out.renderOutlinerTreeContent(60))
	if lipgloss.Height(wide) > 8 || !strings.Contains(wide, tasks[len(tasks)-1].Title+" #") {
		t.Fatalf("scrolled editor escaped viewport height=%d:\n%s", lipgloss.Height(wide), wide)
	}
	if strings.Contains(wide, tasks[0].Title) {
		t.Fatalf("viewport did not remain anchored to edited tail row:\n%s", wide)
	}

	out.SetSize(44, 7)
	narrow := ansi.Strip(out.renderOutlinerTreeContent(44))
	if lipgloss.Height(narrow) > 7 || !strings.Contains(narrow, tasks[len(tasks)-1].Title+" #") {
		t.Fatalf("resized editor escaped viewport height=%d:\n%s", lipgloss.Height(narrow), narrow)
	}
}

func TestInlineRowEditorRendersOnFoldedParentWithoutLeakingChildren(t *testing.T) {
	out, project, parent, child := inlineEditorFixture(t)
	out.expandedNodes = map[TaskIdentity]bool{{ProjectID: project.ID, TaskID: parent.ID}: false}
	out.rebuildRows()
	for index, row := range out.rows {
		if row.task != nil && row.task.ID == parent.ID {
			out.cursor = index
		}
	}
	out.quickEdit.EnterWithContext(parent, project, []*models.Project{project}, false)
	plain := ansi.Strip(out.renderOutlinerTreeContent(80))
	if !strings.Contains(plain, "EDIT TASK") || !strings.Contains(plain, parent.Title) {
		t.Fatalf("folded parent editor missing:\n%s", plain)
	}
	if strings.Contains(plain, child.Title) {
		t.Fatalf("folded editor leaked hidden child:\n%s", plain)
	}
}
