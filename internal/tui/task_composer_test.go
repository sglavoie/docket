package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestTaskComposerHeaderShowsEffectiveRoute(t *testing.T) {
	inbox := &models.Project{ID: "inbox", Name: "Inbox", Slug: "inbox"}
	target := &models.Project{ID: "tt", Name: "tt", Slug: "tt"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            inbox,
		AllProjects:        []*models.Project{inbox, target},
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetSize(80, 30)
	composer.input.SetValue("Write regression test %tt $doing")
	composer.updatePreview()

	project, destination := composer.effectiveRouteHeader()
	if project != "tt" || destination != models.StateDoing.Icon()+" $doing" {
		t.Fatalf("effective header = %q / %q, want tt / doing", project, destination)
	}
	plain := ansi.Strip(composer.View().Content)
	if strings.Contains(plain, "Preset:") || !strings.Contains(plain, "Destination:") {
		t.Fatalf("composer retained launch preset metadata:\n%s", plain)
	}
}

func TestTaskComposerDueTodayRoutesCaptureToToday(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
		Shared: TaskComposerSharedAttributes{
			Destination: app.CaptureDestinationBacklog,
		},
	}, nil)
	composer.input.SetValue("Due today @today")

	requests, ok := composer.parseCompactLines()
	if !ok || len(requests) != 1 {
		t.Fatalf("parseCompactLines = %+v, %v", requests, ok)
	}
	if requests[0].Destination != app.CaptureDestinationToday {
		t.Fatalf("destination = %q, want Today", requests[0].Destination)
	}
	_, destination := composer.effectiveRouteHeader()
	if destination != renderComposerDestination(app.CaptureDestinationToday) {
		t.Fatalf("displayed destination = %q, want Today", destination)
	}
}

func TestTaskComposerHeaderAggregatesMultilineRoutes(t *testing.T) {
	inbox := &models.Project{ID: "inbox", Name: "Inbox", Slug: "inbox"}
	target := &models.Project{ID: "tt", Name: "tt", Slug: "tt"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            inbox,
		AllProjects:        []*models.Project{inbox, target},
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)

	tests := []struct {
		name            string
		input           string
		wantProject     string
		wantDestination string
	}{
		{"uniform", "One %tt $doing\nTwo %tt $doing", "tt", models.StateDoing.Icon() + " $doing"},
		{"mixed project", "One %tt $doing\nTwo $doing", "Mixed", models.StateDoing.Icon() + " $doing"},
		{"mixed destination", "One %tt $doing\nTwo %tt $waiting", "tt", "Mixed"},
		{"both mixed", "One %tt $doing\nTwo $waiting", "Mixed", "Mixed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composer.input.SetValue(tt.input)
			project, destination := composer.effectiveRouteHeader()
			if project != tt.wantProject || destination != tt.wantDestination {
				t.Fatalf("effective header = %q / %q, want %q / %q", project, destination, tt.wantProject, tt.wantDestination)
			}
		})
	}
}

func TestTaskComposerAutocompleteSuppressesUsedSingletonsPerLine(t *testing.T) {
	parent := &models.Task{ID: "parent", ShortID: 1, Title: "Parent"}
	project := &models.Project{ID: "project", Name: "Project", Slug: "project", Tasks: []*models.Task{parent}}
	target := &models.Project{ID: "target", Name: "Target", Slug: "target"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            project,
		AllProjects:        []*models.Project{project, target},
		AllowProjectSwitch: true,
	}, nil)

	for _, input := range []string{
		"Task !1 !",
		"Task @tomorrow @",
		"Task $doing $",
		"Task %target %",
		"Task >1 >",
	} {
		composer.input.SetValue(input)
		composer.refreshAutocomplete()
		if composer.ac.IsActive() {
			t.Errorf("autocomplete remained active for used singleton in %q: %+v", input, composer.ac)
		}
	}

	composer.input.SetValue("First $doing\nSecond $")
	composer.refreshAutocomplete()
	if !composer.ac.IsActive() || composer.ac.Mode != acModeDestination {
		t.Fatalf("singleton on another task line suppressed destination autocomplete: %+v", composer.ac)
	}
}

