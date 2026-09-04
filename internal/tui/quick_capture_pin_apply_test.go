package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/taskform"
)

func TestTaskComposerExpandRefusesToDropPin(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:    taskform.ComposerProject,
		Project: project,
	}, appModel.domain.runtime)
	composer.Input.SetValue("Ship release *pin")

	if cmd := composer.ExpandSingle(); cmd != nil {
		t.Fatal("expand accepted a line carrying a pin")
	}
	if composer.Mode != taskform.ComposerCompact || composer.Expanded != nil {
		t.Fatal("composer entered expanded mode despite the refusal")
	}
	if len(composer.Errors) != 1 ||
		!strings.Contains(composer.Errors[0].Message, "expanded mode cannot pin; submit the line with Enter instead") {
		t.Fatalf("errors = %+v", composer.Errors)
	}
}
