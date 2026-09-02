package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

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

	expanded := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, appModel.runtime)
	expanded.expandRequest(app.CaptureRequest{ProjectID: project.ID})
	expanded.SetSize(60, 18)
	if !strings.Contains(expanded.View().Content, "Terminal too small") {
		t.Fatal("narrow expanded composer did not use the safe responsive guard")
	}
}

func TestDailyTaskEditsUseInlineTitleAndNarrowDescription(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	appModel.crossOutliner = NewOutlinerModel(nil, appModel.configManager, appModel.runtime)
	appModel.crossOutliner.loadedProjects = []*models.Project{project}
	appModel.views.Register(ViewCrossProjectOutliner, appModel.crossOutliner)
	appModel.navigation.ResetRoot()

	appModel.Update(StartTaskLineEditMsg{Project: project, Task: task})
	if appModel.currentView() != ViewCrossProjectOutliner || appModel.crossOutliner == nil || !appModel.crossOutliner.quickEdit.IsActive() {
		t.Fatalf("title edit did not remain in the inline Outliner: view=%v", appModel.currentView())
	}

	appModel.crossOutliner.quickEdit.Exit()
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
