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

const V11ConfigSchemaVersion = 11

type v11DeferInstantAuditEntry struct {
	ProjectID string
	Tasks     int
}

type v11MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
	audit     []v11DeferInstantAuditEntry
}

// v11RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v11RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v11MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV11 lifts every task document to schema 10 by rewriting the
// durable deferral value from a calendar day to a canonical RFC3339Nano
// instant. Catalog and project documents publish in a single transaction after a
// byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV11() error {
	return m.migrateWorkspaceToV11(nil)
}

func (m *Manager) migrateWorkspaceToV11(hooks v11MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-11 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V10ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV10(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 10: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-10 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema == V11ConfigSchemaVersion {
		return nil
	}
	if schema > V11ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade Docket", schema, V11ConfigSchemaVersion)
	}
	if schema != V10ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V11ConfigSchemaVersion)
	}

	plan, err := buildV11MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v11 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreDeferInstantV11Snapshot(dataDir, plan.audit)
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
		if compareErr := compareV11ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV11MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v11 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V11ConfigSchemaVersion {
		return fmt.Errorf("v11 migration did not publish schema %d after rollback snapshot %s: %w", V11ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV11ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV11ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v11 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v11 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

func buildV11MigrationPlan(dataDir string) (v11MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V10ConfigSchemaVersion)
	if err != nil {
		return v11MigrationPlan{}, err
	}
	plan := v11MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v11MigrationPlan{}, fmt.Errorf("read schema-9 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV9ProjectDocument(string(source))
		if parseErr != nil {
			return v11MigrationPlan{}, fmt.Errorf("parse schema-9 project document %s: %w", path, parseErr)
		}
		lifted, liftErr := storage.V10ProjectDocumentFromV9(document, time.Local)
		if liftErr != nil {
			return v11MigrationPlan{}, fmt.Errorf("lift schema-10 project document %s: %w", path, liftErr)
		}
		published, writeErr := storage.WriteV10ProjectDocument(lifted)
		if writeErr != nil {
			return v11MigrationPlan{}, fmt.Errorf("encode schema-10 project document %s: %w", path, writeErr)
		}
		converted := 0
		for _, task := range lifted.Tasks {
			if task.SnoozedUntil != "" {
				converted++
			}
		}
		plan.audit = append(plan.audit, v11DeferInstantAuditEntry{ProjectID: project.ID, Tasks: converted})
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V11ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V11ConfigSchemaVersion); validationErr != nil {
		return v11MigrationPlan{}, fmt.Errorf("validate schema-11 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V11ConfigSchemaVersion)
	if err != nil {
		return v11MigrationPlan{}, fmt.Errorf("encode schema-11 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V11ConfigSchemaVersion)
	if err != nil {
		return v11MigrationPlan{}, err
	}
	return plan, nil
}

func createPreDeferInstantV11Snapshot(dataDir string, conversions []v11DeferInstantAuditEntry) (v11RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v11RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preDeferInstantV11SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v11RollbackSnapshot{}, precheckErr
	}
	total := 0
	projects := 0
	for _, entry := range conversions {
		total += entry.Tasks
		if entry.Tasks > 0 {
			projects++
		}
	}
	audit := []byte(v11DeferInstantAuditReport(createdAt, len(inventory.files), total, projects, conversions))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preDeferInstantV11SnapshotPrefix+"audit.md")
	if err != nil {
		return v11RollbackSnapshot{}, err
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
		return v11RollbackSnapshot{}, fmt.Errorf("pre-defer-instant v11 snapshot failed: %w", err)
	}
	return v11RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v11DeferInstantAuditReport(
	createdAt time.Time,
	files, total, projects int,
	conversions []v11DeferInstantAuditEntry,
) string {
	local := createdAt.In(time.Local)
	var report strings.Builder
	report.WriteString("# Pre-defer-instant schema-11 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n", files)
	fmt.Fprintf(&report, "- Converted deferral dates: %d\n", total)
	fmt.Fprintf(&report, "- Migrating machine timezone: `%s` (`%s`)\n\n", local.Format("MST"), local.Format("-07:00"))
	fmt.Fprintf(&report, "The `deferred` metadata value is rewritten from a `YYYY-MM-DD` calendar day to a "+
		"canonical RFC3339Nano instant on %d tasks across %d projects. Each date becomes the instant that "+
		"day began in the timezone above, which is the timezone a bare date was always implicitly read in, "+
		"so every existing deferral still expires at exactly the moment it expired at before.\n\n",
		total, projects)
	report.WriteString("That offset is frozen into the file. A deferral is now an instant rather than a day, " +
		"so a workspace synced across timezones stops waking at each machine's own midnight: a task " +
		"migrated on a UTC+2 machine keeps waking at that machine's midnight wherever the workspace is " +
		"opened. This is inherent to storing an instant and is what buys a deferral that can expire later " +
		"the same day.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. The ledger stores raw pre-format " +
		"Markdown images whose deferral values are bare dates a schema-10 reader rejects, so replaying one " +
		"would reintroduce the old value layout. This report is the only place that can explain the empty " +
		"history.\n\n")
	report.WriteString("| Project | Converted deferrals |\n| --- | --- |\n")
	for _, entry := range conversions {
		fmt.Fprintf(&report, "| `%s` | %d |\n", entry.ProjectID, entry.Tasks)
	}
	return report.String()
}
