package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectSectionFilterActiveIncludesEmptySectionTasks(t *testing.T) {
	filter := crossProjectSectionFilter(Perspective{Name: "Active"})
	if filter == nil {
		t.Fatal("expected Active perspective filter")
	}

	activeTask := &models.Task{ID: "active"}
	// "Sprint-1" is a retired custom section; the task resolves to the default
	// ready (Active) state.
	nonGTDTask := &models.Task{ID: "legacy"}
	nextTask := &models.Task{ID: "next", WorkflowState: models.StateNext}
	completedTask := &models.Task{ID: "done", WorkflowState: models.StateDone}

	if !filter(activeTask) {
		t.Fatal("expected empty-section task to match Active perspective")
	}
	if !filter(nonGTDTask) {
		t.Fatal("expected legacy non-GTD section task to match Active perspective")
	}
	if filter(nextTask) {
		t.Fatal("expected Next-section task to be excluded from Active perspective")
	}
	if filter(completedTask) {
		t.Fatal("expected completed task to be excluded from Active perspective")
	}
}