func TestTaskComposerAutocompleteKeepsUnusedTagsRepeatable(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.tagCache = []string{"backend", "frontend", "urgent"}
	composer.input.SetValue("Task #backend #")
	composer.refreshAutocomplete()

	if !composer.ac.IsActive() || composer.ac.Mode != acModeTag {
		t.Fatalf("repeatable tag autocomplete is inactive: %+v", composer.ac)
	}
	values := make([]string, len(composer.ac.Items))
	for i, item := range composer.ac.Items {
		values[i] = item.Value
	}
	if slices.Contains(values, "backend") || !slices.Contains(values, "frontend") || !slices.Contains(values, "urgent") {
		t.Fatalf("tag suggestions = %v, want unused tags only", values)
	}
}

func TestTaskComposerAutocompleteUsesAvailableHeight(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 30)
	composer.input.SetValue("Task $")
	composer.updatePreview()
	composer.refreshAutocomplete()

	tall := ansi.Strip(composer.View().Content)
	if strings.Contains(tall, "more") || !strings.Contains(tall, "$canceled") {
		t.Fatalf("tall composer did not show every destination:\n%s", tall)
	}

	composer.SetSize(80, 18)
	short := ansi.Strip(composer.View().Content)
	if !strings.Contains(short, "more") {
		t.Fatalf("short composer did not show an overflow hint:\n%s", short)
	}
	if got := lipgloss.Height(short); got > 18 {
		t.Fatalf("short composer height = %d, want at most 18", got)
	}
}

func TestTaskComposerInputDividerSpacing(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 30)
	composer.input.SetValue("Draft task")
	composer.updatePreview()

	lines := strings.Split(ansi.Strip(composer.View().Content), "\n")
	inputLine := -1
	for i, line := range lines {
		if strings.Contains(line, "Draft task") {
			inputLine = i
			break
		}
	}
	if inputLine < 0 || inputLine+5 >= len(lines) {
		t.Fatalf("could not locate input and lower pane:\n%s", strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(lines[inputLine+1]) != "" || strings.TrimSpace(lines[inputLine+2]) != "" {
		t.Fatalf("expected two blank rows above divider:\n%s", strings.Join(lines[inputLine:inputLine+6], "\n"))
	}
	divider := strings.TrimSpace(lines[inputLine+3])
	if divider != strings.Repeat("─", 80) {
		t.Fatalf("divider = %q, want 80-column rule", divider)
	}
	if strings.TrimSpace(lines[inputLine+4]) != "" || !strings.HasPrefix(lines[inputLine+5], "Task") {
		t.Fatalf("expected one blank row below divider before preview:\n%s", strings.Join(lines[inputLine:inputLine+6], "\n"))
	}
}

func TestTaskComposerPresetModes(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	parentID := task.ID
	tests := []struct {
		name       string
		preset     TaskComposerPreset
		wantMode   TaskComposerMode
		wantKind   TaskComposerPresetKind
		wantParent bool
	}{
		{"global", TaskComposerPreset{Kind: ComposerGlobal, Project: project}, ComposerCompact, ComposerGlobal, false},
		{"project", TaskComposerPreset{Kind: ComposerProject, Project: project}, ComposerCompact, ComposerProject, false},
		{"child", TaskComposerPreset{Kind: ComposerChild, Project: project, Shared: TaskComposerSharedAttributes{ParentID: &parentID}}, ComposerCompact, ComposerChild, true},
		{"edit", TaskComposerPreset{Kind: ComposerEdit, Project: project, Task: task}, ComposerExpanded, ComposerEdit, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composer := NewTaskComposerModel(tt.preset, appModel.runtime)
			if composer.mode != tt.wantMode || composer.preset.Kind != tt.wantKind {
				t.Fatalf("mode/kind = %v/%v, want %v/%v", composer.mode, composer.preset.Kind, tt.wantMode, tt.wantKind)
			}
			if tt.wantParent && (composer.preset.Shared.ParentID == nil || *composer.preset.Shared.ParentID != task.ID) {
				t.Fatal("child preset lost its typed parent")
			}
			if tt.wantMode == ComposerExpanded && composer.expanded == nil {
				t.Fatal("expanded preset did not construct the full form")
			}
		})
	}

	editComposer := NewTaskComposerModel(tests[3].preset, appModel.runtime)
	editComposer.expanded.titleInput.SetValue("one\ntwo")
	editComposer.expanded.commitAllTextInputs()
	if editComposer.expanded.draft.title != "one two" {
		t.Fatalf("edit interpreted multiple lines as tasks: %q", editComposer.expanded.draft.title)
	}
}

