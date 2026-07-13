package planner

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// newTask is a test helper that creates a task with the given ID.
func newTask(id string) *models.Task {
	t, err := models.NewTask(id, 1, models.TaskTypeTask, "Task "+id)
	if err != nil {
		panic(fmt.Sprintf("newTask: %v", err))
	}
	return t
}

// newTaskWithDuration creates a task with a given EstimatedDays.
func newTaskWithDuration(id string, days float64) *models.Task {
	t := newTask(id)
	t.EstimatedDays = days
	return t
}

// withDep adds a dependency ID to a task and returns the task.
func withDep(t *models.Task, depIDs ...string) *models.Task {
	for _, id := range depIDs {
		_ = t.AddDependency(id)
	}
	return t
}

// withStatus marks a task's status and returns the task.
func withStatus(t *models.Task, s models.TaskStatus) *models.Task {
	t.WorkflowState = models.DeriveWorkflowState(s, "", "")
	return t
}

// --- assertion helpers ---

// taskPositions returns a map from task ID to its position in the order slice.
func taskPositions(order []*models.Task) map[string]int {
	pos := make(map[string]int, len(order))
	for i, t := range order {
		pos[t.ID] = i
	}
	return pos
}

// taskIDs returns a slice of task IDs from the given tasks.
func taskIDs(tasks []*models.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

// scheduleByID returns a map from task ID to ScheduledTask.
func scheduleByID(sched []ScheduledTask) map[string]ScheduledTask {
	m := make(map[string]ScheduledTask, len(sched))
	for _, s := range sched {
		m[s.Task.ID] = s
	}
	return m
}

func scheduleByTask(sched []ScheduledTask) map[*models.Task]ScheduledTask {
	m := make(map[*models.Task]ScheduledTask, len(sched))
	for _, s := range sched {
		m[s.Task] = s
	}
	return m
}

func taskPointerPositions(order []*models.Task) map[*models.Task]int {
	pos := make(map[*models.Task]int, len(order))
	for i, t := range order {
		pos[t] = i
	}
	return pos
}

// abs returns the absolute value of x.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
