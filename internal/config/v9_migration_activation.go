package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

const V9ConfigSchemaVersion = 9

type v9StateClockAuditEntry struct {
	ProjectID string
	Tasks     int
}

type v9MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
	audit     []v9StateClockAuditEntry
}

// v9RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v9RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v9MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV9 lifts every task document to schema 8 by giving each
// task a durable state clock. Catalog and project documents publish in a single
// transaction after a byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV9() error {
	return m.migrateWorkspaceToV9(nil)
}

func (m *Manager) migrateWorkspaceToV9(hooks v9MigrationActivationHooks) error {
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
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir, m.localDataRoot()})
	if _, recoverErr := commit.RecoverWorkspace(); recoverErr != nil {
		return fmt.Errorf("recover workspace before schema-9 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V8ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV8(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 8: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-8 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema == V9ConfigSchemaVersion {
		return nil
	}
	if schema > V9ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, V9ConfigSchemaVersion)
	}
	if schema != V8ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V9ConfigSchemaVersion)
	}

	plan, err := buildV9MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v9 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreStateClockV9Snapshot(dataDir, plan.audit)
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
		if compareErr := compareV9ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV9MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v9 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V9ConfigSchemaVersion {
		return fmt.Errorf("v9 migration did not publish schema %d after rollback snapshot %s: %w", V9ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV9ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV9ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v9 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v9 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

func buildV9MigrationPlan(dataDir string) (v9MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V8ConfigSchemaVersion)
	if err != nil {
		return v9MigrationPlan{}, err
	}
	plan := v9MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v9MigrationPlan{}, fmt.Errorf("read schema-7 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV7ProjectDocument(string(source))
		if parseErr != nil {
			return v9MigrationPlan{}, fmt.Errorf("parse schema-7 project document %s: %w", path, parseErr)
		}
		lifted := storage.V8ProjectDocumentFromV7(document)
		published, writeErr := storage.WriteV8ProjectDocument(lifted)
		if writeErr != nil {
			return v9MigrationPlan{}, fmt.Errorf("encode schema-8 project document %s: %w", path, writeErr)
		}
		plan.audit = append(plan.audit, v9StateClockAuditEntry{ProjectID: project.ID, Tasks: len(lifted.Tasks)})
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V9ConfigSchemaVersion
	if validationErr := catalog.Validate(); validationErr != nil {
		return v9MigrationPlan{}, fmt.Errorf("validate schema-9 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeDomainEnvelope(domainCatalog, catalog.WorkspaceID, &catalog)
	if err != nil {
		return v9MigrationPlan{}, fmt.Errorf("encode schema-9 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V9ConfigSchemaVersion)
	if err != nil {
		return v9MigrationPlan{}, err
	}
	return plan, nil
}

func createPreStateClockV9Snapshot(dataDir string, backfills []v9StateClockAuditEntry) (v9RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v9RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := "pre-state-clock-v9-" + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v9RollbackSnapshot{}, precheckErr
	}
	total := 0
	for _, entry := range backfills {
		total += entry.Tasks
	}
	var report strings.Builder
	report.WriteString("# Pre-state-clock schema-9 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n", len(inventory.files))
	fmt.Fprintf(&report, "- Backfilled state clocks: %d\n\n", total)
	report.WriteString("Every existing task receives `state_changed` backfilled from its `updated` " +
		"timestamp. That is an approximation, not a recovered fact: `updated` is the last touch of " +
		"any field, so a task last edited long after its state changed reads a state clock that is " +
		"too recent. The approximation self-corrects on the task's next state transition, which " +
		"overwrites the clock with the true instant.\n\n")
	report.WriteString("| Project | Backfilled tasks |\n| --- | --- |\n")
	for _, entry := range backfills {
		fmt.Fprintf(&report, "| `%s` | %d |\n", entry.ProjectID, entry.Tasks)
	}
	audit := []byte(report.String())
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, "pre-state-clock-v9-audit.md")
	if err != nil {
		return v9RollbackSnapshot{}, err
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
		return v9RollbackSnapshot{}, fmt.Errorf("pre-state-clock v9 snapshot failed: %w", err)
	}
	return v9RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}
