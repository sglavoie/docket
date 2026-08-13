package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	snooze := due.Add(-24 * time.Hour)
	child.DueDate, child.SnoozedUntil = &due, &snooze
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

func TestInlineRowEditorBuildsAtomicCrossProjectRequest(t *testing.T) {
	_, source, _, child := inlineEditorFixture(t)
	parent := &models.Task{ID: "destination-parent", ShortID: 7, Title: "Destination Parent", WorkflowState: models.StateBacklog}
	destination := &models.Project{
		ID: "destination", Slug: "destination", Name: "Destination",
		Tasks: []*models.Task{parent},
	}
	destination.BuildTaskTree()

	result, err := parseQuickEdit(
		"Renamed !2 @0 $today #new -#alpha %destination >7",
		child, source, []*models.Project{source, destination},
	)
	if err != nil {
		t.Fatalf("parse cross-project row edit: %v", err)
	}
	req := taskLineEditRequest(taskIdentityFor(child, source), result)
	if req.Source.ProjectID != source.ID || req.Source.TaskID != child.ID || req.DestinationProjectID != destination.ID {
		t.Fatalf("request route = source %+v destination %q", req.Source, req.DestinationProjectID)
	}
	if req.Title != "Renamed" || req.Priority == nil || *req.Priority != models.PriorityMedium {
		t.Fatalf("request content = %+v", req)
	}
	if !req.DueSet || req.Due != nil || req.Destination == nil || *req.Destination != app.CaptureDestinationToday {
		t.Fatalf("request destination/due = destination %v dueSet %t due %v", req.Destination, req.DueSet, req.Due)
	}
	if req.Parent.Mode != app.TaskLineParentSet || req.Parent.Parent.ProjectID != destination.ID || req.Parent.Parent.TaskID != parent.ID {
		t.Fatalf("request parent = %+v", req.Parent)
	}
}

func TestInlineRowEditorAtomicCommitAndFailureLifecycle(t *testing.T) {
	m, _, task, _ := newPlanningLedgerApp(t, models.StateBacklog)
	out := NewOutlinerModel(m.currentProject, m.configManager, m.runtime)
	out.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	out.SetSize(100, 20)
	out.rebuildRows()
	out.expandedNodes[taskIdentityFor(task, m.currentProject)] = false
	out.collapsedSections["unrelated"] = true
	m.crossOutliner = out
	m.navigation.ResetRoot()
	projection := out.jumpProjectionFingerprint()

	out.Update(StartTaskLineEditMsg{Task: task, Project: m.currentProject})
	out.quickEdit.input.SetValue("Renamed atomically")
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()
	_, cmd := out.handleQuickEditMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !out.quickEdit.IsActive() || !out.quickEdit.pending {
		t.Fatalf("commit did not retain a pending inline draft: active=%t pending=%t", out.quickEdit.IsActive(), out.quickEdit.pending)
	}
	committed, ok := cmd().(TaskLineEditedMsg)
	if !ok || !committed.Result.Changed || committed.Result.TransactionID == "" {
		t.Fatalf("atomic command result = %#v", committed)
	}
	m.handleTaskLineEdited(committed)
	if out.quickEdit.IsActive() {
		t.Fatal("successful atomic edit left the row editor active")
	}
	if got := m.currentProject.GetTaskByID(task.ID).Title; got != "Renamed atomically" {
		t.Fatalf("reloaded title = %q, want Renamed atomically", got)
	}
	if got := out.jumpProjectionFingerprint(); got != projection {
		t.Fatalf("successful inline edit changed projection state:\n before %s\n after  %s", projection, got)
	}

	live := m.currentProject.GetTaskByID(task.ID)
	out.Update(StartTaskLineEditMsg{Task: live, Project: m.currentProject})
	out.quickEdit.input.SetValue("Draft survives")
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()
	out.quickEdit.BeginCommit()
	projection = out.jumpProjectionFingerprint()
	failure := errors.New("simulated revision conflict")
	m.handleTaskLineEditFailed(TaskLineEditFailedMsg{
		Source: taskIdentityFor(live, m.currentProject), Err: failure,
	})
	if !out.quickEdit.IsActive() || out.quickEdit.pending || out.quickEdit.input.Value() != "Draft survives" {
		t.Fatalf("failed commit lost draft: active=%t pending=%t value=%q", out.quickEdit.IsActive(), out.quickEdit.pending, out.quickEdit.input.Value())
	}
	if !strings.Contains(out.quickEdit.GetPreviewError(), failure.Error()) {
		t.Fatalf("inline failure = %q, want %q", out.quickEdit.GetPreviewError(), failure)
	}
	if got := out.jumpProjectionFingerprint(); got != projection {
		t.Fatalf("failed inline edit changed projection state:\n before %s\n after  %s", projection, got)
	}
}

func TestInlineRowEditorUnchangedCommitCreatesNoUndoTransaction(t *testing.T) {
	m, _, task, _ := newPlanningLedgerApp(t, models.StateBacklog)
	out := NewOutlinerModel(m.currentProject, m.configManager, m.runtime)
	out.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	out.SetSize(100, 20)
	out.rebuildRows()
	m.crossOutliner = out
	m.navigation.ResetRoot()

	before, err := m.deps.Ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("undo count before unchanged edit: %v", err)
	}

	out.Update(StartTaskLineEditMsg{Task: task, Project: m.currentProject})
	if !out.quickEdit.IsActive() || out.quickEdit.input.Value() != task.Title {
		t.Fatalf("inline editor did not open with unchanged title: active=%t value=%q", out.quickEdit.IsActive(), out.quickEdit.input.Value())
	}
	_, cmd := out.handleQuickEditMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("unchanged inline commit returned nil command")
	}
	committed, ok := cmd().(TaskLineEditedMsg)
	if !ok || committed.Result.Changed || committed.Result.TransactionID != "" {
		t.Fatalf("unchanged inline result = %#v", committed)
	}
	m.handleTaskLineEdited(committed)

	if out.quickEdit.IsActive() {
		t.Fatal("unchanged inline commit left the row editor active")
	}
	if m.statusMessage != "No task changes" {
		t.Fatalf("unchanged inline status = %q, want No task changes", m.statusMessage)
	}
	after, err := m.deps.Ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("undo count after unchanged edit: %v", err)
	}
	if after != before {
		t.Fatalf("unchanged inline edit created an undo entry: before=%d after=%d", before, after)
	}
}

