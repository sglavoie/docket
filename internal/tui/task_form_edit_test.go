package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskFormEdit_CanonicalStatePreservedInDraft(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := newTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.ApplyWorkflowState(models.StateWaiting)

	m := NewTaskFormModelEdit(project, task, testRuntimeAt(cfg.DataDir(), cfg))
	if m.draft.state != models.StateWaiting {
		t.Errorf("expected draft.state=%q, got %q", models.StateWaiting, m.draft.state)
	}
}

func TestBuildValidParentsExcludesSelfInEditMode(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test")
	cfg := newTestConfig()

	epic := testutil.NewTestTask("Epic")
	story := testutil.NewTestTask("Story")
	story.ParentID = &epic.ID
	task := testutil.NewTestTask("Leaf Task")
	task.ParentID = &story.ID

	project.Tasks = []*models.Task{epic, story, task}
	project.BuildTaskTree()

	m := NewTaskFormModelEdit(project, story, testRuntimeAt(cfg.DataDir(), cfg))
	parents := m.buildValidParents()

	for _, p := range parents {
		if p.ID == story.ID {
			t.Errorf("buildValidParents should exclude the task being edited (story)")
		}
		if p.ID == task.ID {
			t.Errorf("buildValidParents should exclude descendants of the task being edited (leaf task)")
		}
	}

	found := false
	for _, p := range parents {
		if p.ID == epic.ID {
			found = true
		}
	}
	if !found {
		t.Error("buildValidParents should include valid ancestors (epic)")
	}
}

func TestBuildValidParentsIncludesAllInCreateMode(t *testing.T) {
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
		t.Error("buildValidParents in create mode should include all valid parents")
	}
}
