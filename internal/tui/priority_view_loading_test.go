package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestLoadActionableTasks(t *testing.T) { //nolint:gocyclo
	t.Run("filters out completed tasks", func(t *testing.T) {
		project := newTestPriorityProject(t)

		done, err := models.NewTask("1", 1, models.TaskTypeTask, "Done task")
		if err != nil {
			t.Fatal(err)
		}
		done.WorkflowState = models.StateDone

		canceled, err := models.NewTask("2", 2, models.TaskTypeTask, "Canceled task")
		if err != nil {
			t.Fatal(err)
		}
		canceled.WorkflowState = models.StateCanceled

		active, err := models.NewTask("3", 3, models.TaskTypeTask, "Active task")
		if err != nil {
			t.Fatal(err)
		}
		active.WorkflowState = models.StateReady

		project.Tasks = []*models.Task{done, canceled, active}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if m.totalTasks != 1 {
			t.Errorf("expected 1 actionable task, got %d", m.totalTasks)
		}
	})

	t.Run("separates blocked tasks to waitingOn", func(t *testing.T) {
		project := newTestPriorityProject(t)

		blocked, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocked task")
		if err != nil {
			t.Fatal(err)
		}
		blocked.WorkflowState = models.StateWaiting

		active, err := models.NewTask("2", 2, models.TaskTypeTask, "Active task")
		if err != nil {
			t.Fatal(err)
		}
		active.WorkflowState = models.StateReady

		project.Tasks = []*models.Task{blocked, active}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if m.totalTasks != 1 {
			t.Errorf("expected 1 actionable task, got %d", m.totalTasks)
		}
		if len(m.waitingOn) != 1 {
			t.Errorf("expected 1 waiting task, got %d", len(m.waitingOn))
		}
	})

	t.Run("tasks with incomplete dependencies go to waitingOn", func(t *testing.T) {
		project := newTestPriorityProject(t)

		blocker, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocker")
		if err != nil {
			t.Fatal(err)
		}
		blocker.WorkflowState = models.StateReady

		dependent, err := models.NewTask("2", 2, models.TaskTypeTask, "Dependent")
		if err != nil {
			t.Fatal(err)
		}
		dependent.WorkflowState = models.StateReady
		dependent.DependsOn = []string{blocker.ID}

		project.Tasks = []*models.Task{blocker, dependent}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if m.totalTasks != 1 {
			t.Errorf("expected 1 actionable task, got %d", m.totalTasks)
		}
		if len(m.waitingOn) != 1 {
			t.Errorf("expected 1 waiting task, got %d", len(m.waitingOn))
		}
	})

	t.Run("sorts tasks by urgency within groups", func(t *testing.T) {
		project := newTestPriorityProject(t)

		low, err := models.NewTask("1", 1, models.TaskTypeTask, "Low priority")
		if err != nil {
			t.Fatal(err)
		}
		low.Priority = 5

		high, err := models.NewTask("2", 2, models.TaskTypeTask, "High priority")
		if err != nil {
			t.Fatal(err)
		}
		high.Priority = 1

		medium, err := models.NewTask("3", 3, models.TaskTypeTask, "Medium priority")
		if err != nil {
			t.Fatal(err)
		}
		medium.Priority = 3

		project.Tasks = []*models.Task{low, high, medium}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if len(m.backlog) != 3 {
			t.Fatalf("expected 3 backlog tasks, got %d", len(m.backlog))
		}
		if m.backlog[0].Task.Priority != 1 {
			t.Errorf("expected first task to have priority 1, got %d", m.backlog[0].Task.Priority)
		}
		if m.backlog[1].Task.Priority != 3 {
			t.Errorf("expected second task to have priority 3, got %d", m.backlog[1].Task.Priority)
		}
		if m.backlog[2].Task.Priority != 5 {
			t.Errorf("expected third task to have priority 5, got %d", m.backlog[2].Task.Priority)
		}
	})

	t.Run("bugs rank higher than tasks at same priority", func(t *testing.T) {
		project := newTestPriorityProject(t)

		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Regular task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3

		bug, err := models.NewTask("2", 2, models.TaskTypeBug, "Bug task")
		if err != nil {
			t.Fatal(err)
		}
		bug.Priority = 3

		project.Tasks = []*models.Task{task, bug}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if len(m.backlog) != 2 {
			t.Fatalf("expected 2 backlog tasks, got %d", len(m.backlog))
		}
		if m.backlog[0].Task.Type != models.TaskTypeBug {
			t.Errorf("expected bug to be first, got %s", m.backlog[0].Task.Type)
		}
		if m.backlog[1].Task.Type != models.TaskTypeTask {
			t.Errorf("expected task to be second, got %s", m.backlog[1].Task.Type)
		}
	})

	t.Run("higher priority task still outranks lower priority bug", func(t *testing.T) {
		project := newTestPriorityProject(t)

		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Priority 2 task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 2

		bug, err := models.NewTask("2", 2, models.TaskTypeBug, "Priority 3 bug")
		if err != nil {
			t.Fatal(err)
		}
		bug.Priority = 3

		project.Tasks = []*models.Task{bug, task}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if len(m.backlog) != 2 {
			t.Fatalf("expected 2 backlog tasks, got %d", len(m.backlog))
		}
		if m.backlog[0].Task.Priority != 2 {
			t.Errorf("expected priority 2 task to be first, got priority %d", m.backlog[0].Task.Priority)
		}
		if m.backlog[1].Task.Priority != 3 {
			t.Errorf("expected priority 3 bug to be second, got priority %d", m.backlog[1].Task.Priority)
		}
	})

	t.Run("urgency sort is stable for equal scores", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Build five tasks with identical priority/type/status so they all
		// produce the same coarse-integer UrgencyScore. Insertion order must
		// be preserved by a stable sort; an unstable sort can flip them.
		var tasks []*models.Task
		ids := []string{"a", "b", "c", "d", "e"}
		for i, id := range ids {
			task, err := models.NewTask(id, i+1, models.TaskTypeTask, "Task "+id)
			if err != nil {
				t.Fatal(err)
			}
			task.Priority = 3
			tasks = append(tasks, task)
		}
		project.Tasks = tasks

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if len(m.backlog) != len(ids) {
			t.Fatalf("expected %d backlog tasks, got %d", len(ids), len(m.backlog))
		}
		for i, id := range ids {
			if m.backlog[i].Task.ID != id {
				t.Errorf("position %d: got ID %q, want %q (full order=%v)",
					i, m.backlog[i].Task.ID, id, priorityIDs(m.backlog))
				return
			}
		}
	})

	t.Run("totalTasks excludes waitingOn", func(t *testing.T) {
		project := newTestPriorityProject(t)

		active1, err := models.NewTask("1", 1, models.TaskTypeTask, "Active 1")
		if err != nil {
			t.Fatal(err)
		}
		active2, err := models.NewTask("2", 2, models.TaskTypeTask, "Active 2")
		if err != nil {
			t.Fatal(err)
		}

		blocked, err := models.NewTask("3", 3, models.TaskTypeTask, "Blocked")
		if err != nil {
			t.Fatal(err)
		}
		blocked.WorkflowState = models.StateWaiting

		project.Tasks = []*models.Task{active1, active2, blocked}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if m.totalTasks != 2 {
			t.Errorf("expected totalTasks=2 (excluding waitingOn), got %d", m.totalTasks)
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("empty project", func(t *testing.T) {
		project := newTestPriorityProject(t)
		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if m.totalTasks != 0 {
			t.Errorf("expected 0 total tasks for empty project, got %d", m.totalTasks)
		}
		if len(m.overdue) != 0 || len(m.today) != 0 || len(m.thisWeek) != 0 ||
			len(m.later) != 0 || len(m.backlog) != 0 || len(m.waitingOn) != 0 {
			t.Error("expected all groups to be empty for empty project")
		}
	})

	t.Run("all tasks completed", func(t *testing.T) {
		project := newTestPriorityProject(t)

		done1, err := models.NewTask("1", 1, models.TaskTypeTask, "Done 1")
		if err != nil {
			t.Fatal(err)
		}
		done1.WorkflowState = models.StateDone

		done2, err := models.NewTask("2", 2, models.TaskTypeTask, "Done 2")
		if err != nil {
			t.Fatal(err)
		}
		done2.WorkflowState = models.StateCanceled

		project.Tasks = []*models.Task{done1, done2}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if m.totalTasks != 0 {
			t.Errorf("expected 0 total tasks when all completed, got %d", m.totalTasks)
		}
	})

	t.Run("all tasks blocked", func(t *testing.T) {
		project := newTestPriorityProject(t)

		blocked1, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocked 1")
		if err != nil {
			t.Fatal(err)
		}
		blocked1.WorkflowState = models.StateWaiting

		blocked2, err := models.NewTask("2", 2, models.TaskTypeTask, "Blocked 2")
		if err != nil {
			t.Fatal(err)
		}
		blocked2.WorkflowState = models.StateWaiting

		project.Tasks = []*models.Task{blocked1, blocked2}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if m.totalTasks != 0 {
			t.Errorf("expected 0 actionable tasks when all blocked, got %d", m.totalTasks)
		}
		if len(m.waitingOn) != 2 {
			t.Errorf("expected 2 waiting tasks, got %d", len(m.waitingOn))
		}
	})
}

func TestPriorityViewActionableFilter(t *testing.T) {
	t.Run("actionable filter hides tasks in non-actionable GTD sections", func(t *testing.T) {
		project := newTestPriorityProject(t)

		actionable, err := models.NewTask("1", 1, models.TaskTypeTask, "Actionable task")
		if err != nil {
			t.Fatal(err)
		}

		inbox, err := models.NewTask("2", 2, models.TaskTypeTask, "Inbox task")
		if err != nil {
			t.Fatal(err)
		}
		inbox.SetSection(models.SectionInbox)

		waiting, err := models.NewTask("3", 3, models.TaskTypeTask, "Waiting task")
		if err != nil {
			t.Fatal(err)
		}
		waiting.SetSection(models.SectionWaiting)

		someday, err := models.NewTask("4", 4, models.TaskTypeTask, "Someday task")
		if err != nil {
			t.Fatal(err)
		}
		someday.SetSection(models.SectionSomeday)

		review, err := models.NewTask("5", 5, models.TaskTypeTask, "Review task")
		if err != nil {
			t.Fatal(err)
		}
		review.SetSection(models.SectionReview)

		standby, err := models.NewTask("6", 6, models.TaskTypeTask, "Standby task")
		if err != nil {
			t.Fatal(err)
		}
		standby.SetSection(models.SectionStandby)

		next, err := models.NewTask("7", 7, models.TaskTypeTask, "Next task")
		if err != nil {
			t.Fatal(err)
		}
		next.SetSection(models.SectionNext)

		project.Tasks = []*models.Task{actionable, inbox, waiting, someday, review, standby, next}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		// The Waiting-section task derives the waiting state and groups under
		// Waiting On, so only 6 of the 7 tasks land in the backlog.
		if len(m.backlog) != 6 {
			t.Fatalf("without filter: expected 6 backlog tasks, got %d", len(m.backlog))
		}
		if len(m.waitingOn) != 1 || m.waitingOn[0].Task.Title != "Waiting task" {
			t.Fatalf("without filter: expected the Waiting task under waitingOn, got %d entries", len(m.waitingOn))
		}

		m.filter.Actionable = true
		m.loadActionableTasks()

		if len(m.backlog) != 1 {
			t.Errorf("with actionable filter: expected 1 backlog task, got %d", len(m.backlog))
		}
		if len(m.backlog) > 0 && m.backlog[0].Task.Title != "Actionable task" {
			t.Errorf("expected remaining task to be 'Actionable task', got %q", m.backlog[0].Task.Title)
		}
	})

	t.Run("actionable filter hides blocked tasks from groups", func(t *testing.T) {
		project := newTestPriorityProject(t)

		actionable, err := models.NewTask("1", 1, models.TaskTypeTask, "Actionable")
		if err != nil {
			t.Fatal(err)
		}

		blocked, err := models.NewTask("2", 2, models.TaskTypeTask, "Blocked")
		if err != nil {
			t.Fatal(err)
		}
		blocked.WorkflowState = models.StateWaiting

		project.Tasks = []*models.Task{actionable, blocked}

		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)
		m.filter.Actionable = true
		m.loadActionableTasks()

		if len(m.backlog) != 1 {
			t.Errorf("expected 1 backlog task with actionable filter, got %d", len(m.backlog))
		}
	})

	t.Run("HasActiveFilters reflects actionable state", func(t *testing.T) {
		project := newTestPriorityProject(t)
		m := NewPriorityViewModel(project, config.EmptyRuntimeState(""), nil)

		if m.HasActiveFilters() {
			t.Error("HasActiveFilters should be false initially")
		}

		m.filter.Actionable = true
		if !m.HasActiveFilters() {
			t.Error("HasActiveFilters should be true when Actionable is set")
		}
	})
}
