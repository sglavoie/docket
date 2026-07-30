package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
		{"planner", TaskComposerPreset{Kind: ComposerPlanner, Project: project, PlannerTab: boardTabToday}, ComposerCompact, ComposerPlanner, false},
		{"child", TaskComposerPreset{Kind: ComposerChild, Project: project, Parent: task, Shared: TaskComposerSharedAttributes{ParentID: &parentID}}, ComposerCompact, ComposerChild, true},
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

	editComposer := NewTaskComposerModel(tests[4].preset, appModel.runtime)
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
		Shared:  TaskComposerSharedAttributes{State: models.StateInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	input := "First task\n\n!1\nThird task"
	composer.input.SetValue(input)

	if cmd := composer.createCompact(false, false); cmd != nil {
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

func TestTaskComposerBulkCreationAndUndoAreOneTransaction(t *testing.T) {
	env := newScenarioEnv(t, "composer-bulk")
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:    ComposerProject,
		Project: env.project,
		Shared: TaskComposerSharedAttributes{
			State: models.StateNext,
			Tags:  []string{"shared"},
		},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.input.SetValue("First #one\nSecond !1")

	cmd := composer.createCompact(false, false)
	if cmd == nil {
		t.Fatal("valid batch returned nil command")
	}
	msg, ok := cmd().(TaskCreatedMsg)
	if !ok || msg.CreatedCount != 2 {
		t.Fatalf("bulk result = %#v, want TaskCreatedMsg count 2", msg)
	}
	fresh := env.reload(t)
	if len(fresh.Tasks) != 2 || !fresh.Tasks[0].HasTag("shared") || !fresh.Tasks[0].HasTag("one") || fresh.Tasks[1].Priority != 1 {
		t.Fatalf("shared/per-line attributes not committed: %+v", fresh.Tasks)
	}

	env.undo(t)
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("one undo left %d composer tasks", got)
	}
}

func TestTaskComposerRoutesMultilineProjectsAndParentsInOneTransaction(t *testing.T) {
	parent, err := models.NewGenericTask("beta-parent", 1, "Beta Parent")
	if err != nil {
		t.Fatal(err)
	}
	env := newCrossProjectTestEnv(t, nil, []*models.Task{parent})
	features := newFeatureCoordinators(env.svc, env.svc)
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            env.projects[0],
		AllProjects:        env.projects,
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{State: models.StateInbox},
	}, nil)
	composer.SetCaptureCoordinator(features.Capture)
	composer.input.SetValue("Alpha task\nBeta child %out1 >1")

	cmd := composer.createCompact(false, false)
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
		child.ParentID == nil || *child.ParentID != parent.ID || child.State() != models.StateReady {
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

	composer.input.SetValue("Ship it ^bug !1 #release $next")
	composer.expandSingle()
	if composer.mode != ComposerExpanded || composer.expanded == nil {
		t.Fatal("single line did not expand")
	}
	if composer.expanded.draft.title != "Ship it ^bug" || composer.expanded.draft.priority != 1 {
		t.Fatalf("expanded draft lost compact fields: %+v", composer.expanded.draft)
	}
}

func TestTaskComposerUsesTypedOverlayWithoutLegacyRoute(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	installTestSystemInbox(appModel, project)

	handled, _ := appModel.handleViewMessages(appModel.captureComposerMessage(project, appModel.activeProjects()))
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

func TestTaskComposerPlannerPresetCarriesStateAndAtomicArtifacts(t *testing.T) {
	project := &models.Project{ID: "p", Name: "P"}
	builder := app.PlanningArtifactBuilder(func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) { return nil, nil })
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:              ComposerPlanner,
		Project:           project,
		PlannerTab:        boardTabNext,
		Shared:            TaskComposerSharedAttributes{State: models.StateNext},
		PlanningArtifacts: builder,
	}, nil)
	composer.input.SetValue("Plan me")
	reqs, ok := composer.parseCompactLines()
	if !ok || len(reqs) != 1 || reqs[0].State != models.StateNext || reqs[0].PlanningArtifacts == nil {
		t.Fatalf("planner preset request = %+v errors=%+v", reqs, composer.errors)
	}
}
