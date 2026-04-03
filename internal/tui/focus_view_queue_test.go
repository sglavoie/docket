package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

func TestFocusViewQueue(t *testing.T) {
	t.Run("accept and remove from queue", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task1 := newTestFocusTask(t, "Task 1", 1, true)
		task2 := newTestFocusTask(t, "Task 2", 2, true)

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		ft1 := FocusTask{Task: task1, Project: project}
		ft2 := FocusTask{Task: task2, Project: project}

		m.acceptIntoQueue(ft1)
		m.acceptIntoQueue(ft2)
		if len(*m.queue) != 2 {
			t.Fatalf("expected 2 queued tasks, got %d", len(*m.queue))
		}

		m.acceptIntoQueue(ft1)
		if len(*m.queue) != 2 {
			t.Fatalf("expected 2 queued tasks after duplicate, got %d", len(*m.queue))
		}

		if !m.isInQueue(task1.ID) {
			t.Error("expected task1 to be in queue")
		}

		m.removeFromQueue(0)
		if len(*m.queue) != 1 {
			t.Fatalf("expected 1 queued task after removal, got %d", len(*m.queue))
		}
		if m.isInQueue(task1.ID) {
			t.Error("expected task1 to be removed from queue")
		}
	})

	t.Run("reorder queue", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task1 := newTestFocusTask(t, "Task 1", 1, true)
		task2 := newTestFocusTask(t, "Task 2", 2, true)

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		m.acceptIntoQueue(FocusTask{Task: task1, Project: project})
		m.acceptIntoQueue(FocusTask{Task: task2, Project: project})

		m.reorderQueue(0, 1)
		q := *m.queue
		if q[0].Task.Title != "Task 2" || q[1].Task.Title != "Task 1" {
			t.Errorf("expected swapped order, got %s, %s", q[0].Task.Title, q[1].Task.Title)
		}
	})
}

func TestStaleQueuePruning(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	task1 := newTestFocusTask(t, "Active", 1, true)
	task2 := newTestFocusTask(t, "Done", 2, false)
	task2.Status = models.StatusDone
	task3 := newTestFocusTask(t, "Canceled", 3, false)
	task3.Status = models.StatusCanceled

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

		task.Status = models.StatusInProgress

		if (*m.queue)[0].Task.Status == models.StatusInProgress {
			t.Fatal("stale pointer should not reflect status change before reconciliation")
		}

		m.tasks = nil
		for _, tk := range project.Tasks {
			if !tk.Status.IsComplete() {
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

		if (*m.queue)[0].Task.Status != models.StatusInProgress {
			t.Errorf("expected reconciled status InProgress, got %v", (*m.queue)[0].Task.Status)
		}
	})

	t.Run("queue entries for non-actionable tasks are removed", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task1 := newTestFocusTask(t, "Active", 1, true)
		task2 := newTestFocusTask(t, "Completed", 2, false)
		task2.Status = models.StatusDone
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
			if !tk.Status.IsComplete() {
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

func TestAcceptOnAnyTab(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	task1 := newTestFocusTask(t, "Task 1", 1, true)
	task2 := newTestFocusTask(t, "Task 2", 2, true)
	task3 := newTestFocusTask(t, "Task 3", 3, true)

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)
	m.tasks = []FocusTask{
		{Task: task1, Project: project},
		{Task: task2, Project: project},
		{Task: task3, Project: project},
	}

	m.ranked = focus.RankCrossProjectTasks(m.tasks, m.dismissedIDs, cfg)

	t.Run("accept on Queue tab adds top non-queued suggestion", func(t *testing.T) {
		m.tabs.Active = focusTabQueue
		m.handleAccept()

		if len(*m.queue) != 1 {
			t.Fatalf("expected 1 task in queue, got %d", len(*m.queue))
		}
		firstID := (*m.queue)[0].Task.ID
		if firstID != m.ranked[0].Task.ID {
			t.Errorf("expected top ranked task in queue")
		}
	})

	t.Run("accept on Queue tab skips already-queued task", func(t *testing.T) {
		m.handleAccept()

		if len(*m.queue) != 2 {
			t.Fatalf("expected 2 tasks in queue, got %d", len(*m.queue))
		}
		if (*m.queue)[0].Task.ID == (*m.queue)[1].Task.ID {
			t.Error("second accept should add a different task")
		}
	})

	t.Run("accept on By Project tab also adds suggestion", func(t *testing.T) {
		*m.queue = nil
		m.queueIDs = make(map[string]bool)

		m.tabs.Active = focusTabByProject
		m.handleAccept()

		if len(*m.queue) != 1 {
			t.Fatalf("expected 1 task in queue, got %d", len(*m.queue))
		}
	})
}

func TestTopNonQueuedSuggestion(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	task1 := newTestFocusTask(t, "Task 1", 1, true)
	task2 := newTestFocusTask(t, "Task 2", 2, true)

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)
	m.tasks = []FocusTask{
		{Task: task1, Project: project},
		{Task: task2, Project: project},
	}
	m.ranked = focus.RankCrossProjectTasks(m.tasks, m.dismissedIDs, cfg)

	m.acceptIntoQueue(m.ranked[0].FocusTask)

	sft := m.topNonQueuedSuggestion()
	if sft == nil {
		t.Fatal("expected a non-queued suggestion")
	}
	if sft.Task.ID == m.ranked[0].Task.ID {
		t.Error("topNonQueuedSuggestion should skip queued tasks")
	}

	m.acceptIntoQueue(sft.FocusTask)
	if m.topNonQueuedSuggestion() != nil {
		t.Error("expected nil when all suggestions are queued")
	}
}
