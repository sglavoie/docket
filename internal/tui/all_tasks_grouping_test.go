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

	found := false
	for _, g := range groups {
		if g.Name == string(models.StatusStandby) {
			found = true
			if len(g.Tasks) != 1 {
				t.Errorf("standby group has %d tasks, want 1", len(g.Tasks))
			}
		}
	}
	if !found {
		t.Error("groupByStatus did not produce a group for standby tasks")
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

	for _, s := range statuses {
		if !groupNames[string(s)] {
			t.Errorf("groupByStatus missing group for status %q", s)
		}
	}
}
