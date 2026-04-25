package models

import "testing"

func TestRelocateTaskToEndOfSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Task at index 0 (before Next tasks) gets moved to Next section
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = SectionNext // just moved to Next
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionNext
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = SectionInbox

	project.Tasks = []*Task{task1, task2, task3}

	// task1 is before task2 in the slice but both are in Next
	// Relocate task1 to end of Next section
	project.RelocateTaskToEndOfSection("1", SectionNext)

	// task1 should now be after task2
	if project.Tasks[0].ID != "2" {
		t.Errorf("expected task2 at position 0, got %s", project.Tasks[0].ID)
	}
	if project.Tasks[1].ID != "1" {
		t.Errorf("expected task1 at position 1, got %s", project.Tasks[1].ID)
	}
	if project.Tasks[2].ID != "3" {
		t.Errorf("expected task3 at position 2, got %s", project.Tasks[2].ID)
	}
}

func TestRelocateTaskToEndOfSectionAlreadyAtEnd(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = SectionNext
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionNext

	project.Tasks = []*Task{task1, task2}

	// task2 is already the last Next task — no-op
	project.RelocateTaskToEndOfSection("2", SectionNext)

	if project.Tasks[0].ID != "1" || project.Tasks[1].ID != "2" {
		t.Error("order should not change when task is already at end of section")
	}
}

func TestRelocateTaskToEndOfSectionNoOtherTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = SectionNext
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionInbox

	project.Tasks = []*Task{task1, task2}

	// task1 is the only Next task — no-op
	project.RelocateTaskToEndOfSection("1", SectionNext)

	if project.Tasks[0].ID != "1" || project.Tasks[1].ID != "2" {
		t.Error("order should not change when task is only one in section")
	}
}

func TestRelocateTaskToEndOfSectionInterleaved(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Interleaved sections: moved task is before several Next tasks
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = SectionNext // just moved here
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionInbox
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = SectionNext
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Section = SectionInbox
	task5, err := NewTask("5", 5, TaskTypeTask, "Task 5")
	if err != nil {
		t.Fatal(err)
	}
	task5.Section = SectionNext

	project.Tasks = []*Task{task1, task2, task3, task4, task5}

	project.RelocateTaskToEndOfSection("1", SectionNext)

	// task1 should end up after task5 (last Next task at index 4)
	ids := make([]string, len(project.Tasks))
	for i, task := range project.Tasks {
		ids[i] = task.ID
	}
	// Expected: [2, 3, 4, 5, 1]
	expected := []string{"2", "3", "4", "5", "1"}
	for i, id := range expected {
		if ids[i] != id {
			t.Errorf("position %d: expected %s, got %s (full: %v)", i, id, ids[i], ids)
			break
		}
	}
}

func TestRelocateTaskToEndOfCustomSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// task1 is being moved to custom section "Backlog"
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.SetSection("Backlog") // Section="", CustomSection="Backlog"

	// task2 is in Backlog already
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.SetSection("Backlog")

	// task3 is in a different section
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = SectionNext

	// task4 is also in Backlog
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.SetSection("Backlog")

	project.Tasks = []*Task{task1, task2, task3, task4}

	// task1 is first; after relocate it should be after task4 (last Backlog task)
	project.RelocateTaskToEndOfSection("1", "Backlog")

	ids := make([]string, len(project.Tasks))
	for i, task := range project.Tasks {
		ids[i] = task.ID
	}
	// Expected: [2, 3, 4, 1]
	expected := []string{"2", "3", "4", "1"}
	for i, id := range expected {
		if ids[i] != id {
			t.Errorf("position %d: expected %s, got %s (full: %v)", i, id, ids[i], ids)
			break
		}
	}
}

func TestRelocateTaskToEndOfSectionCustomSectionNoOtherTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Only task in the custom section
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.SetSection("Backlog")

	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionNext

	project.Tasks = []*Task{task1, task2}

	// Only one Backlog task — should be a no-op
	project.RelocateTaskToEndOfSection("1", "Backlog")

	if project.Tasks[0].ID != "1" || project.Tasks[1].ID != "2" {
		t.Error("relocate should be no-op when task is only one in custom section")
	}
}
