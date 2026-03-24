package models

import (
	"errors"
	"strings"
	"testing"
)

func TestProjectGetSections(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Default should be just Next
	sections := project.GetSections()
	if len(sections) != 1 || sections[0] != SectionNext {
		t.Errorf("expected [Next], got %v", sections)
	}

	// Add more sections
	project.Sections = []string{"Active", "Done", SectionNext}
	sections = project.GetSections()

	// Next should always be first
	if sections[0] != SectionNext {
		t.Errorf("expected Next first, got %s", sections[0])
	}
	if len(sections) != 3 {
		t.Errorf("expected 3 sections, got %d", len(sections))
	}

	// Test with empty sections - should return Next
	project.Sections = []string{}
	sections = project.GetSections()
	if len(sections) != 1 || sections[0] != SectionNext {
		t.Errorf("expected [Next] for empty sections, got %v", sections)
	}

	// Test with sections missing Next - should prepend it
	project.Sections = []string{"Active", "Done"}
	sections = project.GetSections()
	if sections[0] != SectionNext {
		t.Errorf("expected Next first even when missing, got %s", sections[0])
	}
	if len(sections) != 3 {
		t.Errorf("expected 3 sections after prepending Next, got %d", len(sections))
	}
}

func TestProjectHasSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Sections = []string{SectionNext, "Active", "Done"}

	if !project.HasSection(SectionNext) {
		t.Error("expected HasSection to return true for Next")
	}
	if !project.HasSection("Active") {
		t.Error("expected HasSection to return true for Active")
	}
	if project.HasSection("Nonexistent") {
		t.Error("expected HasSection to return false for Nonexistent")
	}
}

func TestProjectEnsureSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	initialLen := len(project.Sections)

	// Adding existing section should not add duplicate
	project.EnsureSection(SectionNext)
	if len(project.Sections) != initialLen {
		t.Error("EnsureSection should not add duplicate Next")
	}

	// Adding new section should add it
	project.EnsureSection("Active")
	if !project.HasSection("Active") {
		t.Error("EnsureSection should add new section")
	}

	// Adding empty string should do nothing
	prevLen := len(project.Sections)
	project.EnsureSection("")
	if len(project.Sections) != prevLen {
		t.Error("EnsureSection should not add empty string")
	}

	// Adding whitespace-only string should do nothing
	project.EnsureSection("   ")
	if len(project.Sections) != prevLen {
		t.Error("EnsureSection should not add whitespace-only string")
	}

	// Adding a very long name should do nothing
	longName := strings.Repeat("a", MaxSectionNameLength+1)
	project.EnsureSection(longName)
	if len(project.Sections) != prevLen {
		t.Error("EnsureSection should not add excessively long name")
	}
}

func TestProjectRemoveSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Sections = []string{SectionNext, "Active", "Done"}

	// Create tasks in Active section
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = "Active"
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = "Active"
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = "Done"
	project.Tasks = []*Task{task1, task2, task3}

	// Cannot remove Next
	if project.RemoveSection(SectionNext) {
		t.Error("should not be able to remove Next")
	}

	// Remove Active - should clear section from tasks
	if !project.RemoveSection("Active") {
		t.Error("should be able to remove Active")
	}
	if project.HasSection("Active") {
		t.Error("Active should be removed")
	}
	if task1.Section != "" {
		t.Errorf("task1 section should be cleared, got %s", task1.Section)
	}
	if task2.Section != "" {
		t.Errorf("task2 section should be cleared, got %s", task2.Section)
	}
	if task3.Section != "Done" {
		t.Errorf("task3 section should still be Done, got %s", task3.Section)
	}

	// Removing non-existent section returns false
	if project.RemoveSection("Nonexistent") {
		t.Error("should return false for non-existent section")
	}
}

func TestProjectRenameSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Sections = []string{SectionNext, "Active", "Done"}

	// Create tasks in Active section
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = "Active"
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = "Done"
	project.Tasks = []*Task{task1, task2}

	// Cannot rename Next
	if err := project.RenameSection(SectionNext, "NewName"); err == nil {
		t.Error("should not be able to rename Next")
	}

	// Cannot rename to empty string
	if err := project.RenameSection("Active", ""); err == nil {
		t.Error("should not be able to rename to empty string")
	}

	// Cannot rename to existing section name — should wrap ErrSectionAlreadyExists
	if err := project.RenameSection("Active", "Done"); err == nil {
		t.Error("should not be able to rename to existing section name")
	} else if !errors.Is(err, ErrSectionAlreadyExists) {
		t.Errorf("expected ErrSectionAlreadyExists, got: %v", err)
	}

	// Rename Active to InProgress
	if err := project.RenameSection("Active", "InProgress"); err != nil {
		t.Errorf("should be able to rename Active to InProgress: %v", err)
	}
	if project.HasSection("Active") {
		t.Error("Active should no longer exist")
	}
	if !project.HasSection("InProgress") {
		t.Error("InProgress should exist")
	}
	if task1.Section != "InProgress" {
		t.Errorf("task1 section should be InProgress, got %s", task1.Section)
	}
	if task2.Section != "Done" {
		t.Errorf("task2 section should still be Done, got %s", task2.Section)
	}

	// Renaming non-existent section returns error
	if err := project.RenameSection("Nonexistent", "NewName"); err == nil {
		t.Error("should return error for non-existent section")
	}
}

func TestProjectGetTasksBySection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = "Active"
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = "Active"
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = "Done"
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Section = ""

	project.Tasks = []*Task{task1, task2, task3, task4}

	active := project.GetTasksBySection("Active")
	if len(active) != 2 {
		t.Errorf("expected 2 active tasks, got %d", len(active))
	}

	done := project.GetTasksBySection("Done")
	if len(done) != 1 {
		t.Errorf("expected 1 done task, got %d", len(done))
	}

	// Empty section represents Active view, query with empty string
	activeView := project.GetTasksBySection("")
	if len(activeView) != 1 {
		t.Errorf("expected 1 task with empty section (Active view), got %d", len(activeView))
	}

	// Next section should be empty since task4 has empty section
	next := project.GetTasksBySection(SectionNext)
	if len(next) != 0 {
		t.Errorf("expected 0 tasks in Next, got %d", len(next))
	}

	nonexistent := project.GetTasksBySection("Nonexistent")
	if len(nonexistent) != 0 {
		t.Errorf("expected 0 tasks for nonexistent section, got %d", len(nonexistent))
	}
}

func TestRemoveSectionRejectsBuiltInSections(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.EnsureDefaultSections()

	builtInSections := []string{
		SectionNext, SectionInbox, SectionWaiting,
		SectionSomeday, SectionReview, SectionStandby, SectionCompleted,
	}

	for _, section := range builtInSections {
		if project.RemoveSection(section) {
			t.Errorf("RemoveSection(%q) should return false for built-in section", section)
		}
		if !project.HasSection(section) {
			t.Errorf("built-in section %q should still exist after failed removal", section)
		}
	}
}

func TestRenameSectionRejectsBuiltInSections(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.EnsureDefaultSections()

	builtInSections := []string{
		SectionNext, SectionInbox, SectionWaiting,
		SectionSomeday, SectionReview, SectionStandby, SectionCompleted,
	}

	for _, section := range builtInSections {
		if err := project.RenameSection(section, "CustomName"); err == nil {
			t.Errorf("RenameSection(%q, ...) should return error for built-in section", section)
		}
		if !project.HasSection(section) {
			t.Errorf("built-in section %q should still exist after failed rename", section)
		}
	}
}

func TestProjectCountTasksBySection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = "Active"
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = "Active"
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = "Done"
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Section = "" // Empty section

	project.Tasks = []*Task{task1, task2, task3, task4}

	counts := project.CountTasksBySection()

	if counts["Active"] != 2 {
		t.Errorf("expected 2 Active tasks, got %d", counts["Active"])
	}
	if counts["Done"] != 1 {
		t.Errorf("expected 1 Done task, got %d", counts["Done"])
	}
	// Empty section tasks represent Active view and are counted separately
	if counts[""] != 1 {
		t.Errorf("expected 1 empty section task (Active view), got %d", counts[""])
	}
}
