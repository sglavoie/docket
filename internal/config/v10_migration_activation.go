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

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

const V10ConfigSchemaVersion = 10

type v10DeferRenameAuditEntry struct {
	ProjectID string
	Tasks     int
}

type v10MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
	audit     []v10DeferRenameAuditEntry
}

// v10RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v10RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v10MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV10 lifts every task document to schema 9 by renaming the
// durable deferral-date metadata key. Catalog and project documents publish in a
// single transaction after a byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV10() error {
	return m.migrateWorkspaceToV10(nil)
}

func (m *Manager) migrateWorkspaceToV10(hooks v10MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-10 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V9ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV9(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 9: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-9 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema == V10ConfigSchemaVersion {
		return nil
	}
	if schema > V10ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade Docket", schema, V10ConfigSchemaVersion)
	}
	if schema != V9ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V10ConfigSchemaVersion)
	}

	plan, err := buildV10MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v10 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreDeferRenameV10Snapshot(dataDir, plan.audit)
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
		if compareErr := compareV10ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV10MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v10 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V10ConfigSchemaVersion {
		return fmt.Errorf("v10 migration did not publish schema %d after rollback snapshot %s: %w", V10ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV10ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV10ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v10 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v10 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

func buildV10MigrationPlan(dataDir string) (v10MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V9ConfigSchemaVersion)
	if err != nil {
		return v10MigrationPlan{}, err
	}
	plan := v10MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v10MigrationPlan{}, fmt.Errorf("read schema-8 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV8ProjectDocument(string(source))
		if parseErr != nil {
			return v10MigrationPlan{}, fmt.Errorf("parse schema-8 project document %s: %w", path, parseErr)
		}
		lifted := storage.V9ProjectDocumentFromV8(document)
		published, writeErr := storage.WriteV9ProjectDocument(lifted)
		if writeErr != nil {
			return v10MigrationPlan{}, fmt.Errorf("encode schema-9 project document %s: %w", path, writeErr)
		}
		renamed := 0
		for _, task := range lifted.Tasks {
			if task.SnoozedUntil != "" {
				renamed++
			}
		}
		plan.audit = append(plan.audit, v10DeferRenameAuditEntry{ProjectID: project.ID, Tasks: renamed})
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V10ConfigSchemaVersion
	if validationErr := catalog.Validate(); validationErr != nil {
		return v10MigrationPlan{}, fmt.Errorf("validate schema-10 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeDomainEnvelope(domainCatalog, catalog.WorkspaceID, &catalog)
	if err != nil {
		return v10MigrationPlan{}, fmt.Errorf("encode schema-10 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V10ConfigSchemaVersion)
	if err != nil {
		return v10MigrationPlan{}, err
	}
	return plan, nil
}

func createPreDeferRenameV10Snapshot(dataDir string, renames []v10DeferRenameAuditEntry) (v10RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v10RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preDeferRenameV10SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v10RollbackSnapshot{}, precheckErr
	}
	total := 0
	projects := 0
	for _, entry := range renames {
		total += entry.Tasks
		if entry.Tasks > 0 {
			projects++
		}
	}
	var report strings.Builder
	report.WriteString("# Pre-defer-rename schema-10 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n", len(inventory.files))
	fmt.Fprintf(&report, "- Renamed metadata keys: %d\n\n", total)
	fmt.Fprintf(&report, "The metadata key `snoozed` is renamed to `deferred` on %d tasks across %d "+
		"projects. No value is reinterpreted: each date keeps the same `YYYY-MM-DD` calendar day and "+
		"the same position in the task metadata comment, so only the key spelling changes.\n\n",
		total, projects)
	report.WriteString("Undo/redo history is dropped at this boundary. The ledger stores raw " +
		"pre-rename Markdown images that a schema-9 reader cannot parse, so replaying one would " +
		"reintroduce the old key. This report is the only place that can explain the empty history.\n\n")
	report.WriteString("| Project | Renamed keys |\n| --- | --- |\n")
	for _, entry := range renames {
		fmt.Fprintf(&report, "| `%s` | %d |\n", entry.ProjectID, entry.Tasks)
	}
	audit := []byte(report.String())
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preDeferRenameV10SnapshotPrefix+"audit.md")
	if err != nil {
		return v10RollbackSnapshot{}, err
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
		return v10RollbackSnapshot{}, fmt.Errorf("pre-defer-rename v10 snapshot failed: %w", err)
	}
	return v10RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}
