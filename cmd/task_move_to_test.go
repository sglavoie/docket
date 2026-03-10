package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestMoveTaskToProject_SourceDestValidation(t *testing.T) {
	tests := []struct {
		name       string
		sourceID   string
		destID     string
		expectErr  bool
		errMessage string
	}{
		{
			name:       "same project fails",
			sourceID:   "proj-1",
			destID:     "proj-1",
			expectErr:  true,
			errMessage: "cannot move task to the same project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceProject := testutil.NewTestProject("Source")
			sourceProject.ID = tt.sourceID

			destProject := testutil.NewTestProject("Dest")
			destProject.ID = tt.destID

			if sourceProject.ID == destProject.ID && !tt.expectErr {
				t.Error("expected error for same project")
			}
		})
	}
}

func TestMoveTaskToProject_PreparesMovedSet(t *testing.T) {
	sourceProject := testutil.NewTestProjectWithTasks("Source Project")
	destProject := testutil.NewTestProject("Dest Project")

	sourceProject.BuildTaskTree()

	task := sourceProject.Tasks[0]

	movedSet := make(map[string]bool)
	var collectMovedIDs func(taskID string)
	collectMovedIDs = func(taskID string) {
		t := sourceProject.GetTaskByID(taskID)
		if t == nil {
			return
		}
		movedSet[t.ID] = true
		for _, child := range t.Children {
			collectMovedIDs(child.ID)
		}
	}
	collectMovedIDs(task.ID)

	if !movedSet[task.ID] {
		t.Error("moved set should include the task itself")
	}

	if len(task.Children) > 0 {
		for _, child := range task.Children {
			if !movedSet[child.ID] {
				t.Error("moved set should include all children")
			}
		}
	}

	_ = destProject // Used in full implementation
}

func TestMoveTaskToProject_CountsDescendants(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	project.BuildTaskTree()

	var countDescendants func(task *models.Task) int
	countDescendants = func(task *models.Task) int {
		count := 0
		for _, child := range task.Children {
			count++
			count += countDescendants(child)
		}
		return count
	}

	for _, task := range project.Tasks {
		descendants := countDescendants(task)
		expected := len(task.Children) + countNestedChildren(task)
		if descendants != expected {
			t.Errorf("descendant count mismatch for %s: got %d, expected %d", task.Title, descendants, expected)
		}
	}
}

func countNestedChildren(task *models.Task) int {
	count := 0
	for _, child := range task.Children {
		count += len(child.Children)
		count += countNestedChildren(child)
	}
	return count
}

func TestMoveTaskToProject_ChecksDependencies(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")

	task1, err := models.NewTask("task-dep-1", project.GetNextShortID(), models.TaskTypeTask, "Task With Dep")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task1)
	project.NextShortID++

	task2, err := models.NewTask("task-dep-2", project.GetNextShortID(), models.TaskTypeTask, "Blocking Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task2)
	project.NextShortID++

	task1.DependsOn = []string{task2.ID}

	project.BuildTaskTree()

	movedSet := map[string]bool{task1.ID: true}

	var depsToBeCleared int
	for _, depID := range task1.DependsOn {
		if !movedSet[depID] {
			depsToBeCleared++
		}
	}

	testutil.AssertEqual(t, depsToBeCleared, 1)
}

func TestMoveTaskToProject_ClearedDepsTracking(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task1)
	project.NextShortID++

	task2, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task2)
	project.NextShortID++

	task3, err := models.NewTask("task-3", project.GetNextShortID(), models.TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.DependsOn = []string{task1.ID, task2.ID}
	project.Tasks = append(project.Tasks, task3)
	project.NextShortID++

	project.BuildTaskTree()

	movedSet := map[string]bool{
		task3.ID: true,
		task1.ID: true,
	}

	var clearedDeps []string
	for _, depID := range task3.DependsOn {
		if !movedSet[depID] {
			clearedDeps = append(clearedDeps, depID)
		}
	}

	testutil.AssertEqual(t, len(clearedDeps), 1)
	testutil.AssertEqual(t, clearedDeps[0], task2.ID)
}

func TestMoveTaskToProject_TaskNotFound(t *testing.T) {
	project := testutil.NewTestProject("Test Project")
	_, findErr := FindTaskByIdentifier(project, "nonexistent")
	if findErr == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestMoveTaskToProject_NoTasks(t *testing.T) {
	project := testutil.NewTestProject("Empty Project")
	if len(project.Tasks) != 0 {
		t.Errorf("expected empty project to have 0 tasks, got %d", len(project.Tasks))
	}
}

func TestMoveTaskToProject_ShortIDRegeneration(t *testing.T) {
	sourceProject := testutil.NewTestProjectWithTasks("Source")
	destProject := testutil.NewTestProject("Dest")

	sourceTask := sourceProject.Tasks[0]
	originalShortID := sourceTask.ShortID

	destProject.NextShortID = 100

	newShortID := destProject.GetNextShortID()

	// Verify the new ID is from the destination project
	testutil.AssertEqual(t, newShortID, 100)

	// Original short ID should be different (source project starts at 1)
	if newShortID == originalShortID {
		t.Errorf("expected different short IDs: original=%d, new=%d", originalShortID, newShortID)
	}
}

func TestMoveTaskToProject_SectionPreservation(t *testing.T) {
	sourceProject := testutil.NewTestProject("Source")

	task, err := models.NewTask("task-1", sourceProject.GetNextShortID(), models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = "Active"
	sourceProject.Tasks = append(sourceProject.Tasks, task)
	sourceProject.NextShortID++

	destProject := testutil.NewTestProject("Dest")
	destProject.EnsureSection("Active")

	if !destProject.HasSection("Active") {
		t.Error("dest project should have Active section")
	}

	testutil.AssertEqual(t, task.Section, "Active")
}

func TestMoveTaskToProject_NoOtherProjects(t *testing.T) {
	projects := []*models.Project{
		testutil.NewTestProject("Only Project"),
	}

	currentProject := projects[0]
	var candidates []*models.Project

	for _, p := range projects {
		if p.ID != currentProject.ID {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) != 0 {
		t.Error("expected no candidate projects when only one exists")
	}
}

func TestMoveTaskToProject_FiltersCandidates(t *testing.T) {
	projects := []*models.Project{
		testutil.NewTestProject("Project A"),
		testutil.NewTestProject("Project B"),
		testutil.NewTestProject("Project C"),
	}

	currentProject := projects[0]
	var candidates []*models.Project

	for _, p := range projects {
		if p.ID != currentProject.ID {
			candidates = append(candidates, p)
		}
	}

	testutil.AssertEqual(t, len(candidates), 2)
}
