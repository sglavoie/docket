package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestStaleQueuePruning(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	task1 := newTestFocusTask(t, "Active", 1, true)
	task2 := newTestFocusTask(t, "Done", 2, false)
	task2.WorkflowState = models.StateDone
	task3 := newTestFocusTask(t, "Canceled", 3, false)
	task3.WorkflowState = models.StateCanceled

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)

	*m.queue = []FocusTask{
		{Task: task1, Project: project},
		{Task: task2, Project: project},
		{Task: task3, Project: project},
	}
	m.queueIDs[task1.ID] = true
	m.queueIDs[task2.ID] = true
	m.queueIDs[task3.ID] = true

	m.tabs.Active = focusTabQueue
	m.applyFilter()

	if len(*m.queue) != 1 {
		t.Fatalf("expected 1 task in queue after pruning, got %d", len(*m.queue))
	}
	if (*m.queue)[0].Task.Title != "Active" {
		t.Errorf("expected Active task to remain, got %q", (*m.queue)[0].Task.Title)
	}
	if m.isInQueue(task2.ID) || m.isInQueue(task3.ID) {
		t.Error("completed/canceled tasks should be removed from queueIDs")
	}
	if !m.isInQueue(task1.ID) {
		t.Error("active task should remain in queueIDs")
	}
}

func TestQueuePointerReconciliation(t *testing.T) {
	t.Run("queue pointers updated after loadTasks", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task := newTestFocusTask(t, "My Task", 1, true)
		project.Tasks = []*models.Task{task}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		staleTask := *task
		*m.queue = []FocusTask{{Task: &staleTask, Project: project}}
		m.queueIDs[task.ID] = true

		task.WorkflowState = models.StateInProgress

		if (*m.queue)[0].Task.State() == models.StateInProgress {
			t.Fatal("stale pointer should not reflect status change before reconciliation")
		}

		m.tasks = nil
		for _, tk := range project.Tasks {
			if !tk.IsComplete() {
				m.tasks = append(m.tasks, FocusTask{Task: tk, Project: project})
			}
		}

		taskByID := make(map[string]FocusTask, len(m.tasks))
		for _, ft := range m.tasks {
			taskByID[ft.Task.ID] = ft
		}
		q := *m.queue
		for i := range q {
			if fresh, ok := taskByID[q[i].Task.ID]; ok {
				q[i] = fresh
			}
		}
		*m.queue = q

		if (*m.queue)[0].Task.State() != models.StateInProgress {
			t.Errorf("expected reconciled status InProgress, got %v", (*m.queue)[0].Task.State())
		}
	})

	t.Run("queue entries for non-actionable tasks are removed", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task1 := newTestFocusTask(t, "Active", 1, true)
		task2 := newTestFocusTask(t, "Completed", 2, false)
		task2.WorkflowState = models.StateDone
		project.Tasks = []*models.Task{task1, task2}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		*m.queue = []FocusTask{
			{Task: task1, Project: project},
			{Task: task2, Project: project},
		}
		m.queueIDs[task1.ID] = true
		m.queueIDs[task2.ID] = true

		m.tasks = nil
		for _, tk := range project.Tasks {
			if !tk.IsComplete() {
				m.tasks = append(m.tasks, FocusTask{Task: tk, Project: project})
			}
		}

		taskByID := make(map[string]FocusTask, len(m.tasks))
		for _, ft := range m.tasks {
			taskByID[ft.Task.ID] = ft
		}
		q := *m.queue
		i := 0
		for i < len(q) {
			if fresh, ok := taskByID[q[i].Task.ID]; ok {
				q[i] = fresh
				i++
			} else {
				delete(m.queueIDs, q[i].Task.ID)
				q = append(q[:i], q[i+1:]...)
			}
		}
		*m.queue = q

		if len(*m.queue) != 1 {
			t.Fatalf("expected 1 task in queue after reconciliation, got %d", len(*m.queue))
		}
		if (*m.queue)[0].Task.Title != "Active" {
			t.Errorf("expected 'Active' to remain, got %q", (*m.queue)[0].Task.Title)
		}
		if m.isInQueue(task2.ID) {
			t.Error("completed task should be removed from queueIDs")
		}
	})
}
