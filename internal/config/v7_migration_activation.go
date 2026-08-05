package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

type v7MigrationActivationPlan struct {
	artifacts  []storage.ArtifactChange
	cleanupOps []transaction.Op
}

type v7MigrationActivationHooks interface {
	snapshotCreationCallbacks() preScopeV7SnapshotCallbacks
	didPublishSnapshot(*PreScopeV7Snapshot)
}

// MigrateWorkspaceToV7 activates schema 7 at the guarded startup boundary.
func (m *Manager) MigrateWorkspaceToV7() error {
	return m.migrateWorkspaceToV7(nil)
}

//nolint:govet // Each guarded migration stage keeps its error scope local.
func (m *Manager) migrateWorkspaceToV7(hooks v7MigrationActivationHooks) error {
	if m == nil {
		return fmt.Errorf("configuration manager is required")
	}
	dataDir, err := validateV5DryRunRoot(m.workspaceDataDir())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	// A prior activation may have reached the durable prepare barrier before the
	// process stopped. Recover it before inspecting the catalog: a partially
	// applied schema boundary is roll-forward state, never a fresh candidate.
	roots := []string{dataDir, LocalDataRoot()}
	commit := storage.NewCommitService(TransactionsDir(dataDir), roots)
	if _, err := commit.RecoverWorkspace(); err != nil {
		return fmt.Errorf("recover workspace before schema-7 activation: %w", err)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if schema < V6ConfigSchemaVersion {
		if err := m.MigrateWorkspaceToV6(); err != nil {
			return fmt.Errorf("migrate supported workspace through schema 6: %w", err)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("schema-6 migration did not publish a workspace catalog")
		}
	}
	switch {
	case schema == V7ConfigSchemaVersion:
		return nil
	case schema > V7ConfigSchemaVersion:
		return fmt.Errorf(
			"workspace schema %d is newer than supported schema %d; upgrade tt",
			schema,
			V7ConfigSchemaVersion,
		)
	case schema != V6ConfigSchemaVersion:
		return fmt.Errorf(
			"workspace schema %d cannot be migrated directly to schema %d; restore a supported v4, v5, or v6 workspace or use a compatible tt release",
			schema,
			V7ConfigSchemaVersion,
		)
	}

	options := V7WorkspaceDryRunOptions{DataDir: dataDir}
	// Validate both the complete candidate and the publication topology before
	// creating rollback artifacts.
	if _, err := buildV7MigrationActivationPlan(options); err != nil {
		return fmt.Errorf("v7 migration validation failed; workspace was not changed: %w", err)
	}

	var snapshotCallbacks preScopeV7SnapshotCallbacks
	if hooks != nil {
		snapshotCallbacks = hooks.snapshotCreationCallbacks()
	}
	snapshot, err := createPreScopeV7Snapshot(PreScopeV7SnapshotOptions{DataDir: dataDir}, snapshotCallbacks)
	if err != nil {
		return err
	}

	expectedInventory, err := inventoryPreScopeV7(dataDir, false)
	if err != nil {
		return v7MigrationAfterSnapshotError(snapshot, err)
	}
	if err := compareV7SnapshotResultInventory(snapshot, expectedInventory); err != nil {
		return v7MigrationAfterSnapshotError(snapshot, err)
	}
	if hooks != nil {
		hooks.didPublishSnapshot(snapshot)
	}
	currentInventory, err := inventoryPreScopeV7(dataDir, false)
	if err != nil {
		return v7MigrationAfterSnapshotError(snapshot, err)
	}
	if err := comparePreScopeV7Inventories(expectedInventory, currentInventory); err != nil {
		return v7MigrationAfterSnapshotError(snapshot, err)
	}

	err = commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
		current, inventoryErr := inventoryPreScopeV7(dataDir, false)
		if inventoryErr != nil {
			return nil, nil, inventoryErr
		}
		if compareErr := comparePreScopeV7Inventories(expectedInventory, current); compareErr != nil {
			return nil, nil, compareErr
		}
		plan, planErr := buildV7MigrationActivationPlan(options)
		if planErr != nil {
			return nil, nil, planErr
		}
		return plan.cleanupOps, plan.artifacts, nil
	})
	if err != nil {
		return v7MigrationAfterSnapshotError(snapshot, err)
	}

	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil {
		return v7MigrationAfterSnapshotError(snapshot, err)
	}
	if !exists || schema != V7ConfigSchemaVersion {
		return v7MigrationAfterSnapshotError(
			snapshot,
			fmt.Errorf("migration commit did not publish schema %d", V7ConfigSchemaVersion),
		)
	}
	return nil
}

func v7MigrationAfterSnapshotError(snapshot *PreScopeV7Snapshot, cause error) error {
	if snapshot == nil {
		return cause
	}
	return fmt.Errorf(
		"v7 migration failed after creating rollback snapshot %s and audit %s; workspace recovery must complete before retrying, or restore that snapshot to downgrade: %w",
		snapshot.ArchivePath,
		snapshot.AuditPath,
		cause,
	)
}

func compareV7SnapshotResultInventory(snapshot *PreScopeV7Snapshot, inventory *preScopeV7Inventory) error {
	if snapshot == nil || inventory == nil {
		return fmt.Errorf("v7 migration snapshot inventory is unavailable")
	}
	current := publicPreScopeV7Files(inventory.files)
	if len(snapshot.Files) != len(current) {
		return fmt.Errorf("v7 migration source inventory changed after snapshot")
	}
	for index := range current {
		if snapshot.Files[index] != current[index] {
			return fmt.Errorf("v7 migration source changed after snapshot: %s", current[index].Path)
		}
	}
	return nil
}

// buildV7MigrationActivationPlan rewrites the workspace catalog in place. Schema
// 7 removes no file, so the only cleanup is the sidecar the schema-6 writer left
// behind: the loader falls back to it, and a stale schema-6 copy would decode as
// corruption once the retired fields are gone.
func buildV7MigrationActivationPlan(options V7WorkspaceDryRunOptions) (v7MigrationActivationPlan, error) {
	plan, err := DryRunV7WorkspaceConversion(options)
	if err != nil {
		return v7MigrationActivationPlan{}, err
	}
	if err = ValidateV7WorkspaceConversionPlan(plan); err != nil {
		return v7MigrationActivationPlan{}, err
	}

	dataDir := filepath.Clean(options.DataDir)
	target := filepath.Clean(plan.Artifact.Path)
	if err = RefusePreConsolidationV6SnapshotMutation(dataDir, target); err != nil {
		return v7MigrationActivationPlan{}, err
	}
	if err = RefusePreScopeV7SnapshotMutation(dataDir, target); err != nil {
		return v7MigrationActivationPlan{}, err
	}
	source, err := readV5DryRunSource(target)
	if err != nil {
		return v7MigrationActivationPlan{}, err
	}
	expectedRevision := storage.RawArtifactRevision(source)

	activation := v7MigrationActivationPlan{
		artifacts: []storage.ArtifactChange{{
			Path:             target,
			Data:             append([]byte(nil), plan.Artifact.Data...),
			Mode:             FilePermissions,
			ExpectedRevision: &expectedRevision,
		}},
	}
	backup := target + ".bak"
	info, err := os.Lstat(backup)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return v7MigrationActivationPlan{}, fmt.Errorf("inspect v7 migration cleanup file %s: %w", backup, err)
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return v7MigrationActivationPlan{}, fmt.Errorf("v7 migration cleanup target is not a regular file: %s", backup)
	default:
		activation.cleanupOps = append(activation.cleanupOps, transaction.Op{
			Kind: transaction.OpDelete, Target: backup,
		})
	}
	return activation, nil
}
