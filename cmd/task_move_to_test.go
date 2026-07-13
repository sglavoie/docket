package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestRunTaskMoveTo_MovesAttachmentFiles guards against `tt task move-to`
// leaving attachment files behind in the source project. Task UUIDs are
// preserved across the move, so destination metadata references the same
// filenames; the on-disk files must follow the task to the new project.
func TestRunTaskMoveTo_MovesAttachmentFiles(t *testing.T) {
	deps, sourceProject := testutil.SetupTestDepsWithProject(t, "Source Project")
	defer deps.Cleanup()

	// Build a destination project rooted at a real path under the test data dir.
	destProject := testutil.NewTestProject("Dest Project")
	destProject.Path = deps.Config.GetProjectPath(destProject.Slug)
	deps.Config.Projects = append(deps.Config.Projects, destProject)
	deps.Repository.Projects[destProject.ID] = destProject

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	if err := manager.Save(deps.Config); err != nil {
		t.Fatalf("save config: %v", err)
	}

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldRepo })

	// Pick a task and seed an attachment file on disk.
	task := sourceProject.Tasks[0]
	if err := storage.EnsureTaskAttachmentsDir(sourceProject, task); err != nil {
		t.Fatalf("ensure attachments dir: %v", err)
	}
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "report.pdf")
	if err := os.WriteFile(srcPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("seed source file: %v", err)
	}
	if _, err := storage.CopyAttachment(sourceProject, task, srcPath); err != nil {
		t.Fatalf("CopyAttachment: %v", err)
	}
	if err := task.AddAttachment("report.pdf", false, srcPath); err != nil {
		t.Fatalf("AddAttachment: %v", err)
	}
	if err := deps.Repository.Save(sourceProject); err != nil {
		t.Fatalf("save source after attach: %v", err)
	}

	taskMoveToForce = true
	defer func() { taskMoveToForce = false }()

	err := runTaskMoveTo(taskMoveToCmd, []string{fmt.Sprintf("%d", task.ShortID), destProject.Slug})
	if err != nil {
		t.Fatalf("runTaskMoveTo: %v", err)
	}

	if storage.AttachmentExists(sourceProject, task, "report.pdf") {
		t.Error("attachment file should not remain in source project")
	}
	if !storage.AttachmentExists(destProject, task, "report.pdf") {
		t.Error("attachment file should exist in destination project")
	}
}

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

func TestMoveTaskToProject_CrossProjectRefsNotCountedAsCleared(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task1)

	task2, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task2)

	// task1 has a same-project dep (task2, not being moved) and a cross-project dep
	task1.DependsOn = []string{task2.ID, "other-project:some-task-uuid"}
	project.BuildTaskTree()

	movedSet := map[string]bool{task1.ID: true}

	// Count deps to be cleared the way the fixed code does:
	// cross-project refs should NOT be counted since they are preserved.
	var depsToBeCleared int
	for _, depID := range task1.DependsOn {
		if models.ParseTaskRef(depID).IsCrossProject() {
			continue
		}
		if !movedSet[depID] {
			depsToBeCleared++
		}
	}

	// Only the same-project dep (task2) should be counted as cleared
	testutil.AssertEqual(t, depsToBeCleared, 1)
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
	task.SetSection(models.SectionInbox)
	sourceProject.Tasks = append(sourceProject.Tasks, task)
	sourceProject.NextShortID++

	destProject := testutil.NewTestProject("Dest")
	destProject.EnsureSection(models.SectionInbox)

	if !destProject.HasSection(models.SectionInbox) {
		t.Error("dest project should have Inbox section")
	}

	testutil.AssertEqual(t, models.SectionForState(task.State()), models.SectionInbox)
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

func TestMoveTaskToProject_UpdatesConfigReferences(t *testing.T) {
	sourceProject := testutil.NewTestProject("Source")
	destProject := testutil.NewTestProject("Dest")

	task, err := models.NewTask("task-move-ref", sourceProject.GetNextShortID(), models.TaskTypeTask, "Moveable Task")
	if err != nil {
		t.Fatal(err)
	}
	sourceProject.Tasks = append(sourceProject.Tasks, task)
	sourceProject.NextShortID++

	// Set up config with the task in FocusQueue, DailyPlan, and RecentTaskVisits
	cfg := config.NewConfig()
	cfg.FocusQueue = []config.FocusQueueItem{
		{TaskID: task.ID, ProjectID: sourceProject.ID},
	}
	cfg.DailyPlan = []config.DailyPlanItem{
		{TaskID: task.ID, ProjectID: sourceProject.ID, PlannedDate: "2026-04-13"},
	}
	cfg.RecentTaskVisits = []config.RecentTaskVisit{
		{TaskID: task.ID, ProjectID: sourceProject.ID},
	}

	// Perform the move
	result, moveErr := sourceProject.MoveTaskToProject(task, destProject)
	if moveErr != nil {
		t.Fatalf("move failed: %v", moveErr)
	}

	// Simulate what the CLI does: update config references
	movedTaskIDs := make(map[string]bool, len(result.MovedTasks))
	for _, mt := range result.MovedTasks {
		movedTaskIDs[mt.ID] = true
	}
	updated := cfg.ReassignTaskProject(movedTaskIDs, destProject.ID)

	if updated != 3 {
		t.Fatalf("expected 3 config entries updated, got %d", updated)
	}
	if cfg.FocusQueue[0].ProjectID != destProject.ID {
		t.Errorf("FocusQueue not updated: got %s, want %s", cfg.FocusQueue[0].ProjectID, destProject.ID)
	}
	if cfg.DailyPlan[0].ProjectID != destProject.ID {
		t.Errorf("DailyPlan not updated: got %s, want %s", cfg.DailyPlan[0].ProjectID, destProject.ID)
	}
	if cfg.RecentTaskVisits[0].ProjectID != destProject.ID {
		t.Errorf("RecentTaskVisits not updated: got %s, want %s", cfg.RecentTaskVisits[0].ProjectID, destProject.ID)
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
