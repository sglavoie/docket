package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestProjectServiceSaveWithBackupSyncOptionsRecordsUndoSnapshot(t *testing.T) {
	project, err := models.NewProject("proj", "proj", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("task-id", 1, models.TaskTypeTask, "Before")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()
	if err = storage.SaveProject(project); err != nil {
		t.Fatalf("seed save failed: %v", err)
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.Title = "After"
	})
	svc := NewProjectService()
	err = svc.SaveWithBackupSyncOptions(project, ProjectSaveOptions{
		UndoAction:          "Rename Task",
		UndoContext:         "#1 Before",
		UndoDepth:           5,
		UndoTaskIDs:         []string{task.ID},
		UndoGroupID:         "group-1",
		UndoGroupProjectIDs: []string{project.ID},
	})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	action, err := storage.PerformUndo(project)
	if err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if action != "Rename Task: #1 Before" {
		t.Fatalf("undo action = %q, want %q", action, "Rename Task: #1 Before")
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load after undo failed: %v", err)
	}
	restored := project.GetTaskByID(task.ID)
	if restored == nil {
		t.Fatal("restored task missing")
	}
	if restored.Title != "Before" {
		t.Errorf("restored title = %q, want %q", restored.Title, "Before")
	}
}

func TestProjectSaveOptionsFromTaskPropertyMsg(t *testing.T) {
	depth := 7
	app := &AppModel{config: &config.Config{UndoHistoryDepth: &depth}}
	task := &models.Task{ID: "task-id", ShortID: 1, Title: "Task"}

	opts := app.projectSaveOptionsFromTaskPropertyMsg(TaskPropertyChangedMsg{
		Task:                task,
		UndoAction:          "Toggle Pin",
		UndoContext:         "#1 Task",
		UndoGroupID:         "group-1",
		UndoGroupProjectIDs: []string{"proj-1", "proj-2"},
	})

	if opts.UndoDepth != depth {
		t.Errorf("UndoDepth = %d, want %d", opts.UndoDepth, depth)
	}
	if opts.UndoAction != "Toggle Pin" || opts.UndoContext != "#1 Task" {
		t.Errorf("undo metadata = %q / %q", opts.UndoAction, opts.UndoContext)
	}
	if len(opts.UndoTaskIDs) != 1 || opts.UndoTaskIDs[0] != task.ID {
		t.Errorf("UndoTaskIDs = %v, want [%q]", opts.UndoTaskIDs, task.ID)
	}
	if opts.UndoGroupID != "group-1" {
		t.Errorf("UndoGroupID = %q, want group-1", opts.UndoGroupID)
	}
	if len(opts.UndoGroupProjectIDs) != 2 || opts.UndoGroupProjectIDs[0] != "proj-1" || opts.UndoGroupProjectIDs[1] != "proj-2" {
		t.Errorf("UndoGroupProjectIDs = %v", opts.UndoGroupProjectIDs)
	}
}
