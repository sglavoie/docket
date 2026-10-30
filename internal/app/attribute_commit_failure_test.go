package app_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func TestSingleAttributeFailureRestoresProject(t *testing.T) {
	due := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	for _, action := range []string{"priority", "due", "clear due", "pin", "unpin"} {
		t.Run(action, func(t *testing.T) {
			task := newTask("task", 1, "Task")
			task.Priority = 3
			if action == "clear due" {
				task.DueDate = &due
			}
			if action == "unpin" {
				task.PinAt(due.Add(-time.Hour))
			}
			project := newProject("alpha", "Alpha", task)
			repo := newFakeRepo(project)
			before := documentOf(project, nil)
			repo.commitErr = errors.New("disk write failed")
			svc := newTestService(repo)
			locator := repo.loc(task.ID)
			var err error
			switch action {
			case "priority":
				_, err = svc.SetPriority(locator, 1)
			case "due":
				_, err = svc.SetDueDate(locator, &due)
			case "clear due":
				_, err = svc.SetDueDate(locator, nil)
			case "pin":
				_, err = svc.SetPinned(locator, true)
			case "unpin":
				_, err = svc.SetPinned(locator, false)
			}
			if !errors.Is(err, repo.commitErr) {
				t.Fatalf("error = %v, want commit failure", err)
			}
			if project.Tasks[0] != task || documentOf(project, nil).Revision != before.Revision || len(repo.commits) != 0 {
				t.Fatal("failed attribute save changed the live project or committed a transaction")
			}

			// A later successful save must not persist the failed attribute change.
			repo.commitErr = nil
			if _, err := svc.AddComment(locator, "Unrelated comment"); err != nil {
				t.Fatal(err)
			}
			stored := models.CloneTaskDeep(repo.documents[project.ID].Tasks[0])
			stored.Comments = nil
			stored.UpdatedAt = before.Tasks[0].UpdatedAt
			if !reflect.DeepEqual(stored, models.CloneTaskDeep(before.Tasks[0])) {
				t.Fatal("a later save persisted the failed attribute change")
			}
		})
	}
}

func TestPinnedReorderFailureRestoresEveryProject(t *testing.T) {
	for _, chain := range []bool{false, true} {
		for _, crossProject := range []bool{false, true} {
			t.Run(fmt.Sprintf("chain=%t/cross-project=%t", chain, crossProject), func(t *testing.T) {
				at := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
				a := pinnedTask("a", 1, "Alpha", at)
				b := pinnedTask("b", 2, "Beta", at.Add(-time.Hour))
				alpha := newProject("alpha", "Alpha", a)
				beta := alpha
				if crossProject {
					beta = newProject("beta", "Beta", b)
				} else {
					alpha.Tasks = append(alpha.Tasks, b)
					alpha.BuildTaskTree()
				}
				projects := []*models.Project{alpha}
				if crossProject {
					projects = append(projects, beta)
				}
				repo := newFakeRepo(projects...)
				beforeA, beforeB := documentOf(alpha, nil).Revision, documentOf(beta, nil).Revision
				repo.commitErr = errors.New("disk write failed")
				svc := newTestService(repo)
				locA := app.TaskLocator{ProjectID: alpha.ID, TaskID: a.ID}
				locB := app.TaskLocator{ProjectID: beta.ID, TaskID: b.ID}
				var err error
				if chain {
					_, err = svc.ReorderPinnedTasksOrder(locA, []app.TaskLocator{locB, locA})
				} else {
					_, err = svc.ReorderPinnedTasks(locA, locB)
				}
				if !errors.Is(err, repo.commitErr) {
					t.Fatalf("error = %v, want commit failure", err)
				}
				if !a.PinnedAt.Equal(at) || !b.PinnedAt.Equal(at.Add(-time.Hour)) ||
					documentOf(alpha, nil).Revision != beforeA || documentOf(beta, nil).Revision != beforeB || len(repo.commits) != 0 {
					t.Fatal("failed reorder changed pin timestamps or project content")
				}
			})
		}
	}
}
