package app_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

// dependencyLocators renders a task's stored references as canonical locator
// strings so an assertion names order and identity in one comparable value.
func dependencyLocators(task *models.Task) []string {
	locators := make([]string, 0, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		locators = append(locators, reference.Locator())
	}
	return locators
}

func assertDependencies(t *testing.T, task *models.Task, want ...string) {
	t.Helper()
	got := dependencyLocators(task)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dependencies = %v, want %v", got, want)
	}
}

// dependencyFixture seeds two projects so every test can name a same-project
// target, a cross-project target, and a project the snapshot never loaded.
func dependencyFixture() (*fakeRepo, *app.Service, *models.Task, *models.Task, *models.Task) {
	source := newTask("source", 1, "Source")
	sibling := newTask("sibling", 2, "Sibling")
	remote := newTask("remote", 1, "Remote")
	alpha := newProject("proj-a", "Alpha", source, sibling)
	beta := newProject("proj-b", "Beta", remote)
	repo := newFakeRepo(alpha, beta)
	return repo, newTestService(repo), source, sibling, remote
}

func TestCaptureRecordsDependenciesInOrder(t *testing.T) {
	repo, svc, _, sibling, remote := dependencyFixture()

	result, err := svc.Capture(app.CaptureRequest{
		ProjectID: "proj-a",
		Title:     "Depends on two",
		Dependencies: []app.TaskLocator{
			repo.loc(sibling.ID),
			repo.loc(remote.ID),
		},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	created := findTask(t, repo, "proj-a", result.TaskID)
	assertDependencies(t, created, "proj-a:sibling", "proj-b:remote")
}

func TestCaptureCollapsesDuplicateDependencyAliases(t *testing.T) {
	repo, svc, _, sibling, remote := dependencyFixture()

	result, err := svc.Capture(app.CaptureRequest{
		ProjectID: "proj-a",
		Title:     "Duplicate aliases",
		Dependencies: []app.TaskLocator{
			repo.loc(sibling.ID),
			repo.loc(remote.ID),
			repo.loc(sibling.ID),
		},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	created := findTask(t, repo, "proj-a", result.TaskID)
	assertDependencies(t, created, "proj-a:sibling", "proj-b:remote")
}

func TestCaptureRejectsUnusableDependencyTargets(t *testing.T) {
	closed := newTask("closed", 3, "Closed")
	closed.Complete()

	cases := []struct {
		name   string
		target app.TaskLocator
	}{
		{"missing task", app.TaskLocator{ProjectID: "proj-a", TaskID: "nope"}},
		{"unloaded project", app.TaskLocator{ProjectID: "proj-ghost", TaskID: "sibling"}},
		{"terminal task", app.TaskLocator{ProjectID: "proj-a", TaskID: "closed"}},
		{"malformed locator", app.TaskLocator{ProjectID: "proj-a", TaskID: "has:colon"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sibling := newTask("sibling", 2, "Sibling")
			alpha := newProject("proj-a", "Alpha", sibling, closed)
			repo := newFakeRepo(alpha)
			svc := newTestService(repo)

			_, err := svc.Capture(app.CaptureRequest{
				ProjectID:    "proj-a",
				Title:        "Bad dependency",
				Dependencies: []app.TaskLocator{tc.target},
			})
			if !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("Capture error = %v, want ErrInvalidInput", err)
			}
			if len(repo.commits) != 0 {
				t.Fatalf("rejected capture committed %d times", len(repo.commits))
			}
			if len(alpha.Tasks) != 2 {
				t.Fatalf("rejected capture left %d tasks, want 2", len(alpha.Tasks))
			}
		})
	}
}

func TestEditTaskLineAddsRemovesAndClearsDependencies(t *testing.T) {
	repo, svc, source, sibling, remote := dependencyFixture()

	added, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:       repo.loc(source.ID),
		Title:        source.Title,
		Dependencies: app.DependencyPatch{Add: []app.TaskLocator{repo.loc(sibling.ID), repo.loc(remote.ID)}},
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !added.Changed {
		t.Fatal("adding a dependency reported an unchanged result")
	}
	assertDependencies(t, source, "proj-a:sibling", "proj-b:remote")

	removed, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:       repo.loc(source.ID),
		Title:        source.Title,
		Dependencies: app.DependencyPatch{Remove: []app.TaskLocator{repo.loc(sibling.ID)}},
	})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !removed.Changed {
		t.Fatal("removing a dependency reported an unchanged result")
	}
	assertDependencies(t, source, "proj-b:remote")

	cleared, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:       repo.loc(source.ID),
		Title:        source.Title,
		Dependencies: app.DependencyPatch{Clear: true},
	})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if !cleared.Changed {
		t.Fatal("clearing a dependency reported an unchanged result")
	}
	assertDependencies(t, source)
}

