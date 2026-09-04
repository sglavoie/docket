package taskform

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestBuildValidParentsIncludesAllProjectTasks(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test")
	cfg := newTestConfig()

	epic := testutil.NewTestTask("Epic")
	project.Tasks = []*models.Task{epic}
	project.BuildTaskTree()

	m := NewTaskFormModelCreate(project, testRuntimeAt(cfg.DataDir(), cfg), nil)
	parents := m.buildValidParents()

	found := false
	for _, p := range parents {
		if p.ID == epic.ID {
			found = true
		}
	}
	if !found {
		t.Error("buildValidParents should include all project tasks")
	}
}
