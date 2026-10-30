package app_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

func TestProjectCommentFailureRestoresCollection(t *testing.T) {
	for _, action := range []string{"add", "edit", "remove"} {
		t.Run(action, func(t *testing.T) {
			now := projectCommentFixtureTime()
			project := twoCommentProject(now)
			before := models.CloneProjectComments(project.Comments)
			repo := newFakeRepo(project)
			repo.commitErr = errors.New("disk write failed")
			svc := newProjectCommentService(repo, now)
			var err error
			switch action {
			case "add":
				_, err = svc.AddProjectComment(project.ID, "New comment")
			case "edit":
				_, err = svc.EditProjectComment(editTarget(firstCommentID, "Older"), "Replacement")
			case "remove":
				_, err = svc.RemoveProjectComment(editTarget(firstCommentID, "Older"))
			}
			if !errors.Is(err, repo.commitErr) {
				t.Fatalf("error = %v, want commit failure", err)
			}
			if !reflect.DeepEqual(project.Comments, before) || len(repo.commits) != 0 {
				t.Fatal("failed project-comment save changed the live collection")
			}
			repo.commitErr = nil
			if _, err := svc.AddProjectComment(project.ID, "Later comment"); err != nil {
				t.Fatal(err)
			}
			stored := repo.documents[project.ID].Comments
			if len(stored) != len(before)+1 || !reflect.DeepEqual(stored[:len(before)], before) {
				t.Fatal("a later save persisted the failed comment mutation")
			}
		})
	}
}
