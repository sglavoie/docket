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
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection empty, got %q", task.CustomSection)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if result.Warnings[0].Type != format.WarningDataCorrection {
		t.Errorf("expected WarningDataCorrection, got %v", result.Warnings[0].Type)
	}
}

func TestReassignUnknownSections_CustomSection(t *testing.T) {
	project := &models.Project{
		Sections: []string{models.SectionNext, models.SectionInbox, "Backend"},
	}

	// Task has a custom section that does NOT exist in the project's sections
	task := &models.Task{
		ID:            "t1",
		ShortID:       1,
		Section:       "",
		CustomSection: "RemovedSection",
	}
	project.Tasks = []*models.Task{task}

	result := &ParseResult{}
	reassignUnknownSections(project, result)

	if task.Section != models.SectionInbox {
		t.Errorf("expected Section=%q, got %q", models.SectionInbox, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection empty after reassignment, got %q", task.CustomSection)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if result.Warnings[0].Field != "custom_section" {
		t.Errorf("expected warning field 'custom_section', got %q", result.Warnings[0].Field)
	}
}

func TestReassignUnknownSections_ValidCustomSection(t *testing.T) {
	project := &models.Project{
		Sections: []string{models.SectionNext, models.SectionInbox, "Backend"},
	}

	// Task has a valid custom section that exists in the project
	task := &models.Task{
		ID:            "t1",
		ShortID:       1,
		Section:       "",
		CustomSection: "Backend",
	}
	project.Tasks = []*models.Task{task}

	result := &ParseResult{}
	reassignUnknownSections(project, result)

	// Should remain unchanged
	if task.Section != "" {
		t.Errorf("expected Section empty, got %q", task.Section)
	}
	if task.CustomSection != "Backend" {
		t.Errorf("expected CustomSection=%q, got %q", "Backend", task.CustomSection)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(result.Warnings))
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

	// Active view task (Section="" and CustomSection="")
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
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection empty, got %q", task.CustomSection)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(result.Warnings))
	}
}

func TestReassignUnknownSections_FullIntegration(t *testing.T) {
	// Test through the full parser path: a task with custom_section that
	// doesn't appear in the sections list should be reassigned on load.
	content := `# Project: Test
<!-- sections: Next;Backend -->

## Task: Valid section
<!-- id: t1, status: todo, priority: 3, custom_section: Backend -->

## Task: Unknown custom section
<!-- id: t2, status: todo, priority: 3, custom_section: RemovedSection -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	project := &models.Project{
		Tasks:    result.Tasks,
		Sections: result.Sections,
	}
	project.EnsureDefaultSections()

	reassignUnknownSections(project, result)

	// t1 should keep its valid custom section
	if result.Tasks[0].CustomSection != "Backend" {
		t.Errorf("task t1: expected CustomSection=%q, got %q", "Backend", result.Tasks[0].CustomSection)
	}

	// t2 should be reassigned to Inbox
	if result.Tasks[1].Section != models.SectionInbox {
		t.Errorf("task t2: expected Section=%q, got %q", models.SectionInbox, result.Tasks[1].Section)
	}
	if result.Tasks[1].CustomSection != "" {
		t.Errorf("task t2: expected CustomSection empty, got %q", result.Tasks[1].CustomSection)
	}
}
