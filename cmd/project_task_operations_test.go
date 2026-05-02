package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestProjectRemoveTaskClearsReferences(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create parent and child
	ep, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, ep)

	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &ep.ID
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Verify child is linked
	if len(ep.Children) != 1 {
		t.Errorf("epic should have 1 child, got %d", len(ep.Children))
	}

	// Remove child task by filtering slice (standard pattern)
	for i, t := range project.Tasks {
		if t.ID == task.ID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

	testutil.AssertTaskCount(t, project, 1)

	// Rebuild tree and verify epic has no children
	project.BuildTaskTree()
	if len(ep.Children) != 0 {
		t.Errorf("epic should have 0 children after removal, got %d", len(ep.Children))
	}
}

func TestProjectRemoveTaskPreservesOthers(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create hierarchy: epic -> story -> task
	ep, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, ep)

	story, err := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	story.ParentID = &ep.ID
	project.Tasks = append(project.Tasks, story)

	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &story.ID
	project.Tasks = append(project.Tasks, task)

	project.BuildTaskTree()

	testutil.AssertTaskCount(t, project, 3)

	// Remove just the task (children removal is handled separately by TUI)
	for i, t := range project.Tasks {
		if t.ID == task.ID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

	// Epic and story should remain
	testutil.AssertTaskCount(t, project, 2)

	// Verify epic and story still exist
	if project.GetTaskByID(ep.ID) == nil {
		t.Error("epic should still exist")
	}
	if project.GetTaskByID(story.ID) == nil {
		t.Error("story should still exist")
	}
}
