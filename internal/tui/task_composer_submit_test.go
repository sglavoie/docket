package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskform"
)

func TestTaskComposerDoubleSubmitCreatesOneTaskAndOneCommit(t *testing.T) {
	env := newScenarioEnv(t, "composer-double-submit")
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:    taskform.ComposerProject,
		Project: env.project,
		Shared:  taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.Input.SetValue("Create once")

	_, first := composer.Update(syntheticSpecialKey(tea.KeyEnter))
	_, duplicate := composer.Update(syntheticCtrlKey('s'))
	if first == nil || duplicate != nil || !composer.SubmissionInFlight {
		t.Fatalf("commands first=%v duplicate=%v inFlight=%v", first != nil, duplicate != nil, composer.SubmissionInFlight)
	}
	msg, ok := first().(coordinator.TaskCreatedMsg)
	if !ok || msg.Result == nil {
		t.Fatalf("first result = %#v", msg)
	}
	composer.Update(msg)
	if composer.SubmissionInFlight {
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
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:    taskform.ComposerProject,
		Project: env.project,
		Shared:  taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationBacklog},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.Input.SetValue("First\nSecond")

	_, first := composer.Update(syntheticSpecialKey(tea.KeyEnter))
	_, duplicate := composer.Update(syntheticSpecialKey(tea.KeyEnter))
	if first == nil || duplicate != nil {
		t.Fatalf("commands first=%v duplicate=%v", first != nil, duplicate != nil)
	}
	msg, ok := first().(coordinator.TaskCreatedMsg)
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
	features := coordinator.NewFeatureCoordinators(stub)
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{Kind: taskform.ComposerProject, Project: project}, nil)
	composer.SetCaptureCoordinator(features.Capture)
	composer.Input.SetValue("Retry me")

	_, first := composer.Update(syntheticSpecialKey(tea.KeyEnter))
	_, duplicate := composer.Update(syntheticCtrlKey('s'))
	if first == nil || duplicate != nil {
		t.Fatalf("commands first=%v duplicate=%v", first != nil, duplicate != nil)
	}
	failure, ok := first().(status.ErrorMsg)
	if !ok || failure.Err == nil || stub.calls != 1 {
		t.Fatalf("failure=%#v calls=%d", failure, stub.calls)
	}
	composer.Update(failure)
	if composer.SubmissionInFlight {
		t.Fatal("error did not release the composer submission guard")
	}
	if _, retry := composer.Update(syntheticCtrlKey('s')); retry == nil {
		t.Fatal("composer did not allow retry after failure")
	}
}

