package app_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestDependencyPreservationAcrossEditPaths pins the rule that keeps soft
// references safe on surfaces that cannot express them. Every save of an
// existing task funnels through EditTaskLine, whose Dependencies patch is
// preserve-by-default: a zero patch is an absence of intent, not an empty set.
//
// This matters most for the external-editor round trip, which returns a title
// and a body and nothing else, and for the expanded task form, which is
// create-only and therefore never round-trips a reference at all. Both would
// silently erase a user's references if a zero patch meant "store nothing".
func TestDependencyPreservationAcrossEditPaths(t *testing.T) {
	stored := []string{"proj-a:sibling", "proj-b:remote"}

	seed := func(t *testing.T) (*fakeRepo, *app.Service, *models.Task) {
		t.Helper()
		repo, svc, source, sibling, remote := dependencyFixture()
		if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{
			repo.loc(sibling.ID),
			repo.loc(remote.ID),
		}); err != nil {
			t.Fatalf("AddTaskDependencies: %v", err)
		}
		return repo, svc, source
	}

	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	body := "A body the external editor returned."
	priority := 2
	pinned := true

	cases := []struct {
		name    string
		request func(repo *fakeRepo, source *models.Task) app.TaskLineEditRequest
		project string
	}{
		{
			name: "an inline title edit that never mentions a reference",
			request: func(repo *fakeRepo, source *models.Task) app.TaskLineEditRequest {
				return app.TaskLineEditRequest{Source: repo.loc(source.ID), Title: "Retitled"}
			},
			project: "proj-a",
		},
		{
			name: "an external-editor round trip returning a title and a body",
			request: func(repo *fakeRepo, source *models.Task) app.TaskLineEditRequest {
				return app.TaskLineEditRequest{
					Source:      repo.loc(source.ID),
					Title:       "Retitled from the editor",
					Description: &body,
				}
			},
			project: "proj-a",
		},
		{
			name: "an edit rewriting every other patchable field",
			request: func(repo *fakeRepo, source *models.Task) app.TaskLineEditRequest {
				return app.TaskLineEditRequest{
					Source:      repo.loc(source.ID),
					Title:       "Fully repatched",
					Priority:    &priority,
					DueSet:      true,
					Due:         &due,
					Pinned:      &pinned,
					TagsToAdd:   []string{"alpha"},
					Destination: taskLineDestination(t, "waiting"),
				}
			},
			project: "proj-a",
		},
		{
			name: "a move to another project, which rewrites the task's own address",
			request: func(repo *fakeRepo, source *models.Task) app.TaskLineEditRequest {
				return app.TaskLineEditRequest{
					Source:               repo.loc(source.ID),
					Title:                "Moved",
					DestinationProjectID: "proj-b",
				}
			},
			project: "proj-b",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, svc, source := seed(t)
			result, err := svc.EditTaskLine(testCase.request(repo, source))
			if err != nil {
				t.Fatalf("EditTaskLine: %v", err)
			}
			if !result.Changed {
				t.Fatal("edit reported no change, so it proves nothing about preservation")
			}
			assertDependencies(t, findTask(t, repo, testCase.project, source.ID), stored...)
		})
	}
}

// An explicit patch is the only thing that rewrites the set, and it is the same
// request field the preserving edits above left zero. Without this the test
// above would pass over a request struct that had simply lost the ability to
// carry an intent.
func TestDependencyPreservationYieldsToAnExplicitPatch(t *testing.T) {
	repo, svc, source, sibling, remote := dependencyFixture()
	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{
		repo.loc(sibling.ID),
		repo.loc(remote.ID),
	}); err != nil {
		t.Fatalf("AddTaskDependencies: %v", err)
	}

	if _, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:       repo.loc(source.ID),
		Title:        "Cleared",
		Dependencies: app.DependencyPatch{Clear: true},
	}); err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	assertDependencies(t, findTask(t, repo, "proj-a", source.ID))
}
