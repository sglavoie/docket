package models

import (
	"testing"
)

func TestGetTasksBySectionIncludesCustomSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Task with CustomSection set (as happens after parser migration)
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.CustomSection = "Sprint1"

	// Task with Section set to a GTD section
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionNext

	// Task with both Section and CustomSection
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = SectionInbox
	task3.CustomSection = "Sprint1"

	project.Tasks = []*Task{task1, task2, task3}

	// Should find tasks by CustomSection
	sprint1Tasks := project.GetTasksBySection("Sprint1")
	if len(sprint1Tasks) != 2 {
		t.Errorf("expected 2 tasks in Sprint1 (via CustomSection), got %d", len(sprint1Tasks))
	}

	// GTD sections should still work normally
	nextTasks := project.GetTasksBySection(SectionNext)
	if len(nextTasks) != 1 {
		t.Errorf("expected 1 task in Next, got %d", len(nextTasks))
	}
}

func TestCountTasksBySectionIncludesCustomSections(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Task with CustomSection only (Section="" is the zero value for custom section tasks)
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.SetSection("Sprint1") // Uses SetSection to get consistent state

	// Another task in same custom section
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.SetSection("Sprint1")

	// Task in GTD section only
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = SectionNext

	// Task in Inbox
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Section = SectionInbox

	project.Tasks = []*Task{task1, task2, task3, task4}

	counts := project.CountTasksBySection()

	// Sprint1 should count both tasks with CustomSection = "Sprint1"
	if counts["Sprint1"] != 2 {
		t.Errorf("expected 2 tasks in Sprint1, got %d", counts["Sprint1"])
	}

	// GTD sections counted normally
	if counts[SectionInbox] != 1 {
		t.Errorf("expected 1 task in Inbox, got %d", counts[SectionInbox])
	}
	if counts[SectionNext] != 1 {
		t.Errorf("expected 1 task in Next, got %d", counts[SectionNext])
	}

	// Custom section tasks should NOT inflate the Active view count
	if counts[""] != 0 {
		t.Errorf("expected 0 tasks in Active view (empty section), got %d", counts[""])
	}
}

func TestGetTasksBySectionExcludesCustomSectionFromActiveView(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Task in Active view (Section="" and CustomSection="")
	task1, err := NewTask("1", 1, TaskTypeTask, "Active Task")
	if err != nil {
		t.Fatal(err)
	}

	// Task in custom section (Section="" but CustomSection="Sprint1")
	task2, err := NewTask("2", 2, TaskTypeTask, "Sprint Task")
	if err != nil {
		t.Fatal(err)
	}
	task2.SetSection("Sprint1")

	project.Tasks = []*Task{task1, task2}

	// Active view (empty section) should only contain task1
	activeView := project.GetTasksBySection("")
	if len(activeView) != 1 {
		t.Errorf("expected 1 task in Active view, got %d", len(activeView))
	}
	if len(activeView) > 0 && activeView[0].ID != "1" {
		t.Errorf("expected task 1 in Active view, got task %s", activeView[0].ID)
	}

	// Sprint1 should contain task2
	sprint1 := project.GetTasksBySection("Sprint1")
	if len(sprint1) != 1 {
		t.Errorf("expected 1 task in Sprint1, got %d", len(sprint1))
	}
}

func TestCountAndGetTasksBySectionConsistency(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, _ := NewTask("1", 1, TaskTypeTask, "Active")
	task2, _ := NewTask("2", 2, TaskTypeTask, "Custom")
	task2.SetSection("MySection")
	task3, _ := NewTask("3", 3, TaskTypeTask, "Inbox")
	task3.Section = SectionInbox

	project.Tasks = []*Task{task1, task2, task3}

	counts := project.CountTasksBySection()
	for section, count := range counts {
		tasks := project.GetTasksBySection(section)
		if len(tasks) != count {
			t.Errorf("section %q: CountTasksBySection=%d but GetTasksBySection=%d",
				section, count, len(tasks))
		}
	}
}
