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
				got := passesCommonFilters(task, &filter)
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
		if got := passesCommonFilters(task, &filter); got != want {
			t.Errorf("task %q state=%s: multi-state filter got %v, want %v", task.ID, task.State(), got, want)
		}
	}

	// An explicitly empty state set narrows nothing. The facets are clauses now,
	// and the query has no match-nothing term to spell one with: an empty set
	// mints no state clause, so the axis is simply absent from the predicate. No
	// key can author this carrier — the states picker deactivates the facet on an
	// empty result — so the only way to reach it is to write the field directly.
	filter.setStateFilter(models.WorkflowStateSet{}, true)
	if !hasActiveCommonFilters(&filter) {
		t.Fatal("explicit empty state set must remain an active facet for the chrome")
	}
	for _, task := range project.Tasks {
		if !passesCommonFilters(task, &filter) {
			t.Errorf("explicit empty state set narrowed away task %q", task.ID)
		}
	}
}

// TestStateFilter_ActiveClearAndBadge confirms the State filter is recognized
// as active, is reset by clearCommonFilters, and appears in filter badge parts.
func TestStateFilter_ActiveClearAndBadge(t *testing.T) {
	filter := newFilterState()
	filter.setStateFilter(testWorkflowStateSet(models.StateBacklog, models.StateDoing), true)

	if !hasActiveCommonFilters(&filter) {
		t.Fatal("State filter should count as an active common filter")
	}

	// One state is one clause, so the badge carries one $state chip per member.
	parts := commonFilterParts(&filter)
	if !containsString(parts, "$backlog") || !containsString(parts, "$doing") {
		t.Errorf("commonFilterParts()=%v, want a chip per selected state", parts)
	}
	clearCommonFilters(&filter)
	if filter.hasStateFilter() {
		t.Errorf("clearCommonFilters left States=%v", filter.States.Values())
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
