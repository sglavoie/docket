package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestGroupByStatusIncludesStandby(t *testing.T) {
	proj := testutil.NewTestProject("p")
	standbyTask := testutil.NewTestTaskWithStatus("standby task", models.StatusStandby)
	proj.Tasks = []*models.Task{standbyTask}

	m := newMinimalAllTasksVM()
	items := []CrossProjectTask{{Task: standbyTask, Project: proj}}
	groups := m.groupByStatus(items)

	// The legacy standby status maps to the paused workflow state.
	found := false
	for _, g := range groups {
		if g.Name == string(models.StatePaused) {
			found = true
			if len(g.Tasks) != 1 {
				t.Errorf("paused group has %d tasks, want 1", len(g.Tasks))
			}
		}
	}
	if !found {
		t.Error("groupByStatus did not produce a group for paused (standby) tasks")
	}
}

func TestGroupByStatusIncludesAllActiveStatuses(t *testing.T) {
	proj := testutil.NewTestProject("p")

	statuses := models.AllTaskStatuses()
	var items []CrossProjectTask
	for _, s := range statuses {
		task := testutil.NewTestTaskWithStatus("task-"+string(s), s)
		items = append(items, CrossProjectTask{Task: task, Project: proj})
	}

	m := newMinimalAllTasksVM()
	groups := m.groupByStatus(items)

	groupNames := make(map[string]bool)
	for _, g := range groups {
		groupNames[g.Name] = true
	}

	// Every task groups under its derived workflow state.
	for _, item := range items {
		if !groupNames[string(item.Task.State())] {
			t.Errorf("groupByStatus missing group for state %q (task %q)", item.Task.State(), item.Task.Title)
		}
	}
}
