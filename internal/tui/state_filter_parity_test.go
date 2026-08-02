package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func testWorkflowStateSet(states ...models.WorkflowState) models.WorkflowStateSet {
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		panic(err)
	}
	return set
}

// buildWorkflowStateFixture returns a project seeded with one leaf task per
// canonical workflow state. It is the shared fixture the state-filter parity
// tests exercise; the CLI side builds an equivalent project so both layers
// filter the identical task population.
func buildWorkflowStateFixture(t *testing.T) *models.Project {
	t.Helper()
	project := &models.Project{ID: "p1", Name: "Project"}

	add := func(id string, mutate func(*models.Task)) *models.Task {
		task, err := models.NewGenericTask(id, len(project.Tasks)+1, "Task "+id)
		if err != nil {
			t.Fatalf("NewTask(%q): %v", id, err)
		}
		if mutate != nil {
			mutate(task)
		}
		project.Tasks = append(project.Tasks, task)
		return task
	}

	add("inbox", func(tk *models.Task) { tk.ApplyWorkflowState(models.StateInbox) })
	add("next", func(tk *models.Task) { tk.ApplyWorkflowState(models.StateBacklog) })
	add("ready", nil) // default state derives to StateBacklog
	add("in_progress", func(tk *models.Task) { tk.WorkflowState = models.StateDoing })
	add("waiting", func(tk *models.Task) { tk.WorkflowState = models.StateWaiting })
	add("paused", func(tk *models.Task) { tk.WorkflowState = models.StatePaused })
	add("someday", func(tk *models.Task) { tk.WorkflowState = models.StateSomeday })
	add("done", func(tk *models.Task) { tk.WorkflowState = models.StateDone })
	add("canceled", func(tk *models.Task) { tk.WorkflowState = models.StateCanceled })

	project.BuildTaskTree()
	return project
}

// TestStateFilter_SelectsTasksByDerivedState proves the shared common-filter
// State field selects exactly the tasks whose derived WorkflowState equals the
// filter value, across all eight canonical states.
func TestStateFilter_SelectsTasksByDerivedState(t *testing.T) {
	project := buildWorkflowStateFixture(t)

	for _, state := range models.AllWorkflowStates() {
		state := state
		t.Run(state.String(), func(t *testing.T) {
			filter := newFilterState()
			filter.setStateFilter(testWorkflowStateSet(state), true)
			for _, task := range project.Tasks {
				want := task.State() == state
				got := passesCommonFilters(task, &filter, project)
				if got != want {
					t.Errorf("task %q (state %s): passesCommonFilters(State=%s)=%v, want %v",
						task.ID, task.State(), state, got, want)
				}
			}
		})
	}
}

func TestStateFilter_MatchesAnySelectedStateAndExplicitEmptyMatchesNone(t *testing.T) {
	project := buildWorkflowStateFixture(t)
	filter := newFilterState()
	filter.setStateFilter(testWorkflowStateSet(models.StateInbox, models.StateWaiting, models.StateDone), true)

	for _, task := range project.Tasks {
		want := filter.States.Contains(task.State())
		if got := passesCommonFilters(task, &filter, project); got != want {
			t.Errorf("task %q state=%s: multi-state filter got %v, want %v", task.ID, task.State(), got, want)
		}
	}

	filter.setStateFilter(models.WorkflowStateSet{}, true)
	if !hasActiveCommonFilters(&filter) {
		t.Fatal("explicit empty state set must remain an active match-nothing filter")
	}
	for _, task := range project.Tasks {
		if passesCommonFilters(task, &filter, project) {
			t.Errorf("explicit empty state set unexpectedly matched task %q", task.ID)
		}
	}
}

// TestStateFilter_ActiveClearAndBadge confirms the State and Blocked filters are
// recognized as active common filters, are reset by clearCommonFilters, and are
// surfaced in the filter badge parts.
func TestStateFilter_ActiveClearAndBadge(t *testing.T) {
	filter := newFilterState()
	filter.setStateFilter(testWorkflowStateSet(models.StateBacklog, models.StateDoing), true)
	filter.Blocked = true

	if !hasActiveCommonFilters(&filter) {
		t.Fatal("State/Blocked filters should count as active common filters")
	}

	parts := commonFilterParts(&filter)
	if !containsString(parts, "state:backlog,doing") {
		t.Errorf("commonFilterParts()=%v, want canonical multi-state chip", parts)
	}
	if !containsString(parts, "blocked") {
		t.Errorf("commonFilterParts()=%v, want to contain blocked", parts)
	}

	clearCommonFilters(&filter)
	if filter.hasStateFilter() || filter.Blocked {
		t.Errorf("clearCommonFilters left States=%v Blocked=%v", filter.States.Values(), filter.Blocked)
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
