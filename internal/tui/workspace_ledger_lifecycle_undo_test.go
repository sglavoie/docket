package tui

import (
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func newStatusUndoFixture(t *testing.T) (*UITestDriver, *uitestWorkspace, *models.Project, *models.Task) {
	t.Helper()
	driver, workspace, project := newQuickCaptureInterruptFixture(t)
	openFixtureOutliner(t, driver, project)
	runChecklistActions(t, driver, []UITestAction{{Type: "key", Key: "a"}, {Type: "text", Text: "Status undo target $backlog"}, {Type: "key", Key: "enter"}})
	incoming := restartLedgerProject(project)
	if err := storage.LoadProject(incoming); err != nil {
		t.Fatal(err)
	}
	var targetTask *models.Task
	for _, task := range incoming.Tasks {
		if task.Title == "Status undo target" {
			targetTask = task
			break
		}
	}
	if targetTask == nil {
		t.Fatal("capture did not create target")
	}
	target := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: targetTask.ID}
	day := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	if _, err := driver.app.domain.service.BulkSetCheckIn([]app.TaskLocator{{ProjectID: target.ProjectID, TaskID: target.TaskID}}, models.CheckInSchedule{Next: &day, RepeatDays: 7}); err != nil {
		t.Fatal(err)
	}
	driver.app.crossOutliner.NavigateToTaskInProject(targetTask, incoming)
	driver.drain()
	runChecklistActions(t, driver, []UITestAction{{Type: "key", Key: "w"}, {Type: "key", Key: "t"}})
	if err := storage.LoadProject(incoming); err != nil {
		t.Fatal(err)
	}
	before := models.CloneTaskDeep(incoming.GetTaskByID(target.TaskID))
	store := config.NewDomainFileStore(workspace.DataDir)
	planned, err := store.LoadPlanningReadOnly()
	if err != nil || !slices.Contains(planned.Today, target) {
		t.Fatalf("setup did not plan target: %+v %v\n%s", planned, err, driver.Render())
	}
	driver.app.crossOutliner.NavigateToTaskInProject(before, incoming)
	driver.drain()
	runChecklistActions(t, driver, []UITestAction{{Type: "key", Key: "w"}, {Type: "key", Key: "d"}})
	if err := storage.LoadProject(incoming); err != nil {
		t.Fatal(err)
	}
	if !incoming.GetTaskByID(target.TaskID).IsComplete() {
		t.Fatalf("setup did not complete target\n%s", driver.Render())
	}
	return driver, workspace, incoming, before
}

func TestTUIStatusKeysPreserveSameTaskPeerEditsOnUndoRedo(t *testing.T) {
	driver, workspace, incoming, before := newStatusUndoFixture(t)
	target := config.ProjectTaskLocator{ProjectID: incoming.ID, TaskID: before.ID}
	store := config.NewDomainFileStore(workspace.DataDir)
	for step, key := range []string{"u", "ctrl+r", "u"} {
		if loadErr := storage.LoadProject(incoming); loadErr != nil {
			t.Fatal(loadErr)
		}
		task := incoming.GetTaskByID(target.TaskID)
		title := fmt.Sprintf("Peer title %d", step)
		task.Title, task.Description, task.Priority = title, "Peer description", 1
		task.UpdatedAt = task.UpdatedAt.Add(time.Hour)
		content, err := storage.WriteProjectMarkdown(incoming)
		if err != nil {
			t.Fatal(err)
		}
		if writeErr := os.WriteFile(storage.TasksFilePath(incoming.Path), []byte(content), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		runChecklistActions(t, driver, []UITestAction{{Type: "key", Key: key}, {Type: "assert", Assert: "no_layout_issues"}})
		if loadErr := storage.LoadProject(incoming); loadErr != nil {
			t.Fatal(loadErr)
		}
		got := incoming.GetTaskByID(target.TaskID)
		state := before.State()
		if key == "ctrl+r" {
			state = models.StateDone
		}
		if got.Title != title || got.Description != "Peer description" || got.Priority != 1 || got.State() != state {
			t.Fatalf("after %s: task = %+v", key, got)
		}
		if key == "u" && (!got.CheckIn().Equal(before.CheckIn()) || !got.StateChangedAt.Equal(before.StateChangedAt)) {
			t.Fatalf("Undo did not restore schedule and state clock: %+v", got)
		}
		if key == "ctrl+r" && (got.NextCheckIn != nil || got.CompletedAt == nil) {
			t.Fatalf("Redo did not complete and clear schedule: %+v", got)
		}
		planning, err := store.LoadPlanningReadOnly()
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(planning.Today, target) != (key == "u") {
			t.Fatalf("after %s: planning = %+v", key, planning)
		}
		live := driver.app.domain.runtime.AvailableProjects()
		for _, project := range live {
			if task := project.GetTaskByID(target.TaskID); task != nil && (task.Title != title || task.State() != state) {
				t.Fatalf("after %s: runtime did not adopt restored task: %+v", key, task)
			}
		}
	}
}
