package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// This file is the hidden flag's preservation contract. The flag has no sigil,
// so no editor can spell it: every edit surface reaches storage through a
// request that cannot name it, and each one must therefore leave it exactly as
// it found it. The paths are enumerated rather than sampled, because each writes
// the task through different code.

func hiddenTask(id, title string) *models.Task {
	task := newTask(id, 1, title)
	task.Hidden = true
	return task
}

func requireHidden(t *testing.T, repo *fakeRepo, projectID, taskID, where string) {
	t.Helper()
	if !persistedTask(t, repo, projectID, taskID).Hidden {
		t.Fatalf("%s: the stored hidden flag was dropped", where)
	}
}

// TestEveryEditSurfacePreservesTheHiddenFlag names, for each case, a field the
// mutation does write; none names the flag.
func TestEveryEditSurfacePreservesTheHiddenFlag(t *testing.T) {
	newTitle := "Renamed"
	newBody := "Body rewritten in the external editor."
	for name, mutate := range map[string]func(*app.Service, *fakeRepo, app.TaskLocator) error{
		// The expanded form and the external editor's body both land through Edit.
		"edit the title and body": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.Edit(app.EditRequest{Locator: locator, Title: &newTitle, Description: &newBody})
			return err
		},
		"transition the state": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetState, Targets: []app.TaskLocator{locator}, State: models.StateDoing,
			})
			return err
		},
		"complete the task": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetState, Targets: []app.TaskLocator{locator}, State: models.StateDone,
			})
			return err
		},
		"pin the task": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.BulkSetPinned([]app.TaskLocator{locator}, true)
			return err
		},
		"replace the tags": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetTags, Targets: []app.TaskLocator{locator}, Tags: []string{"infra"},
			})
			return err
		},
		"reorder within the project": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.MoveTaskToBottom(locator)
			return err
		},
		"reparent and detach": func(service *app.Service, repo *fakeRepo, locator app.TaskLocator) error {
			parent := repo.loc("other").TaskID
			if _, err := service.Reparent(locator, &parent); err != nil {
				return err
			}
			_, err := service.Reparent(locator, nil)
			return err
		},
		// Quick edit (`e`) and the external-editor round trip share this request.
		"edit the whole task line": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			destination := app.CaptureDestinationBacklog
			_, err := service.EditTaskLine(app.TaskLineEditRequest{
				Source: locator, Title: newTitle, Description: &newBody,
				Destination: &destination, TagsToAdd: []string{"urgent"},
			})
			return err
		},
		// The outline buffer restates a task's whole line, so an absent field is a
		// clear there; the flag is not a field it holds.
		"apply an outline buffer": func(service *app.Service, repo *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ApplyOutlinePlan(app.OutlinePlanRequest{
				ProjectID:        locator.ProjectID,
				ExpectedRevision: repo.revisions[locator.ProjectID],
				ScopeTaskIDs:     []string{"quiet", "other"},
				Lines: []app.OutlinePlanLine{
					{SourceLine: 1, TaskID: "other", Depth: 0, Title: "Other"},
					{SourceLine: 2, TaskID: "quiet", Depth: 1, Title: newTitle, Tags: []string{"infra"}},
				},
			})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepo(newProject("project", "Project", hiddenTask("quiet", "Quiet"), newTask("other", 2, "Other")))
			if err := mutate(newTestService(repo), repo, repo.loc("quiet")); err != nil {
				t.Fatalf("mutation: %v", err)
			}
			requireHidden(t, repo, "project", "quiet", name)
			if persistedTask(t, repo, "project", "other").Hidden {
				t.Fatalf("%s: the flag spread to an untargeted task", name)
			}
		})
	}
}

// TestMovesAcrossProjectsPreserveTheHiddenFlag covers both writers that remove
// a task from one document and write it into another, including a descendant the
// selection never named.
func TestMovesAcrossProjectsPreserveTheHiddenFlag(t *testing.T) {
	root := hiddenTask("root", "Root")
	child := childOf("child", 2, "Child", root)
	child.Hidden = true
	repo := newFakeRepo(newProject("source", "Source", root, child), newProject("destination", "Destination"))
	if _, err := newTestService(repo).BulkMoveToProject(repoLocators(repo, []string{"root"}), "destination"); err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}
	requireHidden(t, repo, "destination", "root", "bulk move")
	requireHidden(t, repo, "destination", "child", "bulk move of an unnamed child")

	single := newFakeRepo(newProject("proj-a", "Alpha", hiddenTask("t1", "Quiet")), newProject("proj-b", "Beta"))
	if _, err := newTestService(single).EditTaskLine(app.TaskLineEditRequest{
		Source: single.loc("t1"), Title: "Quiet", DestinationProjectID: "proj-b",
	}); err != nil {
		t.Fatalf("EditTaskLine across projects: %v", err)
	}
	requireHidden(t, single, "proj-b", "t1", "cross-project line edit")
}

// TestRepeatDraftDoesNotCopyTheHiddenFlag is repetition's boundary: `D` creates
// fresh work, which is visible, and the hidden source keeps its own flag.
func TestRepeatDraftDoesNotCopyTheHiddenFlag(t *testing.T) {
	repo := newFakeRepo(newProject("proj-a", "Alpha", hiddenTask("t1", "Quarterly report")))
	svc := newTestService(repo)
	draft, err := svc.PrepareTaskRepeatDraft(repo.loc("t1"))
	if err != nil {
		t.Fatalf("PrepareTaskRepeatDraft: %v", err)
	}
	result, err := svc.Capture(app.CaptureRequest{
		ProjectID: "proj-a", Title: draft.Title, Description: draft.Description, Priority: draft.Priority,
		Tags: draft.Tags, Context: draft.Context, Destination: draft.Destination,
	})
	if err != nil {
		t.Fatalf("Capture the draft: %v", err)
	}
	if persistedTask(t, repo, "proj-a", result.TaskID).Hidden {
		t.Fatal("the repeated task inherited the hidden flag")
	}
	requireHidden(t, repo, "proj-a", "t1", "repeat source")
}