func TestTaskLineEditRelocationFollowsVisibleResultAndFallsBackAtOrigin(t *testing.T) {
	makeOutliner := func() (*OutlinerModel, *models.Project, *models.Project, *models.Task) {
		task := &models.Task{ID: "moving", ShortID: 1, Title: "Moving", WorkflowState: models.StateBacklog}
		neighbor := &models.Task{ID: "neighbor", ShortID: 2, Title: "Neighbor", WorkflowState: models.StateBacklog}
		source := &models.Project{ID: "source", Slug: "source", Name: "Source", Tasks: []*models.Task{task, neighbor}}
		destination := &models.Project{ID: "destination", Slug: "destination", Name: "Destination"}
		out := NewOutlinerModel(nil, nil, nil)
		out.loadedProjects = []*models.Project{source, destination}
		out.crossSnapshotReady = true
		out.crossForceRebuild = true
		out.crossProjectRebuildRows()
		for index, row := range out.rows {
			if row.task != nil && row.task.ID == task.ID && row.ProjectID == source.ID {
				out.cursor = index
			}
		}
		out.syncJumpProjection()
		return out, source, destination, task
	}

	out, source, destination, task := makeOutliner()
	capture := out.captureOutlinerRelocation([]TaskIdentity{taskIdentityFor(task, source)})
	source.Tasks = source.Tasks[1:]
	destination.Tasks = append(destination.Tasks, task)
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	_, recorded := out.finishTaskLineEditRelocation(capture, taskIdentityFor(task, destination))
	if !recorded || out.currentTaskIdentity() != taskIdentityFor(task, destination) || len(out.relocationHistory.back) != 1 {
		t.Fatalf("visible relocation = current %+v recorded=%t history=%d", out.currentTaskIdentity(), recorded, len(out.relocationHistory.back))
	}

	out, source, _, task = makeOutliner()
	capture = out.captureOutlinerRelocation([]TaskIdentity{taskIdentityFor(task, source)})
	source.Tasks = source.Tasks[1:]
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	_, recorded = out.finishTaskLineEditRelocation(capture, TaskIdentity{ProjectID: "hidden", TaskID: task.ID})
	if recorded || out.currentTaskIdentity().TaskID != "neighbor" || len(out.relocationHistory.back) != 0 {
		t.Fatalf("hidden relocation = current %+v recorded=%t history=%d", out.currentTaskIdentity(), recorded, len(out.relocationHistory.back))
	}
}

// longestVisibleSuffix reports how many trailing runes of value the rendered line
// still shows, which is what distinguishes a horizontally scrolled window from a
// window computed for a stale width.
func longestVisibleSuffix(line, value string) int {
	runes := []rune(value)
	for index := range runes {
		if strings.Contains(line, string(runes[index:])) {
			return len(runes) - index
		}
	}
	return 0
}

func TestInlineRowEditorKeepsTypedTailAndCursorVisibleWhenTitleOverflows(t *testing.T) {
	out, project, _, child := inlineEditorFixture(t)
	out.quickEdit.EnterWithContext(child, project, []*models.Project{project}, false)
	row := out.rows[out.cursor]

	// The first frame is what installs the row width, so the value has to land
	// afterwards for the scroll window to be the one an editing session sees.
	out.renderOutlinerQuickEditRow(row, 80)
	value := strings.Repeat("overflowing title ", 8) + "ZZEND"
	out.quickEdit.input.SetValue(value)
	out.quickEdit.input.CursorEnd()
	out.quickEdit.refreshDraft()

	lines := strings.Split(out.renderOutlinerQuickEditRow(row, 80), "\n")
	if len(lines) != 2 {
		t.Fatalf("quick edit rendered %d lines, want the metadata and input lines only", len(lines))
	}
	for _, line := range lines {
		if got := lipgloss.Width(ansi.Strip(line)); got > 80 {
			t.Fatalf("quick edit line is %d columns, want at most 80: %q", got, ansi.Strip(line))
		}
	}

	inputLine := ansi.Strip(lines[1])
	if !strings.Contains(inputLine, "ZZEND") {
		t.Errorf("tail at the cursor is not visible: %q", inputLine)
	}
	if strings.Contains(inputLine, "…") {
		t.Errorf("input line is ellipsized instead of horizontally scrolled: %q", inputLine)
	}
	if !strings.Contains(lines[1], "\x1b[7;") {
		t.Errorf("overflowing input line has no reverse-video cursor block: %q", lines[1])
	}

	narrow := ansi.Strip(strings.Split(out.renderOutlinerQuickEditRow(row, 40), "\n")[1])
	wide := ansi.Strip(strings.Split(out.renderOutlinerQuickEditRow(row, 120), "\n")[1])
	narrowVisible := longestVisibleSuffix(narrow, value)
	wideVisible := longestVisibleSuffix(wide, value)
	if wideVisible <= narrowVisible {
		t.Errorf("widening the row without a keystroke revealed %d of %d runes, narrow showed %d", wideVisible, len([]rune(value)), narrowVisible)
	}
}
