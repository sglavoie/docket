package engine

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

func TestPlanningRestoreGuardValidatesProposedTaskImages(t *testing.T) {
	env := newHeadlessEnvironment(t)
	runtime := openHeadless(t, env.config(true))
	unfiled := runtime.State().SystemUnfiledProject()
	captured, err := runtime.Service().Capture(app.CaptureRequest{ProjectID: unfiled.ID, Title: "Plan A"})
	if err != nil {
		t.Fatal(err)
	}
	locator := app.TaskLocator{ProjectID: captured.ProjectID, TaskID: captured.TaskID}
	if _, err = runtime.Service().ApplyPlanning(app.PlanningRequest{Locator: locator, Action: app.PlanningAddToday}); err != nil {
		t.Fatal(err)
	}
	planningPath := config.PlanningStatePath(env.dataDir)
	taskPath := storage.TasksFilePath(unfiled.Path)
	desired := readPlanningRestoreFile(t, planningPath)
	desiredTasks := readPlanningRestoreFile(t, taskPath)
	if _, err = runtime.Service().Transition(app.StateRequest{Locator: locator, Target: "done"}); err != nil {
		t.Fatal(err)
	}
	expected := readPlanningRestoreFile(t, planningPath)
	paths, validate, err := preparePlanningRestoreGuard(env.dataDir, expected, desired)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(paths, config.WorkspaceCatalogPath(env.dataDir)) || !slices.Contains(paths, taskPath) {
		t.Fatalf("guard did not lock catalog and task document: %v", paths)
	}
	if err := validate(nil); !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("terminal current target accepted: %v", err)
	}
	if err := validate([]transaction.Op{{Kind: transaction.OpWrite, Target: taskPath, Data: desiredTasks}}); err != nil {
		t.Fatalf("nonterminal proposed target refused: %v", err)
	}
	if err := validate([]transaction.Op{{Kind: transaction.OpDelete, Target: taskPath}}); !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("deleted proposed target accepted: %v", err)
	}
	if _, validateRemoval, err := preparePlanningRestoreGuard(env.dataDir, desired, expected); err != nil || validateRemoval != nil {
		t.Fatalf("removal unexpectedly requires task reads: validator=%v error=%v", validateRemoval != nil, err)
	}
}

func TestPlanningRestoreGuardRefusesChangedOrUnreadableProjects(t *testing.T) {
	for _, damage := range []string{"rename_project", "missing_project", "corrupt_project", "corrupt_catalog"} {
		t.Run(damage, func(t *testing.T) {
			env := newHeadlessEnvironment(t)
			runtime := openHeadless(t, env.config(true))
			created, err := runtime.CreateProject("planning-guard")
			if err != nil {
				t.Fatal(err)
			}
			project := created.Project
			captured, err := runtime.Service().Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Plan A"})
			if err != nil {
				t.Fatal(err)
			}
			planningPath := config.PlanningStatePath(env.dataDir)
			expected := readPlanningRestoreFile(t, planningPath)
			if _, err = runtime.Service().ApplyPlanning(app.PlanningRequest{
				Locator: app.TaskLocator{ProjectID: captured.ProjectID, TaskID: captured.TaskID}, Action: app.PlanningAddToday,
			}); err != nil {
				t.Fatal(err)
			}
			desired := readPlanningRestoreFile(t, planningPath)
			_, validate, err := preparePlanningRestoreGuard(env.dataDir, expected, desired)
			if err != nil {
				t.Fatal(err)
			}
			catalogPath := config.WorkspaceCatalogPath(env.dataDir)
			damagedPath := storage.TasksFilePath(project.Path)
			switch damage {
			case "rename_project":
				// Write a syntactically valid catalog with a new path without
				// moving files, modeling a change between path capture and locks.
				data := readPlanningRestoreFile(t, catalogPath)
				data = bytes.ReplaceAll(data, []byte(`"slug": "planning-guard"`), []byte(`"slug": "renamed"`))
				if err := os.WriteFile(catalogPath, data, 0o600); err != nil {
					t.Fatal(err)
				}
				damagedPath = catalogPath
			case "missing_project":
				if err := os.Remove(damagedPath); err != nil {
					t.Fatal(err)
				}
			case "corrupt_project", "corrupt_catalog":
				if damage == "corrupt_catalog" {
					damagedPath = catalogPath
				}
				original := readPlanningRestoreFile(t, damagedPath)
				if err := os.WriteFile(damagedPath+".bak", original, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(damagedPath, []byte("corrupt"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, beforeErr := os.ReadFile(damagedPath)
			if err := validate(nil); err == nil {
				t.Fatal("guard accepted a changed or unreadable target project")
			} else if damage == "rename_project" && !errors.Is(err, storage.ErrStaleArtifact) {
				t.Fatalf("renamed project error = %v, want stale artifact", err)
			}
			after, afterErr := os.ReadFile(damagedPath)
			if !bytes.Equal(before, after) || errors.Is(beforeErr, os.ErrNotExist) != errors.Is(afterErr, os.ErrNotExist) {
				t.Fatal("guard wrote a file or recovered its backup")
			}
		})
	}
}

func readPlanningRestoreFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
