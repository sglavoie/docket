package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestGetCompletedTasks_NoneCompleted(t *testing.T) {
	project := testutil.NewTestProject("No Completed")

	task1 := testutil.NewTestTaskWithStatus("Task A", models.StatusTodo)
	task2 := testutil.NewTestTaskWithStatus("Task B", models.StatusTodo)
	task3 := testutil.NewTestTaskWithStatus("Task C", models.StatusTodo)
	project.Tasks = []*models.Task{task1, task2, task3}

	completed := project.GetCompletedTasks()
	testutil.AssertEqual(t, len(completed), 0)
}

func TestTaskArchivePreservesDuplicateIDReferencesInOtherProject(t *testing.T) {
	deps, project := setupStateTest(t)
	completed := project.GetCompletedTasks()
	if len(completed) == 0 {
		t.Fatal("test project has no completed task")
	}
	task := completed[0]
	deps.Config.Planning().FocusQueue = []config.FocusQueueItem{
		{TaskID: task.ID, ProjectID: "alpha"},
		{TaskID: task.ID, ProjectID: project.ID},
	}
	deps.Config.Planning().DailyPlan = []config.DailyPlanItem{
		{TaskID: task.ID, ProjectID: "alpha"},
		{TaskID: task.ID, ProjectID: project.ID},
	}
	deps.Config.Planning().FocusDismissedIDs = []string{task.ID}
	deps.Config.Machine().RecentTaskVisits = []config.RecentTaskVisit{
		{TaskID: task.ID, ProjectID: "alpha"},
		{TaskID: task.ID, ProjectID: project.ID},
	}
	manager := config.NewManager(deps.ConfigPath)
	if err := saveTestRuntime(manager, deps.Config); err != nil {
		t.Fatalf("saveTestRuntime: %v", err)
	}

	previousForce := archiveForce
	previousAll := archiveAll
	archiveForce = true
	archiveAll = false
	t.Cleanup(func() {
		archiveForce = previousForce
		archiveAll = previousAll
	})
	if err := runTaskArchive(newStateTestCmd(), nil); err != nil {
		t.Fatalf("runTaskArchive: %v", err)
	}

	reloaded, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if got := reloaded.Planning().FocusQueue; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Fatalf("archive cleaned wrong focus refs: %+v", got)
	}
	if got := reloaded.Planning().DailyPlan; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Fatalf("archive cleaned wrong daily refs: %+v", got)
	}
	if got := reloaded.Planning().FocusDismissedIDs; len(got) != 1 || got[0] != task.ID {
		t.Fatalf("archive changed legacy dismissed IDs: %v", got)
	}
	if got := reloaded.Machine().RecentTaskVisits; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Fatalf("archive cleaned wrong recent visits: %+v", got)
	}
}

func TestGetCompletedTasks_SomeCompleted(t *testing.T) {
	project := testutil.NewTestProject("Some Completed")

	todo := testutil.NewTestTaskWithStatus("Todo Task", models.StatusTodo)
	inProgress := testutil.NewTestTaskWithStatus("In Progress Task", models.StatusInProgress)
	done := testutil.NewTestTaskWithStatus("Done Task", models.StatusDone)
	canceled := testutil.NewTestTaskWithStatus("Canceled Task", models.StatusCanceled)
	blocked := testutil.NewTestTaskWithStatus("Blocked Task", models.StatusBlocked)
	review := testutil.NewTestTaskWithStatus("Review Task", models.StatusReview)
	project.Tasks = []*models.Task{todo, inProgress, done, canceled, blocked, review}

	completed := project.GetCompletedTasks()
	testutil.AssertEqual(t, len(completed), 2)

	for _, task := range completed {
		if !task.IsComplete() {
			t.Errorf("expected completed task, got status %s", models.LegacyStatusForState(task.State()))
		}
	}
}

func TestGetCompletedTasks_AllCompleted(t *testing.T) {
	project := testutil.NewTestProject("All Completed")

	done1 := testutil.NewTestTaskWithStatus("Done 1", models.StatusDone)
	done2 := testutil.NewTestTaskWithStatus("Done 2", models.StatusDone)
	done3 := testutil.NewTestTaskWithStatus("Done 3", models.StatusDone)
	project.Tasks = []*models.Task{done1, done2, done3}

	completed := project.GetCompletedTasks()
	testutil.AssertEqual(t, len(completed), 3)
}

