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
	task := testutil.NewTestTask("Recovered task")
	// Explicit Next membership is independent of lifecycle state.
	task.WorkflowState = models.StateBacklog
	project.Tasks = append(project.Tasks, task)
	project.NextShortID = 2
	if saveErr := storage.SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject: %v", saveErr)
	}

	domains, err := staleRuntime.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	if upsertErr := domains.Catalog.UpsertProject(project); upsertErr != nil {
		t.Fatalf("UpsertProject: %v", upsertErr)
	}
	domains.Planning.SetNextOrder([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: task.ID}})
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
			if len(app.nextOrder) != 1 || app.nextOrder[0].Task.ID != taskID {
				t.Fatalf("recovered focus queue = %+v", app.nextOrder)
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
