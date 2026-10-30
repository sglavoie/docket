package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func TestTUICreationUndoRedoPreservesUnrelatedTaskEdit(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "undo_redo", true: "changed_created_task"}[refuse], func(t *testing.T) {
			dataDir := t.TempDir()
			other := seedTask(t, 1, "Original B", models.StateBacklog)
			project := newLedgerUndoProject(t, dataDir, "creation-undo", other)
			mutator := newLedgerUndoApp(t, dataDir, project, other.ID, true)
			created, err := mutator.domain.service.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Created A"})
			if err != nil {
				t.Fatal(err)
			}
			incoming := restartLedgerProject(project)
			writeIncoming := func(title string) {
				t.Helper()
				if err := storage.LoadProject(incoming); err != nil {
					t.Fatal(err)
				}
				incoming.GetTaskByID(other.ID).Title = title
				if refuse {
					incoming.GetTaskByID(created.TaskID).Description = "Peer's work on A"
				}
				content, err := storage.WriteProjectMarkdown(incoming)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(storage.TasksFilePath(project.Path), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			writeIncoming("Incoming B")
			restarted := newLedgerUndoApp(t, dataDir, restartLedgerProject(project), created.TaskID, false)
			driver := &UITestDriver{app: restarted}
			driver.dispatch(syntheticKey('u'))
			driver.drain()
			if err := storage.LoadProject(incoming); err != nil {
				t.Fatal(err)
			}
			if got := incoming.GetTaskByID(other.ID).Title; got != "Incoming B" {
				t.Fatalf("undo lost B: %q", got)
			}
			if refuse {
				if task := incoming.GetTaskByID(created.TaskID); task == nil || task.Description != "Peer's work on A" {
					t.Fatalf("refused undo lost A: %+v", task)
				}
				if frame := driver.Render(); !strings.Contains(frame, "Undo unavailable") || !strings.Contains(frame, "changed outside this history") {
					t.Fatalf("refusal feedback missing:\n%s", frame)
				}
				return
			}
			if incoming.GetTaskByID(created.TaskID) != nil {
				t.Fatal("undo retained A")
			}
			writeIncoming("Later incoming B")
			driver.dispatch(syntheticCtrlKey('r'))
			driver.drain()
			if err := storage.LoadProject(incoming); err != nil {
				t.Fatal(err)
			}
			if task := incoming.GetTaskByID(created.TaskID); task == nil || task.Title != "Created A" {
				t.Fatalf("redo did not restore A: %+v", task)
			}
			if got := incoming.GetTaskByID(other.ID).Title; got != "Later incoming B" {
				t.Fatalf("redo lost B: %q", got)
			}
		})
	}
}

func TestTUICreationUndoRefusesIncomingPlanningOnlyChange(t *testing.T) {
	driver, workspace, project := newQuickCaptureInterruptFixture(t)
	openFixtureOutliner(t, driver, project)
	runChecklistActions(t, driver, []UITestAction{
		{Type: "key", Key: "a"},
		{Type: "text", Text: "Peer planned creation $backlog"},
		{Type: "key", Key: "enter"},
	})
	incoming := restartLedgerProject(project)
	if err := storage.LoadProject(incoming); err != nil {
		t.Fatal(err)
	}
	createdID := ""
	for _, task := range incoming.Tasks {
		if task.Title == "Peer planned creation" {
			createdID = task.ID
		}
	}
	if createdID == "" {
		t.Fatalf("capture did not create A:\n%s", driver.Render())
	}
	before := readTasksFile(t, incoming)
	store := config.NewDomainFileStore(workspace.DataDir)
	planning, err := store.LoadPlanning()
	if err != nil {
		t.Fatal(err)
	}
	planning.AddToToday(createdID, project.ID)
	if err := store.SavePlanning(planning); err != nil {
		t.Fatal(err)
	}
	runChecklistActions(t, driver, []UITestAction{{Type: "key", Key: "u"}})
	if got := readTasksFile(t, incoming); string(got) != string(before) {
		t.Fatal("undo removed a peer-planned task whose task record was unchanged")
	}
	if feedback := driver.app.feedback.err; feedback == nil || !strings.Contains(feedback.Error(), "Undo unavailable") || !strings.Contains(feedback.Error(), "planning.json") {
		t.Fatalf("missing planning refusal: %v", feedback)
	}
}

func TestTUICaptureKeysUndoRedoAfterIncomingEdits(t *testing.T) {
	driver, _, project := newQuickCaptureInterruptFixture(t)
	openFixtureOutliner(t, driver, project)
	otherID := project.Tasks[0].ID
	runChecklistActions(t, driver, []UITestAction{
		{Type: "key", Key: "a"},
		{Type: "text", Text: "Creation undo key test"},
		{Type: "key", Key: "enter"},
	})
	incoming := restartLedgerProject(project)
	if err := storage.LoadProject(incoming); err != nil {
		t.Fatal(err)
	}
	createdID := ""
	for _, task := range incoming.Tasks {
		if task.Title == "Creation undo key test" {
			createdID = task.ID
		}
	}
	if createdID == "" {
		t.Fatalf("capture keys did not create A:\n%s", driver.Render())
	}
	for _, step := range []struct{ key, peerTitle string }{{"u", "Incoming B"}, {"ctrl+r", "Later incoming B"}} {
		incoming.GetTaskByID(otherID).Title = step.peerTitle
		content, err := storage.WriteProjectMarkdown(incoming)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(storage.TasksFilePath(project.Path), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		runChecklistActions(t, driver, []UITestAction{
			{Type: "key", Key: step.key},
			{Type: "assert", Assert: "no_layout_issues"},
		})
		if err := storage.LoadProject(incoming); err != nil {
			t.Fatal(err)
		}
		if exists := incoming.GetTaskByID(createdID) != nil; exists != (step.key == "ctrl+r") {
			t.Fatalf("after %s: created task exists=%v:\n%s", step.key, exists, driver.Render())
		}
		if got := incoming.GetTaskByID(otherID).Title; got != step.peerTitle {
			t.Fatalf("after %s: peer title = %q, want %q", step.key, got, step.peerTitle)
		}
	}
}
