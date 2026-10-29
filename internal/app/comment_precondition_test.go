package app_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func TestCommentExpectingRejectsStaleTargets(t *testing.T) {
	for _, action := range []string{"edit", "delete"} {
		for _, change := range []string{"shifted", "text changed", "timestamp changed", "removed"} {
			t.Run(action+"/"+change, func(t *testing.T) {
				task := newTask("t1", 1, "Commented")
				at := task.CreatedAt
				task.Comments = []*models.Comment{
					{Text: "first", CreatedAt: at},
					{Text: "second", CreatedAt: at.Add(time.Second)},
					{Text: "third", CreatedAt: at.Add(2 * time.Second)},
				}
				expected := *task.Comments[1]
				switch change {
				case "shifted":
					task.Comments = task.Comments[1:]
				case "text changed":
					task.Comments[1].Text = "changed externally"
				case "timestamp changed":
					task.Comments[1].CreatedAt = at.Add(time.Hour)
				case "removed":
					task.Comments = task.Comments[:1]
				}
				repo := newFakeRepo(newProject("p1", "Alpha", task))
				svc := newTestService(repo)
				before := models.CloneTaskDeep(task)
				var err error
				if action == "edit" {
					_, err = svc.EditComment(repo.loc("t1"), 1, expected, "my replacement")
				} else {
					_, err = svc.RemoveComment(repo.loc("t1"), 1, expected)
				}
				if !errors.Is(err, app.ErrInvalidInput) {
					t.Fatalf("err = %v, want stale-target refusal", err)
				}
				if len(repo.commits) != 0 {
					t.Fatal("stale target recorded a transaction")
				}
				if !reflect.DeepEqual(before, models.CloneTaskDeep(task)) {
					t.Fatal("stale target changed task content or clocks")
				}
			})
		}
	}
}

func TestCommentExpectingAcceptsTheSameInstantAndPreservesVerbatimText(t *testing.T) {
	for _, action := range []string{"edit", "delete"} {
		t.Run(action, func(t *testing.T) {
			task := newTask("t1", 1, "Commented")
			at := task.CreatedAt
			expected := models.Comment{Text: "  original\ttext  ", CreatedAt: at}
			task.Comments = []*models.Comment{{Text: expected.Text, CreatedAt: at.In(time.FixedZone("device", -6*3600))}}
			repo := newFakeRepo(newProject("p1", "Alpha", task))
			svc := newTestService(repo)
			var err error
			if action == "edit" {
				_, err = svc.EditComment(repo.loc("t1"), 0, expected, "  replacement\ttext  ")
				if err == nil && (task.Comments[0].Text != "  replacement\ttext  " || !task.Comments[0].CreatedAt.Equal(at)) {
					t.Fatal("edit changed the text or original creation instant")
				}
			} else {
				_, err = svc.RemoveComment(repo.loc("t1"), 0, expected)
				if err == nil && len(task.Comments) != 0 {
					t.Fatal("confirmed comment survived deletion")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(repo.commits) != 1 {
				t.Fatalf("commits = %d, want one", len(repo.commits))
			}
		})
	}
}
