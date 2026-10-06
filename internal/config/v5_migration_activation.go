package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/transaction"
)

var legacyV4ArchiveFileName = regexp.MustCompile(`^completed(?:\.[1-9][0-9]*)?\.md$`)

const (
	legacyV4ArchiveIndexFileName = ".completed.index.json"
	legacyV4BlockersFileName     = "blockers.md"
	legacyV4ProjectUndoDirName   = ".undo"
	legacyV4ProjectBackupName    = ".tasks.md.bak"
)

type v5MigrationActivationPlan struct {
	artifacts  []storage.ArtifactChange
	cleanupOps []transaction.Op
	audit      []byte
}

type v5MigrationActivationHooks interface {
	didPublishSnapshot(*PreSimplificationV5Snapshot)
}

type v5MigrationTaskAuditRow struct {
	ProjectID string
	SourceID  string
	Entry     format.V5TaskMigrationAuditEntry
}

// MigrateWorkspaceToV5 activates the snapshot-first v5 migration when the
// workspace catalog is schema 4. Fresh schema-5 workspaces and already-migrated
// workspaces are no-ops.
func (m *Manager) MigrateWorkspaceToV5() error {
	return m.migrateWorkspaceToV5(nil)
}

func (m *Manager) migrateWorkspaceToV5(hooks v5MigrationActivationHooks) error {
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
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	switch {
	case schema == V5ConfigSchemaVersion:
		return nil
	case schema > V5ConfigSchemaVersion:
		return fmt.Errorf(
			"workspace schema %d is newer than supported schema %d; upgrade tt",
			schema,
			V5ConfigSchemaVersion,
		)
	case schema != legacyV4ConfigSchema:
		return fmt.Errorf(
			"workspace schema %d cannot be migrated directly to schema %d; restore a supported v4 workspace or use a compatible tt release",
			schema,
			V5ConfigSchemaVersion,
		)
	}

	// Validate the entire conversion graph before publishing rollback artifacts.
	// This catches future task schemas, corrupt notes, invalid references, and
	// unsafe cleanup topology without changing the workspace.
	preflightPlan, err := buildV5MigrationActivationPlan(dataDir, time.Local)
	if err != nil {
		return fmt.Errorf("v5 migration validation failed; workspace was not changed: %w", err)
	}

	snapshot, err := createPreSimplificationV5SnapshotWithMigrationAudit(dataDir, preflightPlan.audit)
	if err != nil {
		return err
	}
	if hooks != nil {
		hooks.didPublishSnapshot(snapshot)
	}
	expectedInventory, err := inventoryPreSimplificationV5(dataDir, false)
	if err != nil {
		return v5MigrationAfterSnapshotError(snapshot, err)
	}
	if compareErr := compareV5SnapshotResultInventory(snapshot, expectedInventory); compareErr != nil {
		return v5MigrationAfterSnapshotError(snapshot, compareErr)
	}

	roots := []string{dataDir, m.localDataRoot()}
	commit := storage.NewCommitService(TransactionsDir(dataDir), roots).OnMachine(m.machineID())
	err = commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
		currentInventory, inventoryErr := inventoryPreSimplificationV5(dataDir, false)
		if inventoryErr != nil {
			return nil, nil, inventoryErr
		}
		if compareErr := comparePreSimplificationV5Inventories(expectedInventory, currentInventory); compareErr != nil {
			return nil, nil, compareErr
		}
		plan, planErr := buildV5MigrationActivationPlan(dataDir, time.Local)
		if planErr != nil {
			return nil, nil, planErr
		}
		return plan.cleanupOps, plan.artifacts, nil
	})
	if err != nil {
		return v5MigrationAfterSnapshotError(snapshot, err)
	}

	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil {
		return v5MigrationAfterSnapshotError(snapshot, err)
	}
	if !exists || schema != V5ConfigSchemaVersion {
		return v5MigrationAfterSnapshotError(
			snapshot,
			fmt.Errorf("migration commit did not publish schema %d", V5ConfigSchemaVersion),
		)
	}
	return nil
}

func workspaceCatalogPayloadSchema(dataDir string) (int, bool, error) {
	path := WorkspaceCatalogPath(dataDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read workspace catalog schema: %w", err)
	}
	env, err := decodeEnvelope(data)
	if err != nil {
		return 0, false, fmt.Errorf("decode workspace catalog envelope: %w", err)
	}
	if env.SchemaVersion > CurrentDomainSchemaVersion {
		return 0, false, &SchemaTooNewError{
			Artifact:  "workspace envelope",
			Found:     env.SchemaVersion,
			Supported: CurrentDomainSchemaVersion,
		}
	}
	if env.Kind != domainCatalog.name() {
		return 0, false, fmt.Errorf("workspace catalog artifact contains %q domain", env.Kind)
	}
	if err := validateWorkspaceID(env.WorkspaceID); err != nil {
		return 0, false, fmt.Errorf("workspace catalog identity: %w", err)
	}
	var marker struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(env.Data, &marker); err != nil {
		return 0, false, fmt.Errorf("decode workspace catalog payload schema: %w", err)
	}
	if marker.SchemaVersion <= 0 {
		return 0, false, fmt.Errorf("workspace catalog payload has no valid schema version")
	}
	return marker.SchemaVersion, true, nil
}

func v5MigrationAfterSnapshotError(snapshot *PreSimplificationV5Snapshot, cause error) error {
	if snapshot == nil {
		return cause
	}
	return fmt.Errorf(
		"v5 migration failed after creating rollback snapshot %s and audit %s; workspace recovery must complete before retrying, or restore that snapshot to downgrade: %w",
		snapshot.ArchivePath,
		snapshot.AuditPath,
		cause,
	)
}

func compareV5SnapshotResultInventory(
	snapshot *PreSimplificationV5Snapshot,
	inventory *preSimplificationV5Inventory,
) error {
	if snapshot == nil || inventory == nil {
		return fmt.Errorf("v5 migration snapshot inventory is unavailable")
	}
	expectedArchived := publicPreSimplificationV5Files(inventory.archived)
	expectedAttachments := publicPreSimplificationV5Files(inventory.attachments)
	if err := compareV5SnapshotFiles("archived", snapshot.ArchivedFiles, expectedArchived); err != nil {
		return err
	}
	return compareV5SnapshotFiles("attachment", snapshot.Attachments, expectedAttachments)
}

func compareV5SnapshotFiles(kind string, left, right []PreSimplificationV5SnapshotFile) error {
	if len(left) != len(right) {
		return fmt.Errorf("v5 migration %s inventory changed after snapshot", kind)
	}
	for index := range left {
		if left[index] != right[index] {
			return fmt.Errorf("v5 migration source changed after snapshot: %s", right[index].Path)
		}
	}
	return nil
}
