package app_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func TestReorderAttachmentRefusesChangedList(t *testing.T) {
	for _, change := range []string{"reordered", "removed", "replaced", "appended"} {
		t.Run(change, func(t *testing.T) {
			task := newTask("t1", 1, "Attached")
			task.Attachments = []models.Attachment{
				{Filename: "a.txt", AddedAt: time.Now()},
				{Filename: "b.txt", AddedAt: time.Now()},
				{Filename: "c.txt", AddedAt: time.Now()},
			}
			expected := slices.Clone(task.Attachments)
			switch change {
			case "reordered":
				task.Attachments[0], task.Attachments[1] = task.Attachments[1], task.Attachments[0]
			case "removed":
				task.Attachments = task.Attachments[1:]
			case "replaced":
				task.Attachments[0].OriginalPath = "/a/different/file"
			case "appended":
				task.Attachments = append(task.Attachments, models.Attachment{Filename: "d.txt", AddedAt: time.Now()})
			}
			before := slices.Clone(task.Attachments)
			updated := task.UpdatedAt
			repo := newFakeRepo(newProject("p1", "Alpha", task))
			_, err := newTestService(repo).ReorderAttachment(repo.loc("t1"), 0, 1, expected)
			if !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("stale reorder error = %v, want ErrInvalidInput", err)
			}
			if len(repo.commits) != 0 || !slices.Equal(task.Attachments, before) || !task.UpdatedAt.Equal(updated) {
				t.Fatal("stale reorder changed attachments, touch time, or committed a transaction")
			}
		})
	}
}

func TestReorderAttachmentCommitFailureRestoresOrder(t *testing.T) {
	task := newTask("t1", 1, "Attached")
	task.Attachments = []models.Attachment{
		{Filename: "a.txt", AddedAt: time.Now()},
		{Filename: "b.txt", AddedAt: time.Now()},
	}
	before := slices.Clone(task.Attachments)
	updated := task.UpdatedAt
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	repo.commitErr = errors.New("disk write failed")
	_, err := newTestService(repo).ReorderAttachment(repo.loc("t1"), 0, 1, before)
	if !errors.Is(err, repo.commitErr) {
		t.Fatalf("reorder error = %v, want commit error", err)
	}
	if !slices.Equal(task.Attachments, before) || !task.UpdatedAt.Equal(updated) || len(repo.commits) != 0 {
		t.Fatal("failed reorder changed order, touch time, or committed a transaction")
	}
}

func TestReorderAttachmentAcceptsEquivalentInstants(t *testing.T) {
	task := newTask("t1", 1, "Attached")
	task.Attachments = []models.Attachment{
		{Filename: "a.txt", AddedAt: time.Now()},
		{Filename: "b.txt", AddedAt: time.Now()},
	}
	expected := slices.Clone(task.Attachments)
	for i := range expected {
		expected[i].AddedAt = expected[i].AddedAt.In(time.FixedZone("other", -6*60*60))
	}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	if _, err := newTestService(repo).ReorderAttachment(repo.loc("t1"), 0, 1, expected); err != nil {
		t.Fatal(err)
	}
	if task.Attachments[0].Filename != "b.txt" || len(repo.commits) != 1 {
		t.Fatal("unchanged attachments did not swap in one transaction")
	}
}
