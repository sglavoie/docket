package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
	task := testutil.NewTestTask("Recovered task", models.TaskTypeTask)
	project.Tasks = append(project.Tasks, task)
	project.NextShortID = 2
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	domains, err := runtime.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	domains.Catalog.UpsertProject(project)
	domains.Planning.SetFocusQueue([]config.FocusQueueItem{{ProjectID: project.ID, TaskID: task.ID}})
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

func TestLoadCLIWorkspaceRecoversTransactionsBeforeLoadingDomains(t *testing.T) {
	for _, state := range []transaction.State{transaction.StatePrepared, transaction.StateCommitting} {
		t.Run(string(state), func(t *testing.T) {
			configPath, projectID, taskID := prepareCLIStartupRecovery(t, state)
			oldConfig := viper.GetString("config")
			viper.Set("config", configPath)
			t.Cleanup(func() { viper.Set("config", oldConfig) })

			workspace, err := LoadCLIWorkspaceStrict()
			if err != nil {
				t.Fatalf("LoadCLIWorkspaceStrict: %v", err)
			}
			if project := workspace.Runtime.GetProject(projectID); project == nil {
				t.Fatalf("recovered catalog missing project %q", projectID)
			}
			queue := workspace.Runtime.Planning().GetFocusQueue()
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

	oldConfig := viper.GetString("config")
	viper.Set("config", configPath)
	t.Cleanup(func() { viper.Set("config", oldConfig) })
	workspace, err := LoadCLIWorkspaceStrict()
	if err == nil {
		t.Fatal("LoadCLIWorkspaceStrict succeeded with quarantined transaction")
	}
	if workspace != nil {
		t.Fatal("LoadCLIWorkspaceStrict returned a partial workspace on quarantine")
	}
	if !errors.Is(err, storage.ErrCommitBlocked) {
		t.Fatalf("error = %v, want ErrCommitBlocked", err)
	}
}