func TestTaskComposerBulkCreationAndUndoAreOneTransaction(t *testing.T) {
	env := newScenarioEnv(t, "composer-bulk")
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:    taskform.ComposerProject,
		Project: env.project,
		Shared: taskform.TaskComposerSharedAttributes{
			Destination: app.CaptureDestinationBacklog,
			Tags:        []string{"shared", "story"},
		},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.Input.SetValue("First #one #bug\nSecond !1")

	cmd := composer.CreateCompact(false)
	if cmd == nil {
		t.Fatal("valid batch returned nil command")
	}
	msg, ok := cmd().(coordinator.TaskCreatedMsg)
	if !ok || msg.CreatedCount != 2 {
		t.Fatalf("bulk result = %#v, want coordinator.TaskCreatedMsg count 2", msg)
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
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:    taskform.ComposerProject,
		Project: env.project,
		Shared:  taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.Input.SetValue("Fresh task #back")
	composer.UpdatePreview()
	composer.RefreshAutocomplete()
	if !composer.AC.IsActive() || !composer.AutocompleteLiteralComplete() {
		t.Fatalf("expected active fuzzy suggestions for a complete literal: %+v", composer.AC)
	}

	_, cmd := composer.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter accepted autocomplete instead of submitting the complete literal")
	}
	if msg, ok := cmd().(coordinator.TaskCreatedMsg); !ok || msg.Result == nil {
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
	features := coordinator.NewFeatureCoordinators(env.svc)
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:               taskform.ComposerGlobal,
		Project:            env.projects[0],
		AllProjects:        env.projects,
		AllowProjectSwitch: true,
		Shared:             taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(features.Capture)
	composer.Input.SetValue("Alpha task\nBeta child %out1 >1")

	cmd := composer.CreateCompact(false)
	if cmd == nil {
		t.Fatal("routed batch returned nil command")
	}
	msg, ok := cmd().(coordinator.TaskCreatedMsg)
	if !ok || msg.CreatedCount != 2 {
		t.Fatalf("routed result = %#v, want coordinator.TaskCreatedMsg count 2", msg)
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

func TestTaskComposerUsesTypedOverlayWithoutLegacyRoute(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	installTestSystemInbox(appModel, project)

	handled, _ := appModel.handleViewMessages(appModel.captureComposerMessage(project, appModel.taskWorkspaceProjects()))
	if !handled || !appModel.activeTaskComposer() {
		t.Fatal("Task Composer message did not activate Task Composer")
	}
	overlay, ok := appModel.viewNav.navigation.Overlays.Current()
	if !ok || overlay.Kind() != route.OverlayComposer || overlay.Key() != taskComposerOverlayKey {
		t.Fatalf("overlay = %+v", overlay)
	}
	if _, legacy := overlay.Presentation(); legacy {
		t.Fatal("Task Composer opened a legacy capture/form overlay route")
	}
	if _, ok := appModel.activeView().(*taskform.TaskComposerModel); !ok {
		t.Fatalf("active view = %T, want TaskComposerModel", appModel.activeView())
	}
}

func TestTaskComposerCancellationFocusAndNarrowLayout(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{Kind: taskform.ComposerProject, Project: project}, appModel.domain.runtime)
	composer.SetSize(60, 18)
	if !composer.Input.Focused() || !composer.HasTextInput() || !composer.ShouldCaptureEsc() {
		t.Fatal("compact composer did not own initial text focus and cancellation")
	}
	for _, line := range strings.Split(composer.View().Content, "\n") {
		if width := lipgloss.Width(line); width > 60 {
			t.Fatalf("narrow compact line width = %d: %q", width, line)
		}
	}

	composer.Input.SetValue("draft")
	composer.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !composer.ConfirmDiscard {
		t.Fatal("dirty compact cancellation skipped confirmation")
	}
	if dialog := composer.GetConfirmDialog(); dialog == nil || !dialog.IsActive() {
		t.Fatal("dirty compact cancellation did not expose its confirmation")
	}

	expanded := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{Kind: taskform.ComposerProject, Project: project}, appModel.domain.runtime)
	expanded.ExpandRequest(app.CaptureRequest{ProjectID: project.ID})
	expanded.SetSize(60, 18)
	// The expanded form's floor is the app's own, so the size every other surface
	// serves is a size it lays out rather than refuses: it shortens its controls
	// from the width it was given instead of spilling past the frame.
	expandedContent := expanded.View().Content
	if strings.Contains(expandedContent, "Terminal too small") {
		t.Fatal("expanded composer refused a size the app itself accepts")
	}
	for _, line := range strings.Split(expandedContent, "\n") {
		if width := lipgloss.Width(line); width > 60 {
			t.Fatalf("narrow expanded line width = %d: %q", width, line)
		}
	}
}

func TestDailyTaskEditsUseInlineTitleAndNarrowDescription(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	appModel.crossOutliner = outliner.NewOutlinerModel(nil, appModel.domain.manager, appModel.domain.runtime)
	appModel.crossOutliner.Population.Projects = []*models.Project{project}
	appModel.viewNav.views.Register(presentation.ViewCrossProjectOutliner, appModel.crossOutliner)
	appModel.viewNav.navigation.ResetRoot()

	appModel.Update(outliner.StartTaskLineEditMsg{Project: project, Task: task})
	if appModel.currentView() != presentation.ViewCrossProjectOutliner || appModel.crossOutliner == nil || !appModel.crossOutliner.QuickEdit.IsActive() {
		t.Fatalf("title edit did not remain in the inline Outliner: view=%v", appModel.currentView())
	}

	appModel.crossOutliner.QuickEdit.Exit()
	appModel.Update(chrome.ShowTaskTextEditorMsg{
		Task: task, Project: project, Operation: app.TaskOperationEditDescription,
	})
	if appModel.currentView() != presentation.ViewTextEditor {
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

func TestTaskComposerCreateAnotherCommitsOnceAndKeepsOriginalDefaults(t *testing.T) {
	env := newScenarioEnv(t, "capture-another")
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind: taskform.ComposerProject, Project: env.project,
		Shared: taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.Input.SetValue("First !1 #temporary $doing")
	_, cmd := composer.Update(syntheticCtrlKey('n'))
	if cmd == nil {
		t.Fatal("create another did not submit")
	}
	composer.Update(tea.PasteMsg{Content: "must not overwrite pending draft"})
	if composer.Input.Value() != "First !1 #temporary $doing" {
		t.Fatal("in-flight paste changed draft")
	}
	if _, duplicate := composer.Update(syntheticCtrlKey('n')); duplicate != nil {
		t.Fatal("duplicate submission")
	}
	raw := cmd()
	msg, ok := raw.(coordinator.TaskCreatedMsg)
	if !ok || !msg.KeepComposerOpen {
		t.Fatalf("result=%#v", raw)
	}
	composer.Update(msg)
	if composer.Input.Value() != "" || composer.SubmissionInFlight {
		t.Fatal("success did not reset draft")
	}
	composer.Input.SetValue("Second")
	_, cmd = composer.Update(syntheticSpecialKey(tea.KeyEnter))
	second, ok := cmd().(coordinator.TaskCreatedMsg)
	if !ok || second.KeepComposerOpen {
		t.Fatal("ordinary Enter inherited keep-open")
	}
	fresh := env.reload(t)
	task := fresh.GetTaskByShortID(2)
	if task == nil || task.State() != models.StateInbox || task.Priority != 0 || len(task.Tags) != 0 {
		t.Fatalf("next draft inherited metadata: %+v", task)
	}
	env.undo(t)
	if len(env.reload(t).Tasks) != 1 {
		t.Fatal("captures did not have independent undo entries")
	}
}

func TestTaskComposerCreateAnotherFailureRetainsDraft(t *testing.T) {
	project := &models.Project{ID: "test", Name: "Test"}
	stub := &stubMutator{err: errors.New("write refused")}
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{Kind: taskform.ComposerProject, Project: project}, nil)
	composer.SetCaptureCoordinator(coordinator.NewFeatureCoordinators(stub).Capture)
	composer.Input.SetValue("Keep this draft")
	_, cmd := composer.Update(syntheticCtrlKey('n'))
	composer.Update(cmd())
	if composer.Input.Value() != "Keep this draft" || composer.SubmissionInFlight {
		t.Fatal("failed save lost draft or blocked retry")
	}
}

func TestTaskComposerCreateAnotherKeepsPanelAndRefreshesOutline(t *testing.T) {
	driver, _ := menuBarDriver(t)
	for _, action := range []UITestAction{
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: "Capture another regression"},
		{Type: UITestActionKey, Key: "ctrl+n"},
	} {
		if _, err := driver.ApplyAction(action, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	m := driver.App()
	if m.currentView() != presentation.ViewTaskComposer || m.taskComposer.Input.Value() != "" {
		t.Fatal("capture another closed or retained submitted draft")
	}
	if !strings.Contains(m.feedback.message, "u undo") {
		t.Fatalf("missing recovery feedback: %s", m.feedback.message)
	}
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionKey, Key: "esc"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	if m.currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatal("empty capture did not close")
	}
	if !strings.Contains(m.View().Content, "Capture another regression") {
		t.Fatal("outline did not refresh committed task")
	}
}
