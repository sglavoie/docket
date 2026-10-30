package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func TestProjectCommentsPanelUndoRedoPreservesIncomingWork(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)
	runChecklistActions(t, driver, []UITestAction{
		{Type: "key", Key: ";"},
		{Type: "key", Key: "a"},
		{Type: "text", Text: "Local comment"},
		{Type: "key", Key: "ctrl+s"},
		{Type: "wait"},
	})
	path := filepath.Join(workspace.DataDir, "projects", projectCommentFixtureSlug, "tasks.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := storage.ParseProjectDocument(string(data))
	if err != nil {
		t.Fatal(err)
	}
	doc.Tasks[0].Title = "Incoming task edit"
	doc.Comments = append(doc.Comments, &models.ProjectComment{
		ID: "00000000-0000-4000-8000-000000000001", Text: "Incoming comment", CreatedAt: doc.Comments[0].CreatedAt,
	})
	content, err := doc.WriteCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), storage.FilePermissions); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		key   string
		count int
	}{
		{"u", 1}, {"ctrl+r", 2}, {"u", 1},
	} {
		runChecklistActions(t, driver, []UITestAction{
			{Type: "key", Key: step.key},
			{Type: "wait"},
			{Type: "assert", Assert: "no_layout_issues"},
		})
		comments := projectCommentsOnDisk(t, workspace)
		if len(comments) != step.count {
			t.Fatalf("after %s: stored comments = %+v", step.key, comments)
		}
		commentByID(t, comments, "00000000-0000-4000-8000-000000000001")
		panel := mountedProjectCommentsPanel(t, driver.App())
		if len(panel.Entries()) != step.count {
			t.Fatalf("after %s: panel did not refresh: %v", step.key, panelCommentTexts(panel))
		}
		if step.count == 1 && !reflect.DeepEqual(panelCommentTexts(panel), []string{"Incoming comment"}) {
			t.Fatalf("undo lost incoming comment: %v", panelCommentTexts(panel))
		}
		owner := driver.App().projectByID(projectCommentFixtureSlug)
		got, err := storage.LoadProjectDocument(owner)
		if err != nil {
			t.Fatal(err)
		}
		if got.Tasks[0].Title != "Incoming task edit" || owner.Tasks[0].Title != "Incoming task edit" {
			t.Fatal("restore lost incoming task edit on disk or in the UI")
		}
	}
}
