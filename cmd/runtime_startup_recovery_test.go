package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/transaction"
)

func prepareCLIStartupRecovery(t *testing.T, state transaction.State) (string, string, string) {
	t.Helper()
	dataDir := t.TempDir()
	configPath := config.ConfigPathForDataDir(dataDir)
	manager := config.NewManager(configPath)
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("LoadOrCreateDomains: %v", err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}

	project := testutil.NewTestProject("Recovered Project")
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	task := testutil.NewTestTask("Recovered task")
	project.Tasks = append(project.Tasks, task)
	project.NextShortID = 2
	if saveErr := storage.SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject: %v", saveErr)
	}

	domains, err := runtime.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	if upsertErr := domains.Catalog.UpsertProject(project); upsertErr != nil {
		t.Fatalf("UpsertProject: %v", upsertErr)
	}
	domains.Planning.SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: task.ID}})
	artifacts, err := runtime.CheckedSyncedArtifacts(domains)
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
	return configPath, project.ID, task.ID
}

func TestLoadCLIWorkspaceRejectsLegacyBeforeStartupEffects(t *testing.T) {
	t.Run("create-on-load", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		dataDir := filepath.Join(home, "legacy-workspace")
		if err := os.MkdirAll(dataDir, config.DirPermissions); err != nil {
			t.Fatalf("mkdir legacy workspace: %v", err)
		}
		legacyPath := config.ConfigPathForDataDir(dataDir)
		if err := os.WriteFile(legacyPath, []byte(`{"version":1}`), config.FilePermissions); err != nil {
			t.Fatalf("write legacy config: %v", err)
		}

		setCommandDataDir(t, dataDir)

		workspace, err := LoadCLIWorkspace()
		if workspace != nil {
			t.Fatal("legacy-only CLI startup returned a workspace")
		}
		if !errors.Is(err, config.ErrLegacyConfigRequiresMigration) {
			t.Fatalf("CLI startup error = %v, want ErrLegacyConfigRequiresMigration", err)
		}
		var migrationErr *config.LegacyConfigRequiresMigrationError
		if !errors.As(err, &migrationErr) {
			t.Fatalf("CLI startup error type = %T, want *LegacyConfigRequiresMigrationError", err)
		}
		if migrationErr.Path != legacyPath {
			t.Fatalf("migration path = %q, want %q", migrationErr.Path, legacyPath)
		}

		entries, readErr := os.ReadDir(dataDir)
		if readErr != nil {
			t.Fatalf("read legacy workspace: %v", readErr)
		}
		if len(entries) != 1 || entries[0].Name() != filepath.Base(legacyPath) {
			t.Fatalf("CLI startup created workspace effects: %v", entries)
		}
		for _, path := range []string{
			config.InternalDir(dataDir),
			filepath.Join(config.LocalDataRoot(), "runtime"),
		} {
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("CLI startup created %q, stat error = %v", path, statErr)
			}
		}
	})
}

func TestLoadCLIWorkspaceRecoversTransactionsBeforeLoadingDomains(t *testing.T) {
	for _, state := range []transaction.State{transaction.StatePrepared, transaction.StateCommitting} {
		t.Run(string(state), func(t *testing.T) {
			configPath, projectID, taskID := prepareCLIStartupRecovery(t, state)
			setCommandDataDir(t, config.WorkspaceRootFromConfigPath(configPath))

			workspace, err := LoadCLIWorkspace()
			if err != nil {
				t.Fatalf("LoadCLIWorkspace: %v", err)
			}
			if project := workspace.Runtime.GetProject(projectID); project == nil {
				t.Fatalf("recovered catalog missing project %q", projectID)
			}
			queue := workspace.Runtime.Planning().GetNext()
			if len(queue) != 1 || queue[0].ProjectID != projectID || queue[0].TaskID != taskID {
				t.Fatalf("recovered focus queue = %+v", queue)
			}
		})
	}
}

func TestLoadCLIWorkspaceBlocksOnRecoveryQuarantine(t *testing.T) {
	dataDir := t.TempDir()
	configPath := config.ConfigPathForDataDir(dataDir)
	manager := config.NewManager(configPath)
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("LoadOrCreateDomains: %v", err)
	}
	txnDir := config.TransactionsDir(dataDir)
	if err := os.MkdirAll(txnDir, storage.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(txnDir, "corrupt.json"), []byte("{"), storage.FilePermissions); err != nil {
		t.Fatalf("write corrupt manifest: %v", err)
	}

	setCommandDataDir(t, dataDir)
	workspace, err := LoadCLIWorkspace()
	if err == nil {
		t.Fatal("LoadCLIWorkspace succeeded with quarantined transaction")
	}
	if workspace != nil {
		t.Fatal("LoadCLIWorkspace returned a partial workspace on quarantine")
	}
	if !errors.Is(err, storage.ErrCommitBlocked) {
		t.Fatalf("error = %v, want ErrCommitBlocked", err)
	}
}
