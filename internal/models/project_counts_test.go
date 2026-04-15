package models

import (
	"testing"
	"time"
)

func TestProjectTaskCounts(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1") // todo
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Start() // in_progress
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Complete() // done
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Block() // blocked
	task5, err := NewTask("5", 5, TaskTypeTask, "Task 5")
	if err != nil {
		t.Fatal(err)
	}
	task5.Cancel() // canceled

	project.Tasks = []*Task{task1, task2, task3, task4, task5}

	if project.CountTasks() != 5 {
		t.Errorf("expected 5 total tasks, got %d", project.CountTasks())
	}

	// Active: todo, in_progress, blocked = 3
	if project.CountActiveTasks() != 3 {
		t.Errorf("expected 3 active tasks, got %d", project.CountActiveTasks())
	}

	// Completed: done, canceled = 2
	if project.CountCompletedTasks() != 2 {
		t.Errorf("expected 2 completed tasks, got %d", project.CountCompletedTasks())
	}
}

func TestProjectGetTasksByStatus(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Start()
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2, task3}

	todoTasks := project.GetTasksByStatus(StatusTodo)
	if len(todoTasks) != 2 {
		t.Errorf("expected 2 todo tasks, got %d", len(todoTasks))
	}

	inProgressTasks := project.GetTasksByStatus(StatusInProgress)
	if len(inProgressTasks) != 1 {
		t.Errorf("expected 1 in_progress task, got %d", len(inProgressTasks))
	}
}

func TestProjectGetTasksByType(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	epic, err := NewTask("1", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := NewTask("2", 2, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("3", 3, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	bug, err := NewTask("4", 4, TaskTypeBug, "Bug")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{epic, task1, task2, bug}

	tasks := project.GetTasksByType(TaskTypeTask)
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks of type Task, got %d", len(tasks))
	}

	epics := project.GetTasksByType(TaskTypeEpic)
	if len(epics) != 1 {
		t.Errorf("expected 1 epic, got %d", len(epics))
	}
}

func TestProjectGetTasksByTag(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.AddTag("backend")
	task1.AddTag("urgent")

	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.AddTag("backend")

	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.AddTag("frontend")

	project.Tasks = []*Task{task1, task2, task3}

	backend := project.GetTasksByTag("backend")
	if len(backend) != 2 {
		t.Errorf("expected 2 backend tasks, got %d", len(backend))
	}

	urgent := project.GetTasksByTag("urgent")
	if len(urgent) != 1 {
		t.Errorf("expected 1 urgent task, got %d", len(urgent))
	}

	nonexistent := project.GetTasksByTag("nonexistent")
	if len(nonexistent) != 0 {
		t.Errorf("expected 0 tasks with nonexistent tag, got %d", len(nonexistent))
	}
}

func TestCountActiveSectionTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Task in Active section (empty string), todo status
	task1, err := NewTask("1", 1, TaskTypeTask, "Active Todo")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = ""

	// Task in Active section, in_progress status
	task2, err := NewTask("2", 2, TaskTypeTask, "Active InProgress")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = ""
	task2.Start()

	// Task in a named section but in_progress (should count)
	task3, err := NewTask("3", 3, TaskTypeTask, "Named Section InProgress")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = "Backlog"
	task3.Start()

	// Task in a named section, todo status (should NOT count)
	task4, err := NewTask("4", 4, TaskTypeTask, "Backlog Todo")
	if err != nil {
		t.Fatal(err)
	}
	task4.Section = "Backlog"

	// Completed task in Active section (should NOT count)
	task5, err := NewTask("5", 5, TaskTypeTask, "Active Done")
	if err != nil {
		t.Fatal(err)
	}
	task5.Section = ""
	task5.Complete()

	// Blocked task - Block() moves to Waiting section, so it should NOT count
	// (section is "Waiting", status is blocked, not in_progress)
	task6, err := NewTask("6", 6, TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}
	task6.Block()

	project.Tasks = []*Task{task1, task2, task3, task4, task5, task6}

	// Active section tasks: task1 (section="", todo), task2 (section="", in_progress),
	// task3 (section="Backlog" but in_progress)
	// NOT task4 (section="Backlog", todo), NOT task5 (section="", done),
	// NOT task6 (section="Waiting", blocked)
	got := project.CountActiveSectionTasks()
	if got != 3 {
		t.Errorf("expected 3 active section tasks, got %d", got)
	}
}

func TestCountOverdueTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	tomorrow := time.Now().AddDate(0, 0, 1)

	// Overdue task (past due date, not complete)
	task1, err := NewTask("1", 1, TaskTypeTask, "Overdue")
	if err != nil {
		t.Fatal(err)
	}
	task1.DueDate = &yesterday

	// Past due date but completed (should NOT count)
	task2, err := NewTask("2", 2, TaskTypeTask, "Done Overdue")
	if err != nil {
		t.Fatal(err)
	}
	task2.DueDate = &yesterday
	task2.Complete()

	// Future due date (should NOT count)
	task3, err := NewTask("3", 3, TaskTypeTask, "Future")
	if err != nil {
		t.Fatal(err)
	}
	task3.DueDate = &tomorrow

	// No due date (should NOT count)
	task4, err := NewTask("4", 4, TaskTypeTask, "No Due Date")
	if err != nil {
		t.Fatal(err)
	}

	// Overdue and blocked (should count - still incomplete)
	task5, err := NewTask("5", 5, TaskTypeTask, "Overdue Blocked")
	if err != nil {
		t.Fatal(err)
	}
	task5.DueDate = &yesterday
	task5.Block()

	project.Tasks = []*Task{task1, task2, task3, task4, task5}

	got := project.CountOverdueTasks()
	if got != 2 {
		t.Errorf("expected 2 overdue tasks, got %d", got)
	}
}

func TestCountDueSoonTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	twoDays := time.Now().AddDate(0, 0, 2)
	tenDays := time.Now().AddDate(0, 0, 10)

	// Due in 2 days, within 3-day window (should count)
	task1, err := NewTask("1", 1, TaskTypeTask, "Due Soon")
	if err != nil {
		t.Fatal(err)
	}
	task1.DueDate = &twoDays

	// Overdue (should NOT count - overdue is separate)
	task2, err := NewTask("2", 2, TaskTypeTask, "Overdue")
	if err != nil {
		t.Fatal(err)
	}
	task2.DueDate = &yesterday

	// Due in 10 days, outside 3-day window (should NOT count)
	task3, err := NewTask("3", 3, TaskTypeTask, "Due Later")
	if err != nil {
		t.Fatal(err)
	}
	task3.DueDate = &tenDays

	// Due in 2 days but completed (should NOT count)
	task4, err := NewTask("4", 4, TaskTypeTask, "Done Due Soon")
	if err != nil {
		t.Fatal(err)
	}
	task4.DueDate = &twoDays
	task4.Complete()

	// No due date (should NOT count)
	task5, err := NewTask("5", 5, TaskTypeTask, "No Due Date")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2, task3, task4, task5}

	got := project.CountDueSoonTasks(3)
	if got != 1 {
		t.Errorf("expected 1 due-soon task, got %d", got)
	}
}

func TestCountBlockedTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Blocked 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Block()

	task2, err := NewTask("2", 2, TaskTypeTask, "Blocked 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Block()

	task3, err := NewTask("3", 3, TaskTypeTask, "Todo")
	if err != nil {
		t.Fatal(err)
	}

	task4, err := NewTask("4", 4, TaskTypeTask, "InProgress")
	if err != nil {
		t.Fatal(err)
	}
	task4.Start()

	project.Tasks = []*Task{task1, task2, task3, task4}

	got := project.CountBlockedTasks()
	if got != 2 {
		t.Errorf("expected 2 blocked tasks, got %d", got)
	}
}

func TestCountActiveSectionTasksExcludesCustomSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Task in Active view (Section="", CustomSection="")
	active, err := NewTask("1", 1, TaskTypeTask, "Active Task")
	if err != nil {
		t.Fatal(err)
	}

	// Task with CustomSection set — should NOT be counted as Active
	// even though Section="" (which is the zero value when CustomSection is set).
	custom, err := NewTask("2", 2, TaskTypeTask, "Custom Section Task")
	if err != nil {
		t.Fatal(err)
	}
	custom.SetSection("Sprint1")

	// in_progress task in a custom section — should count (in_progress always counts)
	ipCustom, err := NewTask("3", 3, TaskTypeTask, "InProgress Custom")
	if err != nil {
		t.Fatal(err)
	}
	ipCustom.SetSection("Sprint1")
	ipCustom.Start()

	// Task in a GTD section, not in_progress — should NOT count
	gtd, err := NewTask("4", 4, TaskTypeTask, "Next Section Task")
	if err != nil {
		t.Fatal(err)
	}
	gtd.Section = SectionNext

	project.Tasks = []*Task{active, custom, ipCustom, gtd}

	// Expected: active (Section="", CustomSection="", todo) + ipCustom (in_progress) = 2
	// NOT custom (has CustomSection, not in_progress)
	// NOT gtd (has Section="Next", not in_progress)
	got := project.CountActiveSectionTasks()
	if got != 2 {
		t.Errorf("expected 2 active section tasks, got %d", got)
	}
}

func TestEmptyProjectCounts(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	if project.CountActiveSectionTasks() != 0 {
		t.Errorf("expected 0 active section tasks, got %d", project.CountActiveSectionTasks())
	}
	if project.CountOverdueTasks() != 0 {
		t.Errorf("expected 0 overdue tasks, got %d", project.CountOverdueTasks())
	}
	if project.CountDueSoonTasks(3) != 0 {
		t.Errorf("expected 0 due-soon tasks, got %d", project.CountDueSoonTasks(3))
	}
	if project.CountBlockedTasks() != 0 {
		t.Errorf("expected 0 blocked tasks, got %d", project.CountBlockedTasks())
	}
	if project.CountTasks() != 0 {
		t.Errorf("expected 0 total tasks, got %d", project.CountTasks())
	}
	if project.CountActiveTasks() != 0 {
		t.Errorf("expected 0 active tasks, got %d", project.CountActiveTasks())
	}
	if project.CountCompletedTasks() != 0 {
		t.Errorf("expected 0 completed tasks, got %d", project.CountCompletedTasks())
	}
}
