package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSelectTaskForMove_WithDirectID(t *testing.T) {
	project := testutil.NewTestProject("Select Move Test")

	task, err := models.NewTask("known-id-123", project.GetNextShortID(), models.TaskTypeTask, "Task to Find")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	args := []string{task.ID}
	found, err := selectTaskForMove(project, args, "up")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, found.ID, task.ID)
}

func TestSelectTaskForMove_TaskNotFound(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Move Not Found Test")
	defer deps.Cleanup()

	args := []string{"nonexistent-task-id"}
	_, err := selectTaskForMove(project, args, "up")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestSelectTaskForMove_FilterUp_NoCandidates(t *testing.T) {
	project := testutil.NewTestProject("Filter Up Test")

	task, err := models.NewTask("single-task", project.GetNextShortID(), models.TaskTypeTask, "Only Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	_, err = selectTaskForMove(project, nil, "up")
	if err == nil {
		t.Error("expected error when no candidates for move up")
	}
}

func TestSelectTaskForMove_FilterDown_NoCandidates(t *testing.T) {
	project := testutil.NewTestProject("Filter Down Test")

	task, err := models.NewTask("single-task", project.GetNextShortID(), models.TaskTypeTask, "Only Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	_, err = selectTaskForMove(project, nil, "down")
	if err == nil {
		t.Error("expected error when no candidates for move down")
	}
}

func TestSelectTaskForMove_FilterOutdent_NoCandidates(t *testing.T) {
	project := testutil.NewTestProject("Filter Outdent Test")

	task, err := models.NewTask("root-task", project.GetNextShortID(), models.TaskTypeTask, "Root Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	_, err = selectTaskForMove(project, nil, "outdent")
	if err == nil {
		t.Error("expected error when no candidates for outdent")
	}
}

func TestSelectTaskForMove_FilterIndent_NoCandidates(t *testing.T) {
	project := testutil.NewTestProject("Filter Indent Test")

	task, err := models.NewTask("only-task", project.GetNextShortID(), models.TaskTypeIdea, "Lonely Idea")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	_, err = selectTaskForMove(project, nil, "indent")
	if err == nil {
		t.Error("expected error when no candidates for indent")
	}
}
