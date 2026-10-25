package taskform

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/grammar"
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

func TestComposerExpansionGuidanceFollowsDraftWithoutValidatingIt(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, runtime)
	for _, tc := range []struct{ draft, hint string }{
		{"Ship @@tomorrow", "Ctrl+F expand"},
		{"Ship ~missing", "Ctrl+F expand"},
		{"Ship <missing", "Ctrl+F expand"},
		{"One\nTwo", "Ctrl+F single task only"},
		{"Ship ;discuss @@tomorrow and ~dependency;", "Ctrl+F expand"},
		{"Ship *pin =review", "Ctrl+F expand"},
		{"", "Ctrl+F expand"},
	} {
		composer.Input.SetValue(tc.draft)
		composer.Errors = []grammar.LineError{{Line: 2, Message: "existing refusal"}}
		line, column := composer.Input.Line(), composer.Input.Column()
		spec, _ := composer.ContextualGuidance()
		found := false
		for _, hint := range spec.Hints {
			if hint.Key == "Ctrl+F" {
				found = chrome.GuidanceHintText(hint) == tc.hint
			}
		}
		if !found {
			t.Fatalf("draft %q: missing %q in %+v", tc.draft, tc.hint, spec)
		}
		if composer.Input.Value() != tc.draft || composer.Input.Line() != line || composer.Input.Column() != column ||
			len(composer.Errors) != 1 || composer.Errors[0].Message != "existing refusal" {
			t.Fatalf("guidance mutated the draft or validation: %+v", composer.Errors)
		}
	}
}
