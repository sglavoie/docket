package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

const V16ConfigSchemaVersion = 16

type v16MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v16RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v16RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v16MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV16 marks the workspace as the home of portable named lens
// definitions. It is the first boundary that changes no task content: task
// Markdown stays at schema 14, so the plan validates every project document
// strictly and rewrites none. The catalog publishes in a single transaction
// after a byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV16() error {
	return m.migrateWorkspaceToV16(nil)
}

func (m *Manager) migrateWorkspaceToV16(hooks v16MigrationActivationHooks) error {
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
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir, m.localDataRoot()}).OnMachine(m.machineID())
	if _, recoverErr := commit.RecoverWorkspace(); recoverErr != nil {
		return fmt.Errorf("recover workspace before schema-16 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V15ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV15(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 15: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-15 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V16ConfigSchemaVersion {
		if schema <= CurrentConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, CurrentConfigVersion)
	}
	if schema != V15ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V16ConfigSchemaVersion)
	}

	_, err = buildV16MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v16 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPrePortableLensesV16Snapshot(dataDir)
	if err != nil {
		return err
	}
	if hooks != nil {
		hooks.didPublishSnapshot(snapshot.archivePath, snapshot.auditPath)
	}
	if commitErr := commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
		live, inventoryErr := inventoryPreScopeV7(dataDir, false)
		if inventoryErr != nil {
			return nil, nil, inventoryErr
		}
		if compareErr := compareV16ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV16MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v16 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V16ConfigSchemaVersion {
		return fmt.Errorf("v16 migration did not publish schema %d after rollback snapshot %s: %w", V16ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV16ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV16ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v16 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v16 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV16MigrationPlan publishes exactly one artifact, the catalog. Task
// Markdown does not move at this boundary, so every project document is read
// and validated as strict schema 14 and none is rewritten: a workspace this
// build cannot read is still refused before anything is published, which is the
// guarantee the source scan exists for.
func buildV16MigrationPlan(dataDir string) (v16MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V15ConfigSchemaVersion)
	if err != nil {
		return v16MigrationPlan{}, err
	}
	plan := v16MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v16MigrationPlan{}, fmt.Errorf("read schema-14 project document %s: %w", path, readErr)
		}
		if _, parseErr := storage.ParseV14ProjectDocument(string(source)); parseErr != nil {
			return v16MigrationPlan{}, fmt.Errorf("parse schema-14 project document %s: %w", path, parseErr)
		}
	}
	catalog.SchemaVersion = V16ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V16ConfigSchemaVersion); validationErr != nil {
		return v16MigrationPlan{}, fmt.Errorf("validate schema-16 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V16ConfigSchemaVersion)
	if err != nil {
		return v16MigrationPlan{}, fmt.Errorf("encode schema-16 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	// Undo entries snapshot `.tt/workspace.json` alongside task Markdown, so an
	// entry recorded before this boundary carries a schema-15 catalog. Replaying
	// one would silently walk the workspace back across the boundary and strand
	// the lens definitions this schema announces.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V16ConfigSchemaVersion)
	if err != nil {
		return v16MigrationPlan{}, err
	}
	return plan, nil
}

func createPrePortableLensesV16Snapshot(dataDir string) (v16RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v16RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := prePortableLensesV16SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v16RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v16PortableLensesAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, prePortableLensesV16SnapshotPrefix+"audit.md")
	if err != nil {
		return v16RollbackSnapshot{}, err
	}
	// The archive owns the bytes now; the activation guard needs only the
	// fingerprints, so releasing them keeps the whole workspace from being held
	// in memory across the commit.
	for index := range inventory.files {
		inventory.files[index].data = nil
	}
	absent := ""
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir})
	_, err = commit.CommitArtifacts([]storage.ArtifactChange{
		{Path: archivePath, Data: archive, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
		{Path: auditPath, Data: audit, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
	})
	if err != nil {
		return v16RollbackSnapshot{}, fmt.Errorf("pre-portable-lenses v16 snapshot failed: %w", err)
	}
	return v16RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v16PortableLensesAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-portable-lenses schema-16 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary moves named lens definitions into the workspace, where they travel with " +
		"the rest of the synced tree instead of staying on the machine that authored them. Which lens is open " +
		"and which theme is chosen remain machine-local. The catalog is the only artifact this boundary " +
		"rewrites.\n\n")
	report.WriteString("Task Markdown stays at schema 14. This is the first boundary that deliberately does not " +
		"advance it: the pairing between a catalog version and the task Markdown version it holds is " +
		"descriptive of boundaries that changed task grammar, and nothing derives one version from the other. " +
		"Every project document is still read and validated as strict schema 14 before anything is published, " +
		"so an unreadable workspace is refused rather than half-migrated.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries snapshot " +
		"`.tt/workspace.json` beside task Markdown, so an entry recorded before activation carries a " +
		"schema-15 catalog; replaying it would walk the workspace back across the boundary. This report is " +
		"the only place that can explain the empty history.\n")
	return report.String()
}
