package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/template"
)

func TestTaskComposerPresetModes(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	parentID := task.ID
	tmpl := &template.TaskTemplate{Name: "Bug report", Type: models.TaskTypeBug, Priority: 1, Tags: []string{"triage"}}

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
		{"linked", TaskComposerPreset{Kind: ComposerLinked, Project: project, SourceTask: task, LinkType: LinkTypeBlocker}, ComposerCompact, ComposerLinked, false},
		{"template", TaskComposerPreset{Kind: ComposerTemplate, Project: project, Template: tmpl}, ComposerExpanded, ComposerTemplate, false},
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

	templateComposer := NewTaskComposerModel(tests[5].preset, appModel.runtime)
	if templateComposer.expanded.draft.taskType != models.TaskTypeBug || templateComposer.expanded.draft.priority != 1 {
		t.Fatalf("template defaults not applied: %+v", templateComposer.expanded.draft)
	}
	editComposer := NewTaskComposerModel(tests[6].preset, appModel.runtime)
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
		Shared:  TaskComposerSharedAttributes{Section: models.SectionInbox},
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
			Section: models.SectionNext,
			Tags:    []string{"shared"},
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

func TestTaskComposerLinkedMultilineIsAtomicAndUndoable(t *testing.T) {
	source, err := models.NewTask("source", 1, models.TaskTypeTask, "Source")
	if err != nil {
		t.Fatal(err)
	}
	env := newScenarioEnv(t, "composer-linked", source)
	composer := NewTaskComposerModel(linkedComposerPreset(env.project, source, LinkTypeBlocker), nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.input.SetValue("Blocker one\nBlocker two")

	cmd := composer.createCompact(false, false)
	if cmd == nil {
		t.Fatal("linked batch returned nil command")
	}
	msg, ok := cmd().(LinkedTaskBatchCreatedMsg)
	if !ok || len(msg.Results) != 2 {
		t.Fatalf("linked result = %#v", msg)
	}
	fresh := env.reload(t)
	committedSource := fresh.GetTaskByID(source.ID)
	if len(fresh.Tasks) != 3 || committedSource == nil || len(committedSource.DependsOn) != 2 {
		t.Fatalf("linked batch was not committed together: tasks=%d source=%+v", len(fresh.Tasks), committedSource)
	}

	env.undo(t)
	fresh = env.reload(t)
	committedSource = fresh.GetTaskByID(source.ID)
	if len(fresh.Tasks) != 1 || committedSource == nil || len(committedSource.DependsOn) != 0 {
		t.Fatalf("linked batch undo was partial: tasks=%d source=%+v", len(fresh.Tasks), committedSource)
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

	composer.input.SetValue("Ship it ^bug !1 #release /next")
	composer.expandSingle()
	if composer.mode != ComposerExpanded || composer.expanded == nil {
		t.Fatal("single line did not expand")
	}
	if composer.expanded.draft.title != "Ship it" || composer.expanded.draft.taskType != models.TaskTypeBug || composer.expanded.draft.priority != 1 {
		t.Fatalf("expanded draft lost compact fields: %+v", composer.expanded.draft)
	}
}

func TestTaskComposerUsesTypedOverlayWithoutLegacyRoute(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	installTestSystemInbox(appModel, project)
	appModel.enableTypedNavigation()

	handled, _ := appModel.handleViewMessages(ShowQuickCaptureMsg{Project: project})
	if !handled || !appModel.activeTaskComposer() {
		t.Fatal("quick-capture alias did not activate Task Composer")
	}
	overlay, ok := appModel.navigation.Overlays.Current()
	if !ok || overlay.Kind() != OverlayComposer || overlay.Key() != taskComposerOverlayKey {
		t.Fatalf("overlay = %+v", overlay)
	}
	if _, legacy := overlay.Presentation(); legacy {
		t.Fatal("Task Composer opened a legacy capture/form overlay route")
	}
	if _, legacyModel := appModel.activeView().(*QuickCaptureModel); legacyModel {
		t.Fatal("legacy QuickCaptureModel remained live")
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

	edit := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerEdit, Project: project, Task: project.Tasks[0]}, appModel.runtime)
	edit.SetSize(60, 18)
	if !strings.Contains(edit.View().Content, "Terminal too small") {
		t.Fatal("narrow expanded composer did not use the safe responsive guard")
	}
}

func TestTaskComposerPlannerPresetCarriesStateAndAtomicArtifacts(t *testing.T) {
	project := &models.Project{ID: "p", Name: "P"}
	builder := app.PlanningArtifactBuilder(func([]app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) { return nil, nil })
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:              ComposerPlanner,
		Project:           project,
		PlannerTab:        boardTabNext,
		Shared:            TaskComposerSharedAttributes{State: models.StateNext},
		PlanningArtifacts: builder,
	}, nil)
	composer.input.SetValue("Plan me")
	reqs, _, ok := composer.parseCompactLines()
	if !ok || len(reqs) != 1 || reqs[0].State != models.StateNext || reqs[0].PlanningArtifacts == nil {
		t.Fatalf("planner preset request = %+v errors=%+v", reqs, composer.errors)
	}
}
