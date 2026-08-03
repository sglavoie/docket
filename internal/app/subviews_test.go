package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestChecklistCommitFailureRestoresTask(t *testing.T) {
	task := newTask("t1", 1, "Task")
	task.Checklist = []models.ChecklistItem{{Text: "original item"}}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	repo.commitErr = errors.New("commit failed")
	svc := newTestService(repo)

	if _, err := svc.ToggleChecklistItem("t1", 0); err == nil {
		t.Fatal("ToggleChecklistItem unexpectedly succeeded")
	}
	if task.Checklist[0].Done {
		t.Fatalf("failed checklist commit left task mutated: %+v", task.Checklist)
	}
}

func TestChecklistLifecycle(t *testing.T) {
	task := newTask("t1", 1, "Checked")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.AddChecklistItem("t1", "  first  "); err != nil {
		t.Fatalf("AddChecklistItem: %v", err)
	}
	res, err := svc.AddChecklistItem("t1", "second")
	if err != nil {
		t.Fatalf("AddChecklistItem: %v", err)
	}
	if res.Count != 2 || res.Index != 1 {
		t.Fatalf("res = %+v, want count 2 index 1", res)
	}
	if task.Checklist[0].Text != "first" {
		t.Fatalf("checklist = %+v, want trimmed first item", task.Checklist)
	}

	toggled, err := svc.ToggleChecklistItem("t1", 0)
	if err != nil {
		t.Fatalf("ToggleChecklistItem: %v", err)
	}
	if !toggled.Done || !task.Checklist[0].Done {
		t.Fatal("item 0 should be done after toggle")
	}

	if _, editErr := svc.EditChecklistItem("t1", 0, "first edited"); editErr != nil {
		t.Fatalf("EditChecklistItem: %v", editErr)
	}
	if task.Checklist[0].Text != "first edited" || !task.Checklist[0].Done {
		t.Fatalf("item 0 = %+v, want edited text with done preserved", task.Checklist[0])
	}

	if _, reorderErr := svc.ReorderChecklistItem("t1", 0, 1); reorderErr != nil {
		t.Fatalf("ReorderChecklistItem: %v", reorderErr)
	}
	if task.Checklist[0].Text != "second" || task.Checklist[1].Text != "first edited" {
		t.Fatalf("checklist after reorder = %+v, want swapped", task.Checklist)
	}

	removed, err := svc.RemoveChecklistItem("t1", 0)
	if err != nil {
		t.Fatalf("RemoveChecklistItem: %v", err)
	}
	if removed.Count != 1 || len(task.Checklist) != 1 || task.Checklist[0].Text != "first edited" {
		t.Fatalf("checklist after remove = %+v, want only first edited", task.Checklist)
	}
}

func TestChecklistValidation(t *testing.T) {
	task := newTask("t1", 1, "Checked")
	task.Checklist = []models.ChecklistItem{{Text: "only"}}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.AddChecklistItem("t1", "   "); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty text err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.ToggleChecklistItem("t1", 1); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("out-of-range toggle err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.ReorderChecklistItem("t1", 0, 2); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("out-of-range reorder err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Fatal("rejected checklist mutations must not commit")
	}
}

func TestReorderAttachmentSwaps(t *testing.T) {
	task := newTask("t1", 1, "Attached")
	if err := task.AddAttachment("a.txt", false, "/tmp/a.txt"); err != nil {
		t.Fatalf("AddAttachment: %v", err)
	}
	if err := task.AddAttachment("b.txt", false, "/tmp/b.txt"); err != nil {
		t.Fatalf("AddAttachment: %v", err)
	}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.ReorderAttachment("t1", 0, 1); err != nil {
		t.Fatalf("ReorderAttachment: %v", err)
	}
	if task.Attachments[0].Filename != "b.txt" || task.Attachments[1].Filename != "a.txt" {
		t.Fatalf("attachments = %+v, want swapped", task.Attachments)
	}
	if cs, ok := repo.lastCommit(); !ok || cs.UndoAction != "Reorder Attachments" {
		t.Fatalf("lastCommit = %+v ok=%v, want Reorder Attachments", cs, ok)
	}

	if _, err := svc.ReorderAttachment("t1", 0, 5); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("out-of-range reorder err = %v, want ErrInvalidInput", err)
	}
}
