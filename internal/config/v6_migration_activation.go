package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

type v6MigrationActivationPlan struct {
	artifacts   []storage.ArtifactChange
	cleanupOps  []transaction.Op
	machinePath string
}

type v6MigrationActivationHooks interface {
	snapshotCreationCallbacks() preConsolidationV6SnapshotCallbacks
	didPublishSnapshot(*PreConsolidationV6Snapshot)
}

// MigrateWorkspaceToV6 activates schema 6 at the guarded startup boundary.
func (m *Manager) MigrateWorkspaceToV6() error {
	return m.migrateWorkspaceToV6(nil)
}

//nolint:govet // Each guarded migration stage keeps its error scope local.
func (m *Manager) migrateWorkspaceToV6(hooks v6MigrationActivationHooks) error {
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
	roots := []string{dataDir, m.localDataRoot()}
	commit := storage.NewCommitService(TransactionsDir(dataDir), roots).OnMachine(m.machineID())
	if _, err := commit.RecoverWorkspace(); err != nil {
		return fmt.Errorf("recover workspace before schema-6 activation: %w", err)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if schema == legacyV4ConfigSchema {
		if err := m.MigrateWorkspaceToV5(); err != nil {
			return fmt.Errorf("migrate supported schema-4 workspace through schema 5: %w", err)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("schema-4 to schema-5 migration did not publish a workspace catalog")
		}
	}
	switch {
	case schema == V6ConfigSchemaVersion:
		return nil
	case schema > V6ConfigSchemaVersion:
		return fmt.Errorf(
			"workspace schema %d is newer than supported schema %d; upgrade tt",
			schema,
			V6ConfigSchemaVersion,
		)
	case schema != V5ConfigSchemaVersion:
		return fmt.Errorf(
			"workspace schema %d cannot be migrated directly to schema %d; restore a supported v4 or v5 workspace or use a compatible tt release",
			schema,
			V6ConfigSchemaVersion,
		)
	}

	options := V6WorkspaceDryRunOptions{DataDir: dataDir}
	// Validate both the complete candidate and the publication topology before
	// creating rollback artifacts.
	preflightPlan, err := buildV6MigrationActivationPlan(options)
	if err != nil {
		return fmt.Errorf("v6 migration validation failed; workspace was not changed: %w", err)
	}

	var snapshotCallbacks preConsolidationV6SnapshotCallbacks
	if hooks != nil {
		snapshotCallbacks = hooks.snapshotCreationCallbacks()
	}
	snapshot, err := createPreConsolidationV6Snapshot(
		PreConsolidationV6SnapshotOptions{DataDir: dataDir},
		snapshotCallbacks,
	)
	if err != nil {
		return err
	}

	expectedInventory, err := inventoryPreConsolidationV6(dataDir, preflightPlan.machinePath, false)
	if err != nil {
		return v6MigrationAfterSnapshotError(snapshot, err)
	}
	if err := compareV6SnapshotResultInventory(snapshot, expectedInventory); err != nil {
		return v6MigrationAfterSnapshotError(snapshot, err)
	}
	if hooks != nil {
		hooks.didPublishSnapshot(snapshot)
	}
	currentInventory, err := inventoryPreConsolidationV6(dataDir, preflightPlan.machinePath, false)
	if err != nil {
		return v6MigrationAfterSnapshotError(snapshot, err)
	}
	if err := comparePreConsolidationV6Inventories(expectedInventory, currentInventory); err != nil {
		return v6MigrationAfterSnapshotError(snapshot, err)
	}

	err = commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
		current, inventoryErr := inventoryPreConsolidationV6(dataDir, preflightPlan.machinePath, false)
		if inventoryErr != nil {
			return nil, nil, inventoryErr
		}
		if compareErr := comparePreConsolidationV6Inventories(expectedInventory, current); compareErr != nil {
			return nil, nil, compareErr
		}
		plan, planErr := buildV6MigrationActivationPlan(options)
		if planErr != nil {
			return nil, nil, planErr
		}
		return plan.cleanupOps, plan.artifacts, nil
	})
	if err != nil {
		return v6MigrationAfterSnapshotError(snapshot, err)
	}

	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil {
		return v6MigrationAfterSnapshotError(snapshot, err)
	}
	if !exists || schema != V6ConfigSchemaVersion {
		return v6MigrationAfterSnapshotError(
			snapshot,
			fmt.Errorf("migration commit did not publish schema %d", V6ConfigSchemaVersion),
		)
	}
	return nil
}

