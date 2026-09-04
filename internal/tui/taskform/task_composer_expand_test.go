package taskform

import (
	"slices"
	"strings"
	"testing"
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
