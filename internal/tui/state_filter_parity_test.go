package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// buildWorkflowStateFixture returns a project seeded with one leaf task per
// canonical workflow state (via the legacy Status/Section backing fields the
// migration adapter derives from) plus a dependency-blocked task. It is the
// shared fixture the state/blocked filter parity tests exercise; the CLI side
// (cmd/task_list_state_filter_test.go) builds an equivalent project so both
// layers filter the identical task population.
func buildWorkflowStateFixture(t *testing.T) *models.Project {
	t.Helper()
	project := &models.Project{ID: "p1", Name: "Project"}

	add := func(id string, mutate func(*models.Task)) *models.Task {
		task, err := models.NewTask(id, len(project.Tasks)+1, models.TaskTypeTask, "Task "+id)
		if err != nil {
			t.Fatalf("NewTask(%q): %v", id, err)
		}
		if mutate != nil {
			mutate(task)
		}
		project.Tasks = append(project.Tasks, task)
		return task
	}

	add("inbox", func(tk *models.Task) { tk.SetSection(models.SectionInbox) })
	add("next", func(tk *models.Task) { tk.SetSection(models.SectionNext) })
	add("ready", nil) // default state derives to StateReady
	add("in_progress", func(tk *models.Task) { tk.WorkflowState = models.StateInProgress })
	add("waiting", func(tk *models.Task) { tk.WorkflowState = models.StateWaiting })
	add("paused", func(tk *models.Task) { tk.WorkflowState = models.StatePaused })
	add("review", func(tk *models.Task) { tk.WorkflowState = models.StateReview })
	add("someday", func(tk *models.Task) { tk.WorkflowState = models.StateSomeday })
	add("done", func(tk *models.Task) { tk.WorkflowState = models.StateDone })
	add("canceled", func(tk *models.Task) { tk.WorkflowState = models.StateCanceled })

	// A dependency-blocked task: derives to StateReady but is gated by the
	// still-incomplete "ready" task, so it is the sole IsDependencyBlocked task.
	blocker := project.Tasks[2] // the "ready" task, which is not complete
	add("blocked", func(tk *models.Task) {
		if err := tk.AddDependency(blocker.ID); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	})

	project.BuildTaskTree()
	return project
}

// TestStateFilter_SelectsTasksByDerivedState proves the shared common-filter
// State field selects exactly the tasks whose derived WorkflowState equals the
// filter value, across all ten canonical states.
func TestStateFilter_SelectsTasksByDerivedState(t *testing.T) {
	project := buildWorkflowStateFixture(t)

	for _, state := range models.AllWorkflowStates() {
		state := state
		t.Run(state.String(), func(t *testing.T) {
			filter := newFilterState()
			filter.State = state
			for _, task := range project.Tasks {
				want := task.State() == state
				got := passesCommonFilters(task, &filter, project, nil)
				if got != want {
					t.Errorf("task %q (state %s): passesCommonFilters(State=%s)=%v, want %v",
						task.ID, task.State(), state, got, want)
				}
			}
		})
	}
}

// TestBlockedFilter_SelectsDependencyBlockedTasks proves the shared Blocked
// filter selects exactly the tasks IsDependencyBlocked reports for the project.
func TestBlockedFilter_SelectsDependencyBlockedTasks(t *testing.T) {
	project := buildWorkflowStateFixture(t)

	filter := newFilterState()
	filter.Blocked = true

	blockedCount := 0
	for _, task := range project.Tasks {
		want := task.IsDependencyBlocked(project)
		if want {
			blockedCount++
		}
		got := passesCommonFilters(task, &filter, project, nil)
		if got != want {
			t.Errorf("task %q: passesCommonFilters(Blocked)=%v, want %v (IsDependencyBlocked)", task.ID, got, want)
		}
	}
	if blockedCount != 1 {
		t.Fatalf("fixture should have exactly one dependency-blocked task, got %d", blockedCount)
	}
}

// TestStateFilter_ActiveClearAndBadge confirms the State and Blocked filters are
// recognized as active common filters, are reset by clearCommonFilters, and are
// surfaced in the filter badge parts.
func TestStateFilter_ActiveClearAndBadge(t *testing.T) {
	filter := newFilterState()
	filter.State = models.StateInProgress
	filter.Blocked = true

	if !hasActiveCommonFilters(&filter) {
		t.Fatal("State/Blocked filters should count as active common filters")
	}

	parts := commonFilterParts(&filter)
	if !containsString(parts, "state:in_progress") {
		t.Errorf("commonFilterParts()=%v, want to contain state:in_progress", parts)
	}
	if !containsString(parts, "blocked") {
		t.Errorf("commonFilterParts()=%v, want to contain blocked", parts)
	}

	clearCommonFilters(&filter)
	if filter.State != "" || filter.Blocked {
		t.Errorf("clearCommonFilters left State=%q Blocked=%v", filter.State, filter.Blocked)
	}
	if hasActiveCommonFilters(&filter) {
		t.Error("cleared filter should not be active")
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
