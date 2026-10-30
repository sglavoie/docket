package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
)

func TestTUIDeletionKeysUndoRedoAfterIncomingEdits(t *testing.T) {
	for _, peerPlansTarget := range []bool{false, true} {
		t.Run(map[bool]string{false: "undo_redo", true: "planned_target"}[peerPlansTarget], func(t *testing.T) {
			driver, workspace, project := newQuickCaptureInterruptFixture(t)
			openFixtureOutliner(t, driver, project)
			otherID := project.Tasks[0].ID
			runChecklistActions(t, driver, []UITestAction{
				{Type: "key", Key: "a"},
				{Type: "text", Text: "Deletion undo key test $backlog"},
				{Type: "key", Key: "enter"},
			})
			incoming := restartLedgerProject(project)
			if err := storage.LoadProject(incoming); err != nil {
				t.Fatal(err)
			}
			targetID := ""
			for _, task := range incoming.Tasks {
				if task.Title == "Deletion undo key test" {
					targetID = task.ID
				}
			}
			if targetID == "" {
				t.Fatal("capture did not create deletion target")
			}
			out, ok := driver.app.outlinerPrimary()
			if !ok {
				t.Fatal("no outliner")
			}
			out.MoveCursorToTask(targetID)
			runChecklistActions(t, driver, []UITestAction{
				{Type: "key", Key: "space"},
				{Type: "key", Key: "j"},
				{Type: "key", Key: "enter"},
				{Type: "assert", Assert: "contains", Value: "Delete task"},
				{Type: "key", Key: "y"},
			})
			if err := storage.LoadProject(incoming); err != nil {
				t.Fatal(err)
			}
			if incoming.GetTaskByID(targetID) != nil {
				t.Fatal("delete keys retained target")
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
				if peerPlansTarget && step.key == "ctrl+r" {
					store := config.NewDomainFileStore(workspace.DataDir)
					planning, err := store.LoadPlanning()
					if err != nil {
						t.Fatal(err)
					}
					planning.AddToToday(targetID, project.ID)
					if err := store.SavePlanning(planning); err != nil {
						t.Fatal(err)
					}
				}
				runChecklistActions(t, driver, []UITestAction{
					{Type: "key", Key: step.key},
					{Type: "assert", Assert: "no_layout_issues"},
				})
				if err := storage.LoadProject(incoming); err != nil {
					t.Fatal(err)
				}
				if exists := incoming.GetTaskByID(targetID) != nil; exists != (step.key == "u" || peerPlansTarget) {
					t.Fatalf("after %s: target exists=%v:\n%s", step.key, exists, driver.Render())
				}
				if got := incoming.GetTaskByID(otherID).Title; got != step.peerTitle {
					t.Fatalf("after %s: peer title = %q, want %q", step.key, got, step.peerTitle)
				}
			}
			if peerPlansTarget {
				if feedback := driver.app.feedback.err; feedback == nil || !strings.Contains(feedback.Error(), "Redo unavailable") || !strings.Contains(feedback.Error(), "planning.json") {
					t.Fatalf("missing planning refusal: %v", feedback)
				}
			}
		})
	}
}
