package taskform

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

func TestTaskComposerExpandSingleAndRejectMultiline(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, runtime)
	composer.ExpandSingle()
	if composer.Mode != ComposerExpanded || composer.Expanded == nil {
		t.Fatal("blank single-task draft could not enter expanded mode")
	}

	composer = NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, runtime)
	composer.Input.SetValue("One\nTwo")
	composer.ExpandSingle()
	if composer.Mode != ComposerCompact || len(composer.Errors) != 1 || !strings.Contains(composer.Errors[0].Message, "exactly one") {
		t.Fatalf("multiline expansion was not rejected: mode=%v errors=%+v", composer.Mode, composer.Errors)
	}

	composer.Input.SetValue("Ship it #bug !1 #release $backlog")
	composer.ExpandSingle()
	if composer.Mode != ComposerExpanded || composer.Expanded == nil {
		t.Fatal("single line did not expand")
	}
	if composer.Expanded.draft.title != "Ship it" || composer.Expanded.draft.priority != 1 ||
		!slices.Contains(composer.Expanded.draft.tags, "bug") || !slices.Contains(composer.Expanded.draft.tags, "release") {
		t.Fatalf("expanded draft lost compact fields: %+v", composer.Expanded.draft)
	}
}

func TestComposerExpansionSavesCapturedFlags(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserved", true: "edited"}[clear], func(t *testing.T) {
			env := newScenarioEnv(t, "expanded-flags")
			cfg := newTestConfig()
			composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: env.project}, testRuntimeAt(cfg.DataDir(), cfg))
			composer.SetTaskFormCoordinator(env.features.TaskForm)
			composer.Input.SetValue("Ship release *pin =review #release !1")
			composer.ExpandSingle()
			if composer.Expanded == nil {
				t.Fatalf("expansion refused: %+v", composer.Errors)
			}
			form := composer.Expanded
			if !form.draft.pinned || !form.draft.review {
				t.Fatal("captured flags did not reach the form")
			}
			if clear {
				form = navigateToField(form, 5)
				for _, key := range []rune{tea.KeyEnter, tea.KeySpace, tea.KeyDown, tea.KeySpace, tea.KeyEnter} {
					form = sendFormKey(form, syntheticSpecialKey(key))
				}
			}
			msg := runCreate(t, form)
			task := env.reload(t).GetTaskByID(msg.Result.TaskID)
			if task == nil || task.Pinned != !clear || task.Review != !clear || task.Priority != 1 || !slices.Contains(task.Tags, "release") {
				t.Fatalf("expanded capture lost values: %+v", task)
			}
			env.undo(t)
			if env.reload(t).GetTaskByID(msg.Result.TaskID) != nil {
				t.Fatal("expanded capture survived undo")
			}
			env.redo(t)
			task = env.reload(t).GetTaskByID(msg.Result.TaskID)
			if task == nil || task.Pinned != !clear || task.Review != !clear {
				t.Fatalf("redo lost captured flags: %+v", task)
			}
		})
	}
}

func TestComposerExpansionRefusesUnsupportedFieldsWithoutLosingDraft(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed func(*app.CaptureRequest)
		want string
	}{
		{"adoption", func(req *app.CaptureRequest) { req.ChildIDs = []string{"child"} }, "adopt children"},
		{"deferral", func(req *app.CaptureRequest) { req.DeferSet = true }, "defer tasks"},
		{"deferral instant", func(req *app.CaptureRequest) { now := time.Now(); req.Defer = &now }, "defer tasks"},
		{"dependency", func(req *app.CaptureRequest) { req.Dependencies = []app.TaskLocator{{TaskID: "other"}} }, "set dependencies"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime, project := newTestRuntimeWithProject()
			composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, runtime)
			composer.Input.SetValue("My unfinished capture")
			req := app.CaptureRequest{ProjectID: project.ID, Title: "My unfinished capture", SourceLine: 3}
			tc.seed(&req)
			composer.ExpandRequest(req)
			if composer.Mode != ComposerCompact || composer.Expanded != nil || composer.Input.Value() != "My unfinished capture" {
				t.Fatal("refused expansion changed the draft or presentation")
			}
			if len(composer.Errors) != 1 || composer.Errors[0].Line != 3 || !strings.Contains(composer.Errors[0].Message, tc.want) {
				t.Fatalf("missing actionable refusal: %+v", composer.Errors)
			}
			composer.Input.SetValue("Updated *pin =review")
			composer.ExpandSingle()
			if composer.Expanded == nil || len(composer.Errors) != 0 {
				t.Fatalf("corrected draft did not expand: %+v", composer.Errors)
			}
		})
	}
}
