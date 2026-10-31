package app_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func TestChangeHistoryExplainsTaskFieldsWithoutUnchangedNeighbors(t *testing.T) {
	task := newTask("a", 1, "Before title")
	task.WorkflowState = models.StateDoing
	other := newTask("b", 2, "Untouched neighbor")
	p := newProject("p", "Example", task, other)
	before, err := storage.WriteProjectMarkdown(p)
	if err != nil {
		t.Fatal(err)
	}
	task.Title, task.WorkflowState = "After title", models.StateWaiting
	task.Description = "First paragraph\n\nLast paragraph, fully retained."
	due := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	task.DueDate = &due
	after, err := storage.WriteProjectMarkdown(p)
	if err != nil {
		t.Fatal(err)
	}
	changes := app.ChangeHistoryDetails([]storage.LedgerArtifact{{Path: filepath.Join("workspace", "projects", "example", "tasks.md"), BeforeExists: true, Before: []byte(before), AfterExists: true, After: []byte(after)}}, []*models.Project{p})
	fields := map[string]app.HistoryChange{}
	for _, change := range changes {
		if strings.Contains(change.Entity, "Untouched neighbor") {
			t.Fatal("summary includes an unchanged task")
		}
		fields[change.Field] = change
	}
	for _, want := range []struct{ field, before, after string }{{"Title", "Before title", "After title"}, {"Status", "Doing", "Waiting"}, {"Due", "None", "2026-11-05"}, {"Description", "", task.Description}} {
		got := fields[want.field]
		if got.Before != want.before || got.After != want.after {
			t.Errorf("%s = %+v", want.field, got)
		}
	}
	if len(changes) != 4 {
		t.Fatalf("automatic clocks or untouched values cluttered the summary: %+v", changes)
	}
}

func TestChangeHistoryNamesPlanningAndDeletedProjects(t *testing.T) {
	p := newProject("p", "Example", newTask("task", 1, "Write release notes"))
	envelope := func(value any) []byte {
		t.Helper()
		data, err := json.Marshal(map[string]any{"data": value})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	ref := config.ProjectTaskLocator{ProjectID: p.ID, TaskID: p.Tasks[0].ID}
	changes := app.ChangeHistoryDetails([]storage.LedgerArtifact{
		{Path: ".docket/planning.json", BeforeExists: true, AfterExists: true, Before: envelope(config.PlanningState{Today: []config.ProjectTaskLocator{ref}}), After: envelope(config.PlanningState{Next: []config.ProjectTaskLocator{ref}})},
		{Path: ".docket/workspace.json", BeforeExists: true, AfterExists: true, Before: envelope(config.WorkspaceCatalog{Projects: []config.CatalogProject{{ID: "gone", Name: "Deleted project", Slug: "deleted"}}}), After: envelope(config.WorkspaceCatalog{})},
	}, []*models.Project{p})
	if len(changes) != 3 {
		t.Fatalf("incomplete plan/project summary: %+v", changes)
	}
	if changes[0].Entity != "[example] Write release notes" || changes[0].Field != "Today" || changes[0].After != "Removed" || changes[1].Field != "Next" || changes[1].After != "Added" {
		t.Fatalf("planning references unreadable: %+v", changes)
	}
	if changes[2].Entity != "Deleted project" || changes[2].After != "Deleted" {
		t.Fatalf("deleted project disappeared from summary: %+v", changes[2])
	}
}

func TestChangeHistoryKeepsFullNoteAndDegradesUnreadableTaskImages(t *testing.T) {
	text := strings.Repeat("Full note text.\n", 100) + "Last line."
	changes := app.ChangeHistoryDetails([]storage.LedgerArtifact{
		{Path: "projects/example/notes/note.md", AfterExists: true, After: []byte(text)},
		{Path: "projects/example/tasks.md", BeforeExists: true, AfterExists: true, Before: []byte("broken"), After: []byte("broken")},
	}, nil)
	if len(changes) != 2 || changes[0].After != text || !strings.Contains(changes[1].After, "cannot be read") {
		t.Fatalf("details dropped content or hid an unreadable image: %+v", changes)
	}
}

func TestChangeHistoryExplainsCollectionsAndTimedDeadline(t *testing.T) {
	task := newTask("task", 1, "Discuss release")
	target := newTask("target", 2, "Approve release")
	p := newProject("p", "Example", task, target)
	before, err := storage.WriteProjectMarkdown(p)
	if err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 11, 5, 14, 30, 0, 0, time.Local)
	task.DueDate = &due
	task.Checklist = []models.ChecklistItem{{Text: "Read the release notes", Done: true}}
	task.Comments = []*models.Comment{{Text: "Waiting for the final review", CreatedAt: due}}
	task.Attachments = []models.Attachment{{Filename: "notes.txt", OriginalPath: "/source/notes.txt", AddedAt: due}}
	task.Dependencies = []models.TaskReference{{ProjectID: p.ID, TaskID: target.ID}}
	p.Comments = []*models.ProjectComment{{ID: uuid.NewString(), Text: "Ship after approval", CreatedAt: due}}
	after, err := storage.WriteProjectMarkdown(p)
	if err != nil {
		t.Fatal(err)
	}
	changes := app.ChangeHistoryDetails([]storage.LedgerArtifact{{Path: "projects/example/tasks.md", BeforeExists: true, AfterExists: true, Before: []byte(before), After: []byte(after)}}, []*models.Project{p})
	fields := map[string]string{}
	for _, change := range changes {
		fields[change.Field] = change.After
	}
	for field, want := range map[string]string{"Due": "14:30", "Checklist": "[x] Read the release notes", "Comments": "Waiting for the final review", "Attachments": "/source/notes.txt", "Dependencies": "[example] Approve release", "Project comments": "Ship after approval"} {
		if !strings.Contains(fields[field], want) {
			t.Errorf("%s omitted its changed value: %q", field, fields[field])
		}
	}
}

func TestChangeHistoryExplainsCreationRemovalAndProjectMetadata(t *testing.T) {
	task := newTask("task", 1, "New task with content")
	task.Description = "Creation details survive deletion."
	p := newProject("p", "Example", task)
	content, err := storage.WriteProjectMarkdown(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, deleting := range []bool{false, true} {
		image := storage.LedgerArtifact{Path: "projects/example/tasks.md", AfterExists: true, After: []byte(content)}
		if deleting {
			image.BeforeExists, image.Before, image.AfterExists, image.After = true, image.After, false, nil
		}
		changes := app.ChangeHistoryDetails([]storage.LedgerArtifact{image}, nil)
		found := false
		for _, change := range changes {
			found = found || change.Before == task.Description || change.After == task.Description
		}
		if !found {
			t.Fatalf("creation/deletion lost task content: %+v", changes)
		}
	}
	encode := func(project config.CatalogProject) []byte {
		t.Helper()
		data, marshalErr := json.Marshal(map[string]any{"data": config.WorkspaceCatalog{Projects: []config.CatalogProject{project}}})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return data
	}
	a := config.CatalogProject{ID: "p", Name: "Before", Slug: "before"}
	b := a
	b.Name, b.Slug, b.Archived, b.Pinned, b.Description, b.Tags = "After", "after", true, true, "Project purpose", []string{"parked"}
	changes := app.ChangeHistoryDetails([]storage.LedgerArtifact{{Path: ".docket/workspace.json", BeforeExists: true, AfterExists: true, Before: encode(a), After: encode(b)}}, nil)
	if len(changes) != 6 {
		t.Fatalf("project metadata change incomplete: %+v", changes)
	}
}
