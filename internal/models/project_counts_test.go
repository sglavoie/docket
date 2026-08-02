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

	task1, err := NewGenericTask("1", 1, "Task 1") // todo
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Start() // in_progress
	task3, err := NewGenericTask("3", 3, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Complete() // done
	task4, err := NewGenericTask("4", 4, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Block() // blocked
	task5, err := NewGenericTask("5", 5, "Task 5")
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

func TestProjectGetTasksByState(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Start()
	task3, err := NewGenericTask("3", 3, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2, task3}

	readyTasks := project.GetTasksByState(StateBacklog)
	if len(readyTasks) != 2 {
		t.Errorf("expected 2 ready tasks, got %d", len(readyTasks))
	}

	inProgressTasks := project.GetTasksByState(StateDoing)
	if len(inProgressTasks) != 1 {
		t.Errorf("expected 1 in_progress task, got %d", len(inProgressTasks))
	}
}

func TestProjectGetTasksByTag(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.AddTag("backend")
	task1.AddTag("urgent")

	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.AddTag("backend")

	task3, err := NewGenericTask("3", 3, "Task 3")
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

func TestCountOverdueTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	tomorrow := time.Now().AddDate(0, 0, 1)

	// Overdue task (past due date, not complete)
	task1, err := NewGenericTask("1", 1, "Overdue")
	if err != nil {
		t.Fatal(err)
	}
	task1.DueDate = &yesterday

	// Past due date but completed (should NOT count)
	task2, err := NewGenericTask("2", 2, "Done Overdue")
	if err != nil {
		t.Fatal(err)
	}
	task2.DueDate = &yesterday
	task2.Complete()

	// Future due date (should NOT count)
	task3, err := NewGenericTask("3", 3, "Future")
	if err != nil {
		t.Fatal(err)
	}
	task3.DueDate = &tomorrow

	// No due date (should NOT count)
	task4, err := NewGenericTask("4", 4, "No Due Date")
	if err != nil {
		t.Fatal(err)
	}

	// Overdue and blocked (should count - still incomplete)
	task5, err := NewGenericTask("5", 5, "Overdue Blocked")
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
	task1, err := NewGenericTask("1", 1, "Due Soon")
	if err != nil {
		t.Fatal(err)
	}
	task1.DueDate = &twoDays

	// Overdue (should NOT count - overdue is separate)
	task2, err := NewGenericTask("2", 2, "Overdue")
	if err != nil {
		t.Fatal(err)
	}
	task2.DueDate = &yesterday

	// Due in 10 days, outside 3-day window (should NOT count)
	task3, err := NewGenericTask("3", 3, "Due Later")
	if err != nil {
		t.Fatal(err)
	}
	task3.DueDate = &tenDays

	// Due in 2 days but completed (should NOT count)
	task4, err := NewGenericTask("4", 4, "Done Due Soon")
	if err != nil {
		t.Fatal(err)
	}
	task4.DueDate = &twoDays
	task4.Complete()

	// No due date (should NOT count)
	task5, err := NewGenericTask("5", 5, "No Due Date")
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

	task1, err := NewGenericTask("1", 1, "Blocked 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Block()

	task2, err := NewGenericTask("2", 2, "Blocked 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Block()

	task3, err := NewGenericTask("3", 3, "Todo")
	if err != nil {
		t.Fatal(err)
	}

	task4, err := NewGenericTask("4", 4, "InProgress")
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

func TestEmptyProjectCounts(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
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
