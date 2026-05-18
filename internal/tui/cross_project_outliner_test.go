package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectSectionFilterActiveExcludesCustomSections(t *testing.T) {
	filter := crossProjectSectionFilter(Perspective{Name: "Active"})
	if filter == nil {
		t.Fatal("expected Active perspective filter")
	}

	activeTask := &models.Task{ID: "active"}
	customTask := &models.Task{ID: "custom"}
	customTask.SetSection("Sprint-1")
	nextTask := &models.Task{ID: "next", Section: models.SectionNext}
	completedTask := &models.Task{ID: "done", Status: models.StatusDone}

	if !filter(activeTask) {
		t.Fatal("expected empty-section task to match Active perspective")
	}
	if filter(customTask) {
		t.Fatal("expected custom-section task to be excluded from Active perspective")
	}
	if filter(nextTask) {
		t.Fatal("expected Next-section task to be excluded from Active perspective")
	}
	if filter(completedTask) {
		t.Fatal("expected completed task to be excluded from Active perspective")
	}
}
