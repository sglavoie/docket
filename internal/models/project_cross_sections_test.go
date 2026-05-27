package models

import "testing"

func TestMoveTaskToProjectPreservesSectionsIfExist(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	sourceProj.Sections = []string{SectionNext, "InProgress", "CustomSection"}
	destProj.Sections = []string{SectionNext, "InProgress"}

	task1, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = "InProgress"

	task2, err := NewTask("task2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = "CustomSection"

	sourceProj.Tasks = []*Task{task1, task2}
	sourceProj.BuildTaskTree()

	result1, err := sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}
	if task1.Section != "InProgress" {
		t.Errorf("expected task1 to keep 'InProgress' section, got %s", task1.Section)
	}

	result2, err := sourceProj.MoveTaskToProject(task2, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}
	if task2.Section != SectionNext {
		t.Errorf("expected task2 section to reset to Next, got %s", task2.Section)
	}

	if len(result1.MovedTasks) != 1 || len(result2.MovedTasks) != 1 {
		t.Error("expected each move to report 1 moved task")
	}
}

func TestMoveTaskToProjectEnsuresCustomSectionInDest(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	sourceProj.Sections = []string{SectionNext, "Sprint-1"}
	destProj.Sections = []string{SectionNext}

	task, err := NewTask("task1", 1, TaskTypeTask, "Task with custom section")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Sprint-1"

	sourceProj.Tasks = []*Task{task}
	sourceProj.BuildTaskTree()

	_, err = sourceProj.MoveTaskToProject(task, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	if task.CustomSection != "Sprint-1" {
		t.Errorf("expected custom section preserved, got %q", task.CustomSection)
	}
	if !destProj.HasSection("Sprint-1") {
		t.Error("expected destination project to have 'Sprint-1' section after move")
	}
}

// TestMoveTaskToProjectAddsGTDSectionToDestList verifies that moving a task
// whose Section is a built-in GTD bucket (e.g. Inbox) also adds that section
// to the destination project's Sections list. Without the fix, the task kept
// its Section field but dest.Sections never enumerated the bucket, hiding the
// task from any rendering pass driven by dest.GetSections() (kanban, section
// pickers, GTD list views).
func TestMoveTaskToProjectAddsGTDSectionToDestList(t *testing.T) {
	sourceProj, err := NewProject("source", "Source", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}
	// Dest has only the default Next section (no Inbox/Waiting/etc.) -
	// EnsureDefaultSections was deliberately not called.
	destProj.Sections = []string{SectionNext}

	inboxTask, err := NewTask("inbox1", 1, TaskTypeTask, "Inbox Task")
	if err != nil {
		t.Fatal(err)
	}
	inboxTask.Section = SectionInbox

	waitingTask, err := NewTask("waiting1", 2, TaskTypeTask, "Waiting Task")
	if err != nil {
		t.Fatal(err)
	}
	waitingTask.Section = SectionWaiting

	sourceProj.Tasks = []*Task{inboxTask, waitingTask}
	sourceProj.BuildTaskTree()

	if _, err := sourceProj.MoveTaskToProject(inboxTask, destProj); err != nil {
		t.Fatalf("MoveTaskToProject (inbox): %v", err)
	}
	if !destProj.HasSection(SectionInbox) {
		t.Errorf("expected dest.Sections to include %q after move, got %v", SectionInbox, destProj.Sections)
	}

	if _, err := sourceProj.MoveTaskToProject(waitingTask, destProj); err != nil {
		t.Fatalf("MoveTaskToProject (waiting): %v", err)
	}
	if !destProj.HasSection(SectionWaiting) {
		t.Errorf("expected dest.Sections to include %q after move, got %v", SectionWaiting, destProj.Sections)
	}
}

func TestMoveTaskToProjectPreservesGTDSections(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	destProj.Sections = []string{SectionNext}

	inboxTask, err := NewTask("inbox1", 1, TaskTypeTask, "Inbox Task")
	if err != nil {
		t.Fatal(err)
	}
	inboxTask.Section = SectionInbox

	waitingTask, err := NewTask("waiting1", 2, TaskTypeTask, "Waiting Task")
	if err != nil {
		t.Fatal(err)
	}
	waitingTask.Section = SectionWaiting

	somedayTask, err := NewTask("someday1", 3, TaskTypeTask, "Someday Task")
	if err != nil {
		t.Fatal(err)
	}
	somedayTask.Section = SectionSomeday

	sourceProj.Tasks = []*Task{inboxTask, waitingTask, somedayTask}
	sourceProj.BuildTaskTree()

	_, err = sourceProj.MoveTaskToProject(inboxTask, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject (inbox) failed: %v", err)
	}
	if inboxTask.Section != SectionInbox {
		t.Errorf("expected inbox task to keep Inbox section, got %q", inboxTask.Section)
	}

	_, err = sourceProj.MoveTaskToProject(waitingTask, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject (waiting) failed: %v", err)
	}
	if waitingTask.Section != SectionWaiting {
		t.Errorf("expected waiting task to keep Waiting section, got %q", waitingTask.Section)
	}

	_, err = sourceProj.MoveTaskToProject(somedayTask, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject (someday) failed: %v", err)
	}
	if somedayTask.Section != SectionSomeday {
		t.Errorf("expected someday task to keep Someday section, got %q", somedayTask.Section)
	}
}