func TestTaskComposerMultilineValidationIsAtomicAndUsesSourceLines(t *testing.T) {
	env := newScenarioEnv(t, "composer-atomic")
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:    ComposerProject,
		Project: env.project,
		Shared:  TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	input := "First task\n\n!1\nThird task"
	composer.input.SetValue(input)

	if cmd := composer.createCompact(false); cmd != nil {
		t.Fatalf("invalid batch returned commit command %T", cmd())
	}
	if len(composer.errors) != 1 || composer.errors[0].line != 3 {
		t.Fatalf("errors = %+v, want source line 3", composer.errors)
	}
	if composer.input.Value() != input {
		t.Fatalf("failed batch changed input: %q", composer.input.Value())
	}
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("atomic validation created %d tasks", got)
	}
}

func TestTaskComposerDoubleSubmitCreatesOneTaskAndOneCommit(t *testing.T) {
	env := newScenarioEnv(t, "composer-double-submit")
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:    ComposerProject,
		Project: env.project,
		Shared:  TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.input.SetValue("Create once")

	_, first := composer.Update(syntheticSpecialKey(tea.KeyEnter))
	_, duplicate := composer.Update(syntheticCtrlKey('s'))
	if first == nil || duplicate != nil || !composer.submissionInFlight {
		t.Fatalf("commands first=%v duplicate=%v inFlight=%v", first != nil, duplicate != nil, composer.submissionInFlight)
	}
	msg, ok := first().(TaskCreatedMsg)
	if !ok || msg.Result == nil {
		t.Fatalf("first result = %#v", msg)
	}
	composer.Update(msg)
	if composer.submissionInFlight {
		t.Fatal("success did not release the composer submission guard")
	}

	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 || fresh.Tasks[0].ShortID != 1 || fresh.NextShortID != 2 {
		t.Fatalf("created tasks=%+v nextShortID=%d", fresh.Tasks, fresh.NextShortID)
	}
	env.undo(t)
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("one undo left %d tasks; duplicate commit escaped the guard", got)
	}
}

func TestTaskComposerMultilineDoubleSubmitMintsEachShortIDOnce(t *testing.T) {
	env := newScenarioEnv(t, "composer-multiline-double-submit")
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:    ComposerProject,
		Project: env.project,
		Shared:  TaskComposerSharedAttributes{Destination: app.CaptureDestinationBacklog},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.input.SetValue("First\nSecond")

	_, first := composer.Update(syntheticSpecialKey(tea.KeyEnter))
	_, duplicate := composer.Update(syntheticSpecialKey(tea.KeyEnter))
	if first == nil || duplicate != nil {
		t.Fatalf("commands first=%v duplicate=%v", first != nil, duplicate != nil)
	}
	msg, ok := first().(TaskCreatedMsg)
	if !ok || msg.CreatedCount != 2 {
		t.Fatalf("multiline result = %#v", msg)
	}
	composer.Update(msg)

	fresh := env.reload(t)
	firstTask := fresh.GetTaskByShortID(1)
	secondTask := fresh.GetTaskByShortID(2)
	if len(fresh.Tasks) != 2 || firstTask == nil || secondTask == nil || firstTask.ID == secondTask.ID || fresh.NextShortID != 3 {
		t.Fatalf("multiline tasks=%+v nextShortID=%d", fresh.Tasks, fresh.NextShortID)
	}
	env.undo(t)
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("one undo left %d tasks; multiline capture committed twice", got)
	}
}

func TestTaskComposerSubmissionGuardClearsAfterError(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	stub := &stubMutator{err: errors.New("capture failed")}
	features := newFeatureCoordinators(stub)
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetCaptureCoordinator(features.Capture)
	composer.input.SetValue("Retry me")

	_, first := composer.Update(syntheticSpecialKey(tea.KeyEnter))
	_, duplicate := composer.Update(syntheticCtrlKey('s'))
	if first == nil || duplicate != nil {
		t.Fatalf("commands first=%v duplicate=%v", first != nil, duplicate != nil)
	}
	failure, ok := first().(ErrorMsg)
	if !ok || failure.Err == nil || stub.calls != 1 {
		t.Fatalf("failure=%#v calls=%d", failure, stub.calls)
	}
	composer.Update(failure)
	if composer.submissionInFlight {
		t.Fatal("error did not release the composer submission guard")
	}
	if _, retry := composer.Update(syntheticCtrlKey('s')); retry == nil {
		t.Fatal("composer did not allow retry after failure")
	}
}