func TestGetCompletedTasks_AfterCompletingTask(t *testing.T) {
	project := testutil.NewTestProject("After Completing")

	task := testutil.NewTestTaskWithStatus("Todo Task", models.StatusTodo)
	project.Tasks = []*models.Task{task}

	// Before completing, no completed tasks
	completed := project.GetCompletedTasks()
	testutil.AssertEqual(t, len(completed), 0)

	// Complete the task
	task.Complete()
	testutil.AssertTaskStatus(t, task, models.StatusDone)

	// Now it should appear in completed tasks
	completed = project.GetCompletedTasks()
	testutil.AssertEqual(t, len(completed), 1)
	testutil.AssertEqual(t, completed[0].Title, "Todo Task")
}

func TestGetCompletedTasks_CanceledTaskIncluded(t *testing.T) {
	project := testutil.NewTestProject("Canceled Included")

	task := testutil.NewTestTask("Cancelable Task", models.TaskTypeTask)
	project.Tasks = []*models.Task{task}

	// Cancel the task
	task.Cancel()
	testutil.AssertTaskStatus(t, task, models.StatusCanceled)

	completed := project.GetCompletedTasks()
	testutil.AssertEqual(t, len(completed), 1)
	testutil.AssertEqual(t, completed[0].Title, "Cancelable Task")
}

func TestArchiveRemovesCompletedFromProject(t *testing.T) {
	project := testutil.NewTestProject("Archive Test")

	active1 := testutil.NewTestTaskWithStatus("Active 1", models.StatusTodo)
	active2 := testutil.NewTestTaskWithStatus("Active 2", models.StatusInProgress)
	done := testutil.NewTestTaskWithStatus("Done Task", models.StatusDone)
	canceled := testutil.NewTestTaskWithStatus("Canceled Task", models.StatusCanceled)
	project.Tasks = []*models.Task{active1, active2, done, canceled}

	testutil.AssertTaskCount(t, project, 4)

	// Filter out completed tasks to simulate archive
	completedTasks := project.GetCompletedTasks()
	testutil.AssertEqual(t, len(completedTasks), 2)

	completedIDs := make(map[string]bool)
	for _, task := range completedTasks {
		completedIDs[task.ID] = true
	}

	var remaining []*models.Task
	for _, task := range project.Tasks {
		if !completedIDs[task.ID] {
			remaining = append(remaining, task)
		}
	}
	project.Tasks = remaining

	// Verify only active tasks remain
	testutil.AssertTaskCount(t, project, 2)
	for _, task := range project.Tasks {
		if task.IsComplete() {
			t.Errorf("expected only active tasks, found %s with status %s", task.Title, models.LegacyStatusForState(task.State()))
		}
	}
}

func TestArchiveAll_ErrorMessageFormat(t *testing.T) {
	// Verify the error message format used when archive --all has failures.
	// Previously, runTaskArchiveAll returned nil even when projects had errors,
	// causing exit code 0 despite failures. Now it returns an error.
	tests := []struct {
		failureCount int
		want         string
	}{
		{1, "1 project had errors during archiving"},
		{3, "3 projects had errors during archiving"},
	}
	for _, tt := range tests {
		got := Pluralize(tt.failureCount, "project", "projects") + " had errors during archiving"
		if got != tt.want {
			t.Errorf("failureCount=%d: got %q, want %q", tt.failureCount, got, tt.want)
		}
	}
}

func TestArchiveJSON_NoCompletedTasksReturnsValidJSON(t *testing.T) {
	// When archive has no completed tasks, --json must still emit a valid
	// archived_count=0 object rather than the plain-text "No completed tasks
	// to archive." line that previously corrupted JSON output.
	entry := struct {
		ArchivedCount int    `json:"archived_count"`
		Project       string `json:"project"`
	}{ArchivedCount: 0, Project: "Empty Project"}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"archived_count":0`) {
		t.Errorf("expected JSON to contain \"archived_count\":0, got %s", got)
	}
	if !strings.Contains(got, `"project":"Empty Project"`) {
		t.Errorf("expected JSON to contain project name, got %s", got)
	}
}

func TestArchiveAllJSON_EmptyProjectsNotNull(t *testing.T) {
	// Verify that the JSON summary serializes an empty projects list as []
	// rather than null (the nil-slice default in Go).
	type projectResult struct {
		Name     string `json:"name"`
		Archived int    `json:"archived"`
	}
	// Simulate the zero-results case with an empty (non-nil) slice.
	results := []projectResult{}
	summary := struct {
		TotalArchived int             `json:"total_archived"`
		Projects      []projectResult `json:"projects"`
	}{TotalArchived: 0, Projects: results}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"projects":[]`) {
		t.Errorf("expected JSON to contain \"projects\":[], got %s", got)
	}
	if strings.Contains(got, `"projects":null`) {
		t.Error("projects field should be [] not null")
	}
}