// TestEditTaskLineAddExistingTerminalOrUnavailableIsNoOp pins the
// existing-versus-new distinction on the patch resolver. Additions are the only
// direction that asks the workspace anything, but a reference the workspace
// deliberately preserved must stay idempotent once its target closes or its
// project stops being loaded: re-submitting a stored locator is the same request
// it was the moment before the target changed, so it may neither reorder the set
// nor advance the touch time nor become an error. Only a genuinely new target is
// measured against the eligibility rules.
func TestEditTaskLineAddExistingTerminalOrUnavailableIsNoOp(t *testing.T) {
	stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

	t.Run("stored target that has since closed", func(t *testing.T) {
		repo, svc, source, sibling, remote := dependencyFixture()
		if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
			t.Fatalf("seed dependency: %v", err)
		}
		sibling.Complete()
		source.UpdatedAt = stamp

		result, err := svc.EditTaskLine(app.TaskLineEditRequest{
			Source:       repo.loc(source.ID),
			Title:        source.Title,
			Dependencies: app.DependencyPatch{Add: []app.TaskLocator{repo.loc(sibling.ID)}},
		})
		if err != nil {
			t.Fatalf("re-adding a stored closed target: %v", err)
		}
		if result.Changed {
			t.Fatal("re-adding a stored closed target reported a change")
		}
		assertDependencies(t, source, "proj-a:sibling")
		if !source.UpdatedAt.Equal(stamp) {
			t.Fatalf("UpdatedAt = %v, want %v", source.UpdatedAt, stamp)
		}

		// The same closed task is still refused as a first-time addition elsewhere.
		if _, err := svc.EditTaskLine(app.TaskLineEditRequest{
			Source:       repo.loc(remote.ID),
			Title:        remote.Title,
			Dependencies: app.DependencyPatch{Add: []app.TaskLocator{repo.loc(sibling.ID)}},
		}); !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("new closed target error = %v, want ErrInvalidInput", err)
		}
		assertDependencies(t, remote)
	})

	t.Run("stored target whose project is no longer loaded", func(t *testing.T) {
		source := newTask("source", 1, "Source")
		source.Dependencies = []models.TaskReference{{ProjectID: "proj-b", TaskID: "remote"}}
		source.UpdatedAt = stamp
		alpha := newProject("proj-a", "Alpha", source)
		beta := newProject("proj-b", "Beta", newTask("remote", 1, "Remote"), newTask("stranger", 2, "Stranger"))
		repo := newFakeRepo(alpha).withUnloaded(beta)
		svc := newTestService(repo)

		result, err := svc.EditTaskLine(app.TaskLineEditRequest{
			Source:       repo.loc(source.ID),
			Title:        source.Title,
			Dependencies: app.DependencyPatch{Add: []app.TaskLocator{{ProjectID: "proj-b", TaskID: "remote"}}},
		})
		if err != nil {
			t.Fatalf("re-adding a stored unavailable target: %v", err)
		}
		if result.Changed {
			t.Fatal("re-adding a stored unavailable target reported a change")
		}
		assertDependencies(t, source, "proj-b:remote")
		if !source.UpdatedAt.Equal(stamp) {
			t.Fatalf("UpdatedAt = %v, want %v", source.UpdatedAt, stamp)
		}

		// A different task in that same unloaded project is still an ordinary refusal.
		if _, err := svc.EditTaskLine(app.TaskLineEditRequest{
			Source:       repo.loc(source.ID),
			Title:        source.Title,
			Dependencies: app.DependencyPatch{Add: []app.TaskLocator{{ProjectID: "proj-b", TaskID: "stranger"}}},
		}); !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("new unavailable target error = %v, want ErrInvalidInput", err)
		}
		assertDependencies(t, source, "proj-b:remote")
	})
}

