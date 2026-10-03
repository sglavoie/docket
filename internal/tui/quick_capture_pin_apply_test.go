package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/taskform"
)

func TestTaskComposerExpandShowsCapturedPin(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:    taskform.ComposerProject,
		Project: project,
	}, appModel.domain.runtime)
	composer.Input.SetValue("Ship release *pin")

	composer.SetSize(100, 35)
	composer.ExpandSingle()
	if composer.Mode != taskform.ComposerExpanded || composer.Expanded == nil {
		t.Fatalf("pinned capture did not expand: %+v", composer.Errors)
	}
	if len(composer.Errors) != 0 || !strings.Contains(composer.View().Content, "Pinned") {
		t.Fatalf("expanded form did not show its captured pin:\n%s", composer.View().Content)
	}
}