func v6MigrationAfterSnapshotError(snapshot *PreConsolidationV6Snapshot, cause error) error {
	if snapshot == nil {
		return cause
	}
	return fmt.Errorf(
		"v6 migration failed after creating rollback snapshot %s and audit %s; workspace recovery must complete before retrying, or restore that snapshot to downgrade: %w",
		snapshot.ArchivePath,
		snapshot.AuditPath,
		cause,
	)
}

func compareV6SnapshotResultInventory(
	snapshot *PreConsolidationV6Snapshot,
	inventory *preConsolidationV6Inventory,
) error {
	if snapshot == nil || inventory == nil {
		return fmt.Errorf("v6 migration snapshot inventory is unavailable")
	}
	current := publicPreConsolidationV6Files(inventory.files)
	if len(snapshot.Files) != len(current) {
		return fmt.Errorf("v6 migration source inventory changed after snapshot")
	}
	for index := range current {
		if snapshot.Files[index] != current[index] {
			return fmt.Errorf("v6 migration source changed after snapshot: %s", current[index].Path)
		}
	}
	return nil
}

//nolint:govet // Builder operations use scoped errors while the source-plan error remains available.
func buildV6MigrationActivationPlan(options V6WorkspaceDryRunOptions) (v6MigrationActivationPlan, error) {
	plan, err := DryRunV6WorkspaceConversion(options)
	if err != nil {
		return v6MigrationActivationPlan{}, err
	}
	if err := ValidateV6WorkspaceConversionPlan(plan); err != nil {
		return v6MigrationActivationPlan{}, err
	}

	machinePath, err := preConsolidationV6MachineSource(plan)
	if err != nil {
		return v6MigrationActivationPlan{}, err
	}
	protectedID := plan.Domains.Domains.Catalog.SystemUnfiledProjectID
	var protected V6ProjectDocumentCandidate
	for _, project := range plan.Projects {
		if project.ProjectID == protectedID {
			protected = project
			break
		}
	}
	if protected.ProjectID == "" {
		return v6MigrationActivationPlan{}, fmt.Errorf("v6 activation has no protected Unfiled project candidate")
	}

	builder := newV6ActivationBuilder(
		options.DataDir,
		filepath.Dir(protected.SourcePath),
		filepath.Dir(protected.Path),
	)
	for _, candidate := range plan.Artifacts {
		if err := builder.addArtifact(candidate); err != nil {
			return v6MigrationActivationPlan{}, err
		}
		switch candidate.Kind {
		case "workspace", "planning", "machine", "project":
			if err := builder.addOptionalFileDelete(candidate.SourcePath + ".bak"); err != nil {
				return v6MigrationActivationPlan{}, err
			}
		}
	}
	for _, attachment := range plan.Attachments {
		if err := builder.addAttachmentMove(attachment); err != nil {
			return v6MigrationActivationPlan{}, err
		}
	}
	for _, obsolete := range plan.ObsoleteAttachments {
		if err := builder.addCleanupTree(obsolete.Path); err != nil {
			return v6MigrationActivationPlan{}, err
		}
	}
	if err := builder.addCleanupTree(UndoLedgerDir(options.DataDir)); err != nil {
		return v6MigrationActivationPlan{}, err
	}
	if err := builder.addProtectedProjectMove(); err != nil {
		return v6MigrationActivationPlan{}, err
	}

	result, err := builder.finish()
	if err != nil {
		return v6MigrationActivationPlan{}, err
	}
	result.machinePath = machinePath
	return result, nil
}
