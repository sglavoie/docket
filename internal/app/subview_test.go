package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestAddCommentAppendsAndCommits(t *testing.T) {
	task := newTask("t1", 1, "Commented")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	res, err := svc.AddComment(repo.loc("t1"), "  root cause found  ")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if len(task.Comments) != 1 || task.Comments[0].Text != "root cause found" {
		t.Fatalf("comments = %+v, want one trimmed comment", task.Comments)
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Add Comment" {
		t.Fatalf("lastCommit = %+v ok=%v, want Add Comment", cs, ok)
	}
	if res.Task.TaskID != "t1" || res.Task.Revision == "" {
		t.Fatalf("task result = %+v, want t1 with revision", res.Task)
	}
}

func TestAddCommentRejectsEmptyText(t *testing.T) {
	repo := newFakeRepo(newProject("p1", "Alpha", newTask("t1", 1, "Task")))
	svc := newTestService(repo)

	_, err := svc.AddComment(repo.loc("t1"), "   ")
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Fatal("empty comment must not commit")
	}
}

func TestEditCommentPreservesTimestamp(t *testing.T) {
	task := newTask("t1", 1, "Commented")
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	task.Comments = []*models.Comment{{Text: "old", CreatedAt: created}}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	_, err := svc.EditComment(repo.loc("t1"), 0, "new text")
	if err != nil {
		t.Fatalf("EditComment: %v", err)
	}
	if task.Comments[0].Text != "new text" || !task.Comments[0].CreatedAt.Equal(created) {
		t.Fatalf("comment = %+v, want new text with preserved timestamp", task.Comments[0])
	}

	if _, err := svc.EditComment(repo.loc("t1"), 5, "text"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("out-of-range err = %v, want ErrInvalidInput", err)
	}
}

func TestRemoveComments(t *testing.T) {
	task := newTask("t1", 1, "Commented")
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	task.Comments = []*models.Comment{{Text: "first", CreatedAt: created}, {Text: "second", CreatedAt: created}, {Text: "third", CreatedAt: created}}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	_, err := svc.RemoveComment(repo.loc("t1"), 0)
	if err != nil {
		t.Fatalf("RemoveComment: %v", err)
	}
	if len(task.Comments) != 2 || task.Comments[0].Text != "second" {
		t.Fatalf("after remove: comments=%+v", task.Comments)
	}

	if _, rangeErr := svc.RemoveComment(repo.loc("t1"), 9); !errors.Is(rangeErr, app.ErrInvalidInput) {
		t.Fatalf("out-of-range err = %v, want ErrInvalidInput", rangeErr)
	}
}

func TestCommentCommitFailureRestoresTask(t *testing.T) {
	task := newTask("t1", 1, "Commented")
	task.Comments = []*models.Comment{{Text: "original", CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	repo.commitErr = errors.New("commit failed")
	svc := newTestService(repo)

	if _, err := svc.AddComment(repo.loc("t1"), "transient"); err == nil {
		t.Fatal("AddComment unexpectedly succeeded")
	}
	if len(task.Comments) != 1 || task.Comments[0].Text != "original" {
		t.Fatalf("failed commit left comments mutated: %+v", task.Comments)
	}
}