func TestTaskComposerBulkCreationAndUndoAreOneTransaction(t *testing.T) {
	env := newScenarioEnv(t, "composer-bulk")
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:    ComposerProject,
		Project: env.project,
		Shared: TaskComposerSharedAttributes{
			Destination: app.CaptureDestinationBacklog,
			Tags:        []string{"shared", "story"},
		},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.input.SetValue("First #one #bug\nSecond !1")

	cmd := composer.createCompact(false)
	if cmd == nil {
		t.Fatal("valid batch returned nil command")
	}
	msg, ok := cmd().(TaskCreatedMsg)
	if !ok || msg.CreatedCount != 2 {
		t.Fatalf("bulk result = %#v, want TaskCreatedMsg count 2", msg)
	}
	fresh := env.reload(t)
	if len(fresh.Tasks) != 2 || !fresh.Tasks[0].HasTag("shared") || !fresh.Tasks[0].HasTag("one") ||
		!fresh.Tasks[0].HasTag("bug") || !fresh.Tasks[0].HasTag("story") ||
		!fresh.Tasks[1].HasTag("story") || fresh.Tasks[1].Priority != 1 {
		t.Fatalf("shared/per-line attributes not committed: %+v", fresh.Tasks)
	}

	env.undo(t)
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("one undo left %d composer tasks", got)
	}
}

func TestTaskComposerEnterSubmitsCompleteNewTagDespiteFuzzySuggestion(t *testing.T) {
	existing, err := models.NewGenericTask("existing", 1, "Existing")
	if err != nil {
		t.Fatal(err)
	}
	existing.Tags = []string{"backend"}
	env := newScenarioEnv(t, "composer-new-tag", existing)
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:    ComposerProject,
		Project: env.project,
		Shared:  TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.input.SetValue("Fresh task #back")
	composer.updatePreview()
	composer.refreshAutocomplete()
	if !composer.ac.IsActive() || !composer.autocompleteLiteralComplete() {
		t.Fatalf("expected active fuzzy suggestions for a complete literal: %+v", composer.ac)
	}

	_, cmd := composer.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter accepted autocomplete instead of submitting the complete literal")
	}
	if msg, ok := cmd().(TaskCreatedMsg); !ok || msg.Result == nil {
		t.Fatalf("submit result = %#v", msg)
	}
	fresh := env.reload(t)
	created := fresh.GetTaskByShortID(2)
	if created == nil || !created.HasTag("back") {
		t.Fatalf("created task = %+v, want on-the-fly #back tag", created)
	}
}

func TestTaskComposerRoutesMultilineProjectsAndParentsInOneTransaction(t *testing.T) {
	parent, err := models.NewGenericTask("beta-parent", 1, "Beta Parent")
	if err != nil {
		t.Fatal(err)
	}
	env := newCrossProjectTestEnv(t, nil, []*models.Task{parent})
	features := newFeatureCoordinators(env.svc)
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            env.projects[0],
		AllProjects:        env.projects,
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(features.Capture)
	composer.input.SetValue("Alpha task\nBeta child %out1 >1")

	cmd := composer.createCompact(false)
	if cmd == nil {
		t.Fatal("routed batch returned nil command")
	}
	msg, ok := cmd().(TaskCreatedMsg)
	if !ok || msg.CreatedCount != 2 {
		t.Fatalf("routed result = %#v, want TaskCreatedMsg count 2", msg)
	}
	alpha := env.reload(t, 0)
	beta := env.reload(t, 1)
	if len(alpha.Tasks) != 1 || alpha.Tasks[0].Title != "Alpha task" || alpha.Tasks[0].State() != models.StateInbox {
		t.Fatalf("alpha capture = %+v", alpha.Tasks)
	}
	child := beta.GetTaskByShortID(2)
	if len(beta.Tasks) != 2 || child == nil || child.Title != "Beta child" ||
		child.ParentID == nil || *child.ParentID != parent.ID || child.State() != models.StateBacklog {
		t.Fatalf("beta routed child = %+v tasks=%+v", child, beta.Tasks)
	}

	env.undo(t)
	if got := len(env.reload(t, 0).Tasks); got != 0 {
		t.Fatalf("one undo left %d alpha tasks", got)
	}
	if got := len(env.reload(t, 1).Tasks); got != 1 {
		t.Fatalf("one undo left %d beta tasks, want original parent", got)
	}
}

