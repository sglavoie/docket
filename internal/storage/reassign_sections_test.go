package storage

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

func TestReassignUnknownSections_GTDSection(t *testing.T) {
	project := &models.Project{
		Sections: []string{models.SectionNext, models.SectionInbox},
	}

	task := &models.Task{
		ID:      "t1",
		ShortID: 1,
		Section: "NonExistent",
	}
	project.Tasks = []*models.Task{task}

	result := &ParseResult{}
	reassignUnknownSections(project, result)

	if task.Section != models.SectionInbox {
		t.Errorf("expected Section=%q, got %q", models.SectionInbox, task.Section)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if result.Warnings[0].Type != format.WarningDataCorrection {
		t.Errorf("expected WarningDataCorrection, got %v", result.Warnings[0].Type)
	}
}

func TestReassignUnknownSections_ValidGTDSection(t *testing.T) {
	project := &models.Project{
		Sections: []string{models.SectionNext, models.SectionInbox, models.SectionWaiting},
	}

	task := &models.Task{
		ID:      "t1",
		ShortID: 1,
		Section: models.SectionWaiting,
	}
	project.Tasks = []*models.Task{task}

	result := &ParseResult{}
	reassignUnknownSections(project, result)

	// Should remain unchanged
	if task.Section != models.SectionWaiting {
		t.Errorf("expected Section=%q, got %q", models.SectionWaiting, task.Section)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(result.Warnings))
	}
}

func TestReassignUnknownSections_EmptySectionNoCustom(t *testing.T) {
	project := &models.Project{
		Sections: []string{models.SectionNext},
	}

	// Active view task (Section="")
	task := &models.Task{
		ID:      "t1",
		ShortID: 1,
		Section: "",
	}
	project.Tasks = []*models.Task{task}

	result := &ParseResult{}
	reassignUnknownSections(project, result)

	// Should remain unchanged
	if task.Section != "" {
		t.Errorf("expected Section empty, got %q", task.Section)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(result.Warnings))
	}
}