func TestEditTaskLineDependencyNoOpsLeaveTheTouchTimeAlone(t *testing.T) {
	repo, svc, source, sibling, _ := dependencyFixture()

	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
		t.Fatalf("seed dependency: %v", err)
	}
	stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	source.UpdatedAt = stamp

	cases := []struct {
		name  string
		patch app.DependencyPatch
	}{
		{"add existing", app.DependencyPatch{Add: []app.TaskLocator{repo.loc(sibling.ID)}}},
		{"remove missing", app.DependencyPatch{Remove: []app.TaskLocator{{ProjectID: "proj-b", TaskID: "remote"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.EditTaskLine(app.TaskLineEditRequest{
				Source:       repo.loc(source.ID),
				Title:        source.Title,
				Dependencies: tc.patch,
			})
			if err != nil {
				t.Fatalf("EditTaskLine: %v", err)
			}
			if result.Changed {
				t.Fatal("a dependency no-op reported a change")
			}
			assertDependencies(t, source, "proj-a:sibling")
			if !source.UpdatedAt.Equal(stamp) {
				t.Fatalf("UpdatedAt = %v, want %v", source.UpdatedAt, stamp)
			}
		})
	}

	t.Run("clear empty", func(t *testing.T) {
		bare := findTask(t, repo, "proj-a", sibling.ID)
		bare.UpdatedAt = stamp
		result, err := svc.EditTaskLine(app.TaskLineEditRequest{
			Source:       repo.loc(bare.ID),
			Title:        bare.Title,
			Dependencies: app.DependencyPatch{Clear: true},
		})
		if err != nil {
			t.Fatalf("EditTaskLine: %v", err)
		}
		if result.Changed {
			t.Fatal("clearing an empty dependency set reported a change")
		}
		if !bare.UpdatedAt.Equal(stamp) {
			t.Fatalf("UpdatedAt = %v, want %v", bare.UpdatedAt, stamp)
		}
	})
}

func TestEditTaskLineRejectsSelfReference(t *testing.T) {
	repo, svc, source, _, _ := dependencyFixture()

	_, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:       repo.loc(source.ID),
		Title:        source.Title,
		Dependencies: app.DependencyPatch{Add: []app.TaskLocator{repo.loc(source.ID)}},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("self-reference error = %v, want ErrInvalidInput", err)
	}
	assertDependencies(t, source)
}

// TestEditTaskLineRejectsSelfReferenceAtTheDestinationLocator proves the
// self-reference test is measured against the locator the task ends the edit
// with, not the one it started from: a move plus a dependency on the new address
// is still the task naming itself.
func TestEditTaskLineRejectsSelfReferenceAtTheDestinationLocator(t *testing.T) {
	repo, svc, source, _, _ := dependencyFixture()

	_, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:               repo.loc(source.ID),
		Title:                source.Title,
		DestinationProjectID: "proj-b",
		Dependencies:         app.DependencyPatch{Add: []app.TaskLocator{{ProjectID: "proj-b", TaskID: source.ID}}},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("destination self-reference error = %v, want ErrInvalidInput", err)
	}
	if source.Project != nil && source.Project.ID != "proj-a" {
		t.Fatalf("rejected edit moved the task to %s", source.Project.ID)
	}
}

func TestEditTaskLineRejectsClearCombinedWithAddOrRemove(t *testing.T) {
	repo, svc, source, sibling, _ := dependencyFixture()

	for name, patch := range map[string]app.DependencyPatch{
		"clear and add":    {Clear: true, Add: []app.TaskLocator{repo.loc(sibling.ID)}},
		"clear and remove": {Clear: true, Remove: []app.TaskLocator{repo.loc(sibling.ID)}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.EditTaskLine(app.TaskLineEditRequest{
				Source:       repo.loc(source.ID),
				Title:        source.Title,
				Dependencies: patch,
			})
			if !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestEditTaskLineRejectsAddingAndRemovingTheSameTarget(t *testing.T) {
	repo, svc, source, sibling, _ := dependencyFixture()

	_, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source: repo.loc(source.ID),
		Title:  source.Title,
		Dependencies: app.DependencyPatch{
			Add:    []app.TaskLocator{repo.loc(sibling.ID)},
			Remove: []app.TaskLocator{repo.loc(sibling.ID)},
		},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}