func TestTaskComposerExpandSingleAndRejectMultiline(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, appModel.runtime)
	composer.expandSingle()
	if composer.mode != ComposerExpanded || composer.expanded == nil {
		t.Fatal("blank single-task draft could not enter expanded mode")
	}

	composer = NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, appModel.runtime)
	composer.input.SetValue("One\nTwo")
	composer.expandSingle()
	if composer.mode != ComposerCompact || len(composer.errors) != 1 || !strings.Contains(composer.errors[0].message, "exactly one") {
		t.Fatalf("multiline expansion was not rejected: mode=%v errors=%+v", composer.mode, composer.errors)
	}

	composer.input.SetValue("Ship it #bug !1 #release $backlog")
	composer.expandSingle()
	if composer.mode != ComposerExpanded || composer.expanded == nil {
		t.Fatal("single line did not expand")
	}
	if composer.expanded.draft.title != "Ship it" || composer.expanded.draft.priority != 1 ||
		!slices.Contains(composer.expanded.draft.tags, "bug") || !slices.Contains(composer.expanded.draft.tags, "release") {
		t.Fatalf("expanded draft lost compact fields: %+v", composer.expanded.draft)
	}
}

func TestTaskComposerUsesTypedOverlayWithoutLegacyRoute(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	installTestSystemInbox(appModel, project)

	handled, _ := appModel.handleViewMessages(appModel.captureComposerMessage(project, appModel.taskWorkspaceProjects()))
	if !handled || !appModel.activeTaskComposer() {
		t.Fatal("Task Composer message did not activate Task Composer")
	}
	overlay, ok := appModel.navigation.Overlays.Current()
	if !ok || overlay.Kind() != OverlayComposer || overlay.Key() != taskComposerOverlayKey {
		t.Fatalf("overlay = %+v", overlay)
	}
	if _, legacy := overlay.Presentation(); legacy {
		t.Fatal("Task Composer opened a legacy capture/form overlay route")
	}
	if _, ok := appModel.activeView().(*TaskComposerModel); !ok {
		t.Fatalf("active view = %T, want TaskComposerModel", appModel.activeView())
	}
}

func TestTaskComposerCancellationFocusAndNarrowLayout(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, appModel.runtime)
	composer.SetSize(60, 18)
	if !composer.input.Focused() || !composer.HasTextInput() || !composer.ShouldCaptureEsc() {
		t.Fatal("compact composer did not own initial text focus and cancellation")
	}
	for _, line := range strings.Split(composer.View().Content, "\n") {
		if width := lipgloss.Width(line); width > 60 {
			t.Fatalf("narrow compact line width = %d: %q", width, line)
		}
	}

	composer.input.SetValue("draft")
	composer.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !composer.confirmDiscard {
		t.Fatal("dirty compact cancellation skipped confirmation")
	}
	if dialog := composer.GetConfirmDialog(); dialog == nil || !dialog.IsActive() {
		t.Fatal("dirty compact cancellation did not expose its confirmation")
	}

	edit := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerEdit, Project: project, Task: project.Tasks[0]}, appModel.runtime)
	edit.SetSize(60, 18)
	if !strings.Contains(edit.View().Content, "Terminal too small") {
		t.Fatal("narrow expanded composer did not use the safe responsive guard")
	}
}

func TestDailyTaskEditsUseNarrowTypedOverlays(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	appModel.Update(EditTaskMsg{Project: project, Task: task})
	if appModel.currentView() != ViewTextEditor {
		t.Fatalf("title edit view = %v, want TextEditor", appModel.currentView())
	}
	if appModel.activeTaskComposer() || appModel.taskComposer != nil {
		t.Fatal("title edit opened the full Task Composer")
	}
	if appModel.pendingTaskTextEdit == nil ||
		appModel.pendingTaskTextEdit.Operation != app.TaskOperationEditTitle {
		t.Fatalf("title edit pending operation = %+v", appModel.pendingTaskTextEdit)
	}

	appModel.popView()
	appModel.Update(ShowTaskTextEditorMsg{
		Task: task, Project: project, Operation: app.TaskOperationEditDescription,
	})
	if appModel.currentView() != ViewTextEditor {
		t.Fatalf("description edit view = %v, want TextEditor", appModel.currentView())
	}
	if appModel.activeTaskComposer() || appModel.taskComposer != nil {
		t.Fatal("description edit opened the full Task Composer")
	}
	if appModel.pendingTaskTextEdit == nil ||
		appModel.pendingTaskTextEdit.Operation != app.TaskOperationEditDescription {
		t.Fatalf("description edit pending operation = %+v", appModel.pendingTaskTextEdit)
	}
}
