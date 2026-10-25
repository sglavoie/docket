package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/transaction"
)

func TestManagerV5MigrationCorruptInputRefusesBeforeSnapshot(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	corrupt, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, "unsupported", "corrupt-note.md"))
	if err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join(root, ".tt", "global-notes", "global-reference.md")
	originalWorkspace, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(notePath, corrupt, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	manager := NewManager(ConfigPathForDataDir(root))
	err = manager.RecoverWorkspace()
	if err == nil || !strings.Contains(err.Error(), "workspace was not changed") {
		t.Fatalf("corrupt migration error = %v, want unchanged refusal", err)
	}
	afterWorkspace, readErr := os.ReadFile(WorkspaceCatalogPath(root))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(afterWorkspace) != string(originalWorkspace) {
		t.Fatal("corrupt-input refusal changed workspace catalog bytes")
	}
	snapshots, inspectErr := InspectPreSimplificationV5Snapshots(root)
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	if snapshots.HasArtifacts {
		t.Fatalf("corrupt-input refusal created snapshot artifacts: %+v", snapshots)
	}
}

func TestManagerV5MigrationRejectsFutureTaskSchemaBeforeSnapshot(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	future, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, "unsupported", "future-tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(filepath.Join(root, "projects", "alpha", "tasks.md"), future, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	err = NewManager(ConfigPathForDataDir(root)).RecoverWorkspace()
	if err == nil || !strings.Contains(err.Error(), "unsupported task schema 99") {
		t.Fatalf("future task migration error = %v", err)
	}
	snapshots, inspectErr := InspectPreSimplificationV5Snapshots(root)
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	if snapshots.HasArtifacts {
		t.Fatalf("future task refusal created snapshot artifacts: %+v", snapshots)
	}
}

func TestManagerV5MigrationRecoversPreparedActivationBeforeInspection(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	plan, err := buildV5MigrationActivationPlan(root, time.Local)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, snapshotErr := createPreSimplificationV5SnapshotWithMigrationAudit(root, plan.audit); snapshotErr != nil {
		t.Fatalf("create migration snapshot: %v", snapshotErr)
	}

	ops := make([]transaction.Op, 0, len(plan.artifacts)+len(plan.cleanupOps))
	for _, artifact := range plan.artifacts {
		ops = append(ops, transaction.Op{
			Kind:   transaction.OpWrite,
			Target: artifact.Path,
			Data:   artifact.Data,
			Mode:   artifact.Mode,
		})
	}
	ops = append(ops, plan.cleanupOps...)
	journal := transaction.NewJournal(
		TransactionsDir(root),
		[]string{root, LocalDataRoot()}, "",
	)
	manifest, err := journal.Prepare(ops)
	if err != nil {
		t.Fatalf("prepare interrupted activation: %v", err)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); statErr != nil {
		t.Fatalf("prepared migration has no manifest: %v", statErr)
	}

	manager := NewManager(ConfigPathForDataDir(root))
	if recoverErr := manager.RecoverWorkspace(); recoverErr != nil {
		t.Fatalf("recover prepared activation: %v", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	if !exists || schema != CurrentConfigVersion {
		t.Fatalf("recovered catalog schema = %d, exists=%v", schema, exists)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered migration manifest still exists: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "projects", "alpha", "completed.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered migration retained completed archive: %v", statErr)
	}
	snapshots, err := InspectPreSimplificationV5Snapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots.Pairs) != 1 || len(snapshots.Issues) != 0 {
		t.Fatalf("recovered migration snapshot inspection = %+v", snapshots)
	}
}
