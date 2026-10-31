package engine

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
)

func TestTerminalRestoreRefusesIncomingPlanningWithoutTaskDrift(t *testing.T) {
	for _, redo := range []bool{false, true} {
		for _, horizon := range []string{"today", "next", "unreadable"} {
			t.Run(fmt.Sprintf("%s/redo=%v", horizon, redo), func(t *testing.T) {
				env := newHeadlessEnvironment(t)
				runtime := openHeadless(t, env.config(true))
				project := runtime.State().SystemUnfiledProject()
				captured, err := runtime.Service().Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Status A"})
				if err != nil {
					t.Fatal(err)
				}
				target := app.TaskLocator{ProjectID: project.ID, TaskID: captured.TaskID}
				if _, err = runtime.Service().Transition(app.StateRequest{Locator: target, Target: "done"}); err != nil {
					t.Fatal(err)
				}
				if redo {
					if _, err = runtime.Ledger().Undo(storage.GlobalScope()); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err = runtime.Service().Transition(app.StateRequest{Locator: target, Target: "inbox"}); err != nil {
						t.Fatal(err)
					}
				}
				store := config.NewDomainFileStore(env.dataDir)
				planningPath := config.PlanningStatePath(env.dataDir)
				planning, err := store.LoadPlanningReadOnly()
				if err != nil {
					t.Fatal(err)
				}
				switch horizon {
				case "today":
					planning.AddToToday(target.TaskID, target.ProjectID)
				case "next":
					planning.AddToNext(target.TaskID, target.ProjectID)
				}
				if err = store.SavePlanning(planning); err != nil {
					t.Fatal(err)
				}
				if horizon == "unreadable" {
					original := readPlanningRestoreFile(t, planningPath)
					if err = os.WriteFile(planningPath+".bak", original, 0o600); err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(planningPath, []byte("corrupt"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				tasksPath := storage.TasksFilePath(project.Path)
				beforeTasks, beforePlanning := readPlanningRestoreFile(t, tasksPath), readPlanningRestoreFile(t, planningPath)
				if redo {
					_, err = runtime.Ledger().Redo(storage.GlobalScope())
				} else {
					_, err = runtime.Ledger().Undo(storage.GlobalScope())
				}
				if err == nil || (horizon != "unreadable" && !errors.Is(err, storage.ErrStaleArtifact)) {
					t.Fatalf("restore error = %v, want planning refusal", err)
				}
				if !bytes.Equal(beforeTasks, readPlanningRestoreFile(t, tasksPath)) || !bytes.Equal(beforePlanning, readPlanningRestoreFile(t, planningPath)) {
					t.Fatal("refused restore changed task or planning bytes, or recovered a backup")
				}
			})
		}
	}
}
