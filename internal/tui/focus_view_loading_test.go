package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestFocusViewLoadTasks_FilteringLogic(t *testing.T) {
	t.Run("includes all non-complete tasks", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		active := newTestFocusTask(t, "Active task", 3, true)
		inactive := newTestFocusTask(t, "Inactive task", 3, false)
		project.Tasks = []*models.Task{active, inactive}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		for _, task := range project.Tasks {
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}

		if len(m.tasks) != 2 {
			t.Errorf("expected 2 tasks (active and inactive), got %d", len(m.tasks))
		}
	})

	t.Run("skips completed tasks", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		done := newTestFocusTask(t, "Done task", 3, true)
		done.Status = models.StatusDone

		canceled := newTestFocusTask(t, "Canceled task", 3, true)
		canceled.Status = models.StatusCanceled

		active := newTestFocusTask(t, "Active task", 3, true)
		project.Tasks = []*models.Task{done, canceled, active}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		for _, task := range project.Tasks {
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}

		if len(m.tasks) != 1 {
			t.Errorf("expected 1 task (non-completed), got %d", len(m.tasks))
		}
	})

	t.Run("includes blocked-status tasks (dependency check is separate)", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		blocked := newTestFocusTask(t, "Blocked task", 3, true)
		blocked.Status = models.StatusBlocked
		project.Tasks = []*models.Task{blocked}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		for _, task := range project.Tasks {
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}

		if len(m.tasks) != 1 {
			t.Errorf("expected 1 blocked task, got %d", len(m.tasks))
		}
	})
}

func TestFocusViewLoadTasks_ExcludesSnoozedTasks(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	normal := newTestFocusTask(t, "Normal task", 3, false)
	snoozed := newTestFocusTask(t, "Snoozed task", 3, false)
	future := time.Now().AddDate(0, 0, 7)
	snoozed.SnoozedUntil = &future
	project.Tasks = []*models.Task{normal, snoozed}

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)

	for _, task := range project.Tasks {
		if task.Status.IsComplete() {
			continue
		}
		if task.IsSnoozed() {
			continue
		}
		m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
	}

	if len(m.tasks) != 1 {
		t.Fatalf("expected 1 task (snoozed excluded), got %d", len(m.tasks))
	}
	if m.tasks[0].Task.Title != "Normal task" {
		t.Errorf("expected 'Normal task', got %q", m.tasks[0].Task.Title)
	}
}

func TestFocusViewLoadTasks_ExcludesDependencyBlockedTasks(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	blocker := newTestFocusTask(t, "Blocker", 1, false)
	dependent := newTestFocusTask(t, "Dependent", 2, false)
	dependent.DependsOn = []string{blocker.ID}
	project.Tasks = []*models.Task{blocker, dependent}

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)

	for _, task := range project.Tasks {
		if task.Status.IsComplete() {
			continue
		}
		if task.IsSnoozed() {
			continue
		}
		if project.HasIncompleteDependencies(task) {
			continue
		}
		m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
	}

	if len(m.tasks) != 1 {
		t.Fatalf("expected 1 task (dependency-blocked excluded), got %d", len(m.tasks))
	}
	if m.tasks[0].Task.Title != "Blocker" {
		t.Errorf("expected 'Blocker', got %q", m.tasks[0].Task.Title)
	}
}

func TestFocusViewMultipleProjects(t *testing.T) {
	t.Run("aggregates tasks from multiple projects", func(t *testing.T) {
		project1 := newTestFocusProject(t, "Project 1")
		task1 := newTestFocusTask(t, "Task from P1", 1, true)
		project1.Tasks = []*models.Task{task1}

		project2 := newTestFocusProject(t, "Project 2")
		task2 := newTestFocusTask(t, "Task from P2", 2, true)
		project2.Tasks = []*models.Task{task2}

		cfg := newTestFocusConfig([]*models.Project{project1, project2})
		m := newTestFocusViewModel(cfg)

		for _, project := range []*models.Project{project1, project2} {
			for _, task := range project.Tasks {
				if !task.Status.IsComplete() {
					m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
				}
			}
		}
		m.sortTasks()

		if len(m.tasks) != 2 {
			t.Fatalf("expected 2 tasks from 2 projects, got %d", len(m.tasks))
		}

		if m.tasks[0].Task.Title != "Task from P1" {
			t.Errorf("expected 'Task from P1' first (priority 1), got %q", m.tasks[0].Task.Title)
		}
	})
}
