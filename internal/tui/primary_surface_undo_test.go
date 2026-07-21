package tui

import (
	"bytes"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestTUIWorkspaceLedgerUndoRedoAcrossPrimaryTaskSurfaces is the FR-012-001
// regression: the surviving single-project Outliner, Task Detail, and Inbox
// processors must route u/ctrl+r through the durable workspace ledger so a
// mutation committed from those surfaces is undoable and redoable — restoring
// exact task AND Today/Next planning disk state — and the restore must survive a
// restart (no reliance on in-memory undo history).
//
// Each case commits a planning-aware transition through the ledger-backed
// service, discards the mutating model entirely (a restart with no in-memory
// undo state), then drives undo/redo on a fresh model reading only the durable
// ledger.
func TestTUIWorkspaceLedgerUndoRedoAcrossPrimaryTaskSurfaces(t *testing.T) {
	// Outliner and Task Detail: their u/ctrl+r flow through the app-level global
	// task undo/redo dispatch, which now recognizes both view types and selects
	// the durable workspace-ledger transaction.
	surfaces := []struct {
		name string
		view PresentationID
	}{
		{name: "outliner", view: ViewOutliner},
		{name: "task-detail", view: ViewTaskDetail},
	}

	for _, sf := range surfaces {
		t.Run(sf.name, func(t *testing.T) {
			dataDir := t.TempDir()
			task := seedTask(t, 1, "Primary surface undo target", models.StateReady)
			project := newLedgerUndoProject(t, dataDir, "surface-"+sf.name, task)

			mutator := newLedgerUndoApp(t, dataDir, project, sf.view, task.ID, true)

			// Seed the task into Today so a transition into Next must drop the Today
			// entry through the folded planning artifact — proving the undo restores
			// task state AND planning state together.
			seedTodayMembership(t, mutator.configManager, project.ID, task.ID)
			if !planningTaskInToday(t, mutator.configManager, task.ID) {
				t.Fatal("failed to seed Today membership")
			}
			before := readTasksFile(t, project)

			if _, err := mutator.appService.Transition(app.StateRequest{TaskID: task.ID, Target: models.StateNext}); err != nil {
				t.Fatalf("planning-aware transition: %v", err)
			}
			if planningTaskInToday(t, mutator.configManager, task.ID) {
				t.Fatal("transition into Next did not drop the Today entry on disk")
			}
			after := readTasksFile(t, project)
			if bytes.Equal(before, after) {
				t.Fatal("transition did not change tasks.md on disk")
			}

			// Restart: a fresh AppModel with no in-memory undo state, hosting the
			// surface view, reading the durable ledger the previous session wrote.
			restarted := newLedgerUndoApp(t, dataDir, restartLedgerProject(project), sf.view, task.ID, false)

			restarted = sendAppKey(restarted, syntheticKey('u'))
			if got := readTasksFile(t, project); !bytes.Equal(got, before) {
				t.Fatalf("%s undo did not restore exact tasks.md\n before: %q\n after undo: %q", sf.name, before, got)
			}
			if !planningTaskInToday(t, restarted.configManager, task.ID) {
				t.Fatalf("%s undo did not restore the Today entry on disk", sf.name)
			}
			if fresh := restarted.currentProject.GetTaskByID(task.ID); fresh == nil || fresh.State() == models.StateNext {
				t.Fatalf("%s undo did not restore pre-transition workflow state: %v", sf.name, fresh)
			}

			restarted = sendAppKey(restarted, syntheticCtrlKey('r'))
			if got := readTasksFile(t, project); !bytes.Equal(got, after) {
				t.Fatalf("%s redo did not restore exact tasks.md\n after: %q\n after redo: %q", sf.name, after, got)
			}
			if planningTaskInToday(t, restarted.configManager, task.ID) {
				t.Fatalf("%s redo did not re-drop the Today entry on disk", sf.name)
			}
		})
	}

	// Inbox: stateful triage commits through the ledger-backed inbox commit path;
	// its own u/ctrl+r (undoLastAction/redoLastAction) now restore that durable
	// transaction rather than a lossy in-memory task snapshot, so both task state
	// and the Today planning artifact restore exactly — and it works from a fresh
	// processor with an empty in-session history (a restart).
	t.Run("inbox", func(t *testing.T) {
		dataDir := t.TempDir()
		task := seedTask(t, 1, "Inbox triage undo target", models.StateReady)
		task.SetSection(models.SectionInbox)
		project := newLedgerUndoProject(t, dataDir, "inbox", task)

		manager, runtime := seedDurableInboxWorkspace(t, dataDir, project)

		seedTodayMembership(t, manager, project.ID, task.ID)
		if !planningTaskInToday(t, manager, task.ID) {
			t.Fatal("failed to seed Today membership")
		}
		before := readTasksFile(t, project)

		// Triage the inbox task onto the Next section (a state change into
		// StateNext). The ledger-backed commit drops the Today entry via the folded
		// planning artifact.
		inbox := NewInboxProcessorModel(project, manager, runtime)
		drainInboxCmd(inbox.moveToSection(models.SectionNext))

		if planningTaskInToday(t, manager, task.ID) {
			t.Fatal("inbox triage into Next did not drop the Today entry on disk")
		}
		after := readTasksFile(t, project)
		if bytes.Equal(before, after) {
			t.Fatal("inbox triage did not change tasks.md on disk")
		}

		// Restart: a fresh inbox processor with an empty processingHistory, reading
		// only the durable ledger.
		restartProject := restartLedgerProject(project)
		if err := storage.LoadProject(restartProject); err != nil {
			t.Fatalf("reload project on restart: %v", err)
		}
		restarted := NewInboxProcessorModel(restartProject, manager, runtime)

		if cmd := restarted.undoLastAction(); cmd != nil {
			drainInboxCmd(cmd)
		}
		if got := readTasksFile(t, project); !bytes.Equal(got, before) {
			t.Fatalf("inbox undo did not restore exact tasks.md\n before: %q\n after undo: %q", before, got)
		}
		if !planningTaskInToday(t, manager, task.ID) {
			t.Fatal("inbox undo did not restore the Today entry on disk")
		}
		if fresh := restartProject.GetTaskByID(task.ID); fresh == nil || fresh.State() == models.StateNext {
			t.Fatalf("inbox undo did not restore pre-triage workflow state: %v", fresh)
		}

		if cmd := restarted.redoLastAction(); cmd != nil {
			drainInboxCmd(cmd)
		}
		if got := readTasksFile(t, project); !bytes.Equal(got, after) {
			t.Fatalf("inbox redo did not restore exact tasks.md\n after: %q\n after redo: %q", after, got)
		}
		if planningTaskInToday(t, manager, task.ID) {
			t.Fatal("inbox redo did not re-drop the Today entry on disk")
		}
	})
}

// seedDurableInboxWorkspace persists the project's initial tasks.md through a
// ledger-backed commit service and returns a manager/runtime rooted at dataDir,
// wired exactly like production so inbox commits and their undo restore share one
// durable workspace ledger and one planning.json.
func seedDurableInboxWorkspace(t *testing.T, dataDir string, project *models.Project) (*config.Manager, *config.RuntimeState) {
	t.Helper()

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{project})
	cfg.Machine().CurrentProject = project.ID
	manager := config.NewManager(filepath.Join(dataDir, "tt.json"))
	runtime := testRuntimeAt(cfg.DataDir(), cfg)
	return manager, runtime
}

// drainInboxCmd executes a tea.Cmd (flattening any tea.Batch) so the deferred
// save side effects run synchronously in the test.
func drainInboxCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			drainInboxCmd(c)
		}
	}
}
