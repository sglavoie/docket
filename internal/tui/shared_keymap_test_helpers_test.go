package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func newSharedBulkProject(name string, titles ...string) (*models.Project, []*models.Task) {
	project := testutil.NewTestProject(name)
	tasks := make([]*models.Task, 0, len(titles))
	for i, title := range titles {
		task := testutil.NewTestTask(title, models.TaskTypeTask)
		task.ShortID = i + 1
		task.Project = project
		project.Tasks = append(project.Tasks, task)
		tasks = append(tasks, task)
	}
	project.NextShortID = len(project.Tasks) + 1
	project.BuildTaskTree()
	return project, tasks
}

func findSharedTaskKeymapAction(t *testing.T, km *keymap.KeyMap, keys ...string) keymap.Action {
	t.Helper()

	if len(keys) != 1 {
		t.Fatalf("direct binding lookup requires one key, got %v", keys)
	}
	for _, entry := range km.Entries {
		if entry.Binding != nil && entry.Binding.Key == keys[0] {
			return entry.Binding.Action
		}
	}

	t.Fatalf("binding %v did not resolve to an action", keys)
	return nil
}
