package models

import (
	"testing"
)

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
