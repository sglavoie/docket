package tui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/transaction"
)

func prepareTUIStartupRecovery(t *testing.T, state transaction.State) (*config.RuntimeState, *config.Manager, string, string) {
	t.Helper()
	dataDir := t.TempDir()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("LoadOrCreateDomains: %v", err)
	}
	staleRuntime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}

	project := testutil.NewTestProject("Recovered Project")
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	task := testutil.NewTestTask("Recovered task", models.TaskTypeTask)
	// Queue membership is the StateNext workflow state, so a hydrated member must
	// be StateNext to survive prune-on-load.
	task.WorkflowState = models.StateNext
	project.Tasks = append(project.Tasks, task)
	project.NextShortID = 2
	if saveErr := storage.SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject: %v", saveErr)
	}

	domains, err := staleRuntime.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	domains.Catalog.UpsertProject(project)
	domains.Planning.SetFocusQueue([]config.FocusQueueItem{{ProjectID: project.ID, TaskID: task.ID}})
	artifacts, err := staleRuntime.CheckedSyncedArtifacts(domains)
	if err != nil {
		t.Fatalf("CheckedSyncedArtifacts: %v", err)
	}
	var ops []transaction.Op
	for _, artifact := range artifacts {
		switch filepath.Base(artifact.Path) {
		case "workspace.json", "planning.json":
			ops = append(ops, transaction.Op{Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: storage.FilePermissions})
		}
	}
	if len(ops) != 2 {
		t.Fatalf("startup recovery ops = %d, want 2", len(ops))
	}

	journal := transaction.NewJournal(config.TransactionsDir(dataDir), []string{dataDir})
	manifest, err := journal.Prepare(ops)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if state == transaction.StateCommitting {
		manifest.State = transaction.StateCommitting
		data, marshalErr := json.MarshalIndent(manifest, "", "  ")
		if marshalErr != nil {
			t.Fatalf("marshal committing manifest: %v", marshalErr)
		}
		if writeErr := os.WriteFile(journal.ManifestPath(manifest.ID), data, storage.FilePermissions); writeErr != nil {
			t.Fatalf("write committing manifest: %v", writeErr)
		}
		if renameErr := os.Rename(manifest.Ops[0].Staged, manifest.Ops[0].Target); renameErr != nil {
			t.Fatalf("apply first transaction op: %v", renameErr)
		}
	}
	return staleRuntime, manager, project.ID, task.ID
}

func writeTUILegacyMachineState(t *testing.T, dataDir, workspaceID string, machine *config.MachineState) string {
	t.Helper()
	legacyPath := filepath.Join(config.LocalRuntimeDirForDataDir(dataDir), "machine.json")
	data, err := json.MarshalIndent(map[string]any{
		"schema_version": config.CurrentDomainSchemaVersion,
		"workspace_id":   workspaceID,
		"kind":           "machine",
		"data":           machine,
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy machine state: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), config.DirPermissions); err != nil {
		t.Fatalf("mkdir legacy machine dir: %v", err)
	}
	if err := os.WriteFile(legacyPath, data, config.FilePermissions); err != nil {
		t.Fatalf("write legacy machine state: %v", err)
	}
	return legacyPath
}

func TestTUIStartupUsesMigratedMachineState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	domains, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("LoadOrCreateDomains: %v", err)
	}
	canonicalPath, err := config.MachineStatePath(domains.Catalog.WorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	if removeErr := os.Remove(canonicalPath); removeErr != nil {
		t.Fatalf("remove canonical machine fixture: %v", removeErr)
	}
	legacyPath := writeTUILegacyMachineState(t, dataDir, domains.Catalog.WorkspaceID, &config.MachineState{Editor: "tui-legacy-editor"})

	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	app, err := NewAppModel(runtime, manager)
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	if got := app.runtime.Machine().Editor; got != "tui-legacy-editor" {
		t.Fatalf("TUI startup editor = %q, want tui-legacy-editor", got)
	}
	if _, err := os.Stat(canonicalPath); err != nil {
		t.Fatalf("canonical machine state not created: %v", err)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy rollback source not retained: %v", err)
	}
}

func TestNewAppModelRecoversTransactionsBeforeLoadingWorkspace(t *testing.T) {
	for _, state := range []transaction.State{transaction.StatePrepared, transaction.StateCommitting} {
		t.Run(string(state), func(t *testing.T) {
			staleRuntime, manager, projectID, taskID := prepareTUIStartupRecovery(t, state)
			app, err := NewAppModel(staleRuntime, manager)
			if err != nil {
				t.Fatalf("NewAppModel: %v", err)
			}
			if project := app.runtime.GetProject(projectID); project == nil {
				t.Fatalf("recovered catalog missing project %q", projectID)
			}
			if len(app.focusQueue) != 1 || app.focusQueue[0].Task.ID != taskID {
				t.Fatalf("recovered focus queue = %+v", app.focusQueue)
			}
		})
	}
}

func TestNewAppModelBlocksOnRecoveryQuarantine(t *testing.T) {
	dataDir := t.TempDir()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("LoadOrCreateDomains: %v", err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	txnDir := config.TransactionsDir(dataDir)
	if mkdirErr := os.MkdirAll(txnDir, storage.DirPermissions); mkdirErr != nil {
		t.Fatalf("MkdirAll: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(txnDir, "corrupt.json"), []byte("{"), storage.FilePermissions); writeErr != nil {
		t.Fatalf("write corrupt manifest: %v", writeErr)
	}

	app, err := NewAppModel(runtime, manager)
	if err == nil {
		t.Fatal("NewAppModel succeeded with quarantined transaction")
	}
	if app != nil {
		t.Fatal("NewAppModel returned a partial model on quarantine")
	}
	if !errors.Is(err, storage.ErrCommitBlocked) {
		t.Fatalf("error = %v, want ErrCommitBlocked", err)
	}
}
