package cmd

import (
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// buildStateFilterProject returns a project seeded with one leaf task per
// canonical workflow state (via the legacy Status/Section backing fields) plus
// a dependency-blocked task. It mirrors the TUI fixture in
// internal/tui/state_filter_parity_test.go so both layers filter the identical
// task population — the parity the migration requires.
func buildStateFilterProject(t *testing.T) *models.Project {
	t.Helper()
	project := &models.Project{ID: "p1", Name: "Project"}

	add := func(mutate func(*models.Task)) *models.Task {
		task, err := models.NewTask(uuid.New().String(), len(project.Tasks)+1, models.TaskTypeTask, "task")
		if err != nil {
			t.Fatalf("NewTask: %v", err)
		}
		if mutate != nil {
			mutate(task)
		}
		project.Tasks = append(project.Tasks, task)
		return task
	}

	add(func(tk *models.Task) { tk.WorkflowState = models.StateInbox })
	add(func(tk *models.Task) { tk.WorkflowState = models.StateNext })
	ready := add(nil) // default derives to StateReady
	add(func(tk *models.Task) { tk.WorkflowState = models.StateInProgress })
	add(func(tk *models.Task) { tk.WorkflowState = models.StateWaiting })
	add(func(tk *models.Task) { tk.WorkflowState = models.StatePaused })
	add(func(tk *models.Task) { tk.WorkflowState = models.StateReview })
	add(func(tk *models.Task) { tk.WorkflowState = models.StateSomeday })
	add(func(tk *models.Task) { tk.WorkflowState = models.StateDone })
	add(func(tk *models.Task) { tk.WorkflowState = models.StateCanceled })

	add(func(tk *models.Task) {
		if err := tk.AddDependency(ready.ID); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	})

	project.BuildTaskTree()
	return project
}

// TestFilterTasks_ByState proves the CLI --state filter selects exactly the
// tasks whose derived WorkflowState equals the flag value, across all ten
// canonical states — matching the TUI shared filter's selection.
func TestFilterTasks_ByState(t *testing.T) {
	project := buildStateFilterProject(t)

	for _, state := range models.AllWorkflowStates() {
		state := state
		t.Run(state.String(), func(t *testing.T) {
			defer resetFilterVars()
			listShowAll = true
			listState = state.String()
			listFilterProject = project

			result := filterTasks(project.Tasks)

			for _, task := range result {
				if task.State() != state {
					t.Errorf("--state %s returned task in state %s", state, task.State())
				}
			}

			var want int
			for _, task := range project.Tasks {
				if task.State() == state {
					want++
				}
			}
			if len(result) != want {
				t.Errorf("--state %s: got %d tasks, want %d", state, len(result), want)
			}
		})
	}
}

// TestFilterTasks_ByBlocked proves the CLI --blocked filter selects exactly the
// dependency-blocked tasks IsDependencyBlocked reports.
func TestFilterTasks_ByBlocked(t *testing.T) {
	defer resetFilterVars()
	project := buildStateFilterProject(t)

	listShowAll = true
	listBlocked = true
	listFilterProject = project

	result := filterTasks(project.Tasks)

	if len(result) != 1 {
		t.Fatalf("--blocked: got %d tasks, want 1", len(result))
	}
	if !result[0].IsDependencyBlocked(project) {
		t.Errorf("--blocked returned task %q that is not dependency-blocked", result[0].ID)
	}
}

// TestFilterTasks_InvalidStateSelectsNone confirms an unparseable --state value
// matches no tasks (the value is validated up front in runTaskList; the
// predicate fails closed).
func TestFilterTasks_InvalidStateSelectsNone(t *testing.T) {
	defer resetFilterVars()
	project := buildStateFilterProject(t)

	listShowAll = true
	listState = "not_a_state"
	listFilterProject = project

	if got := len(filterTasks(project.Tasks)); got != 0 {
		t.Errorf("invalid --state matched %d tasks, want 0", got)
	}
}
