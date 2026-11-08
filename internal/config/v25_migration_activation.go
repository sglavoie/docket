package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
	"github.com/sglavoie/docket/internal/util"
)

// V25ConfigSchemaVersion is the first catalog schema that admits a project
// alias. The catalog is decoded with unknown fields disallowed, so an older
// Docket would refuse an alias-bearing catalog as malformed rather than as
// newer than it supports; advancing the number is what makes that refusal say
// what it means. Task Markdown stays at schema 20.
const V25ConfigSchemaVersion = 25

// v25ActivationLockName is the lock one activating process holds from before
// its snapshot until the schema-25 catalog is published.
const v25ActivationLockName = "v25-activation.lock"

// validateCatalogProjectAlias holds one catalog project's alias to the schema
// that carries it: none before schema 25, and only what
// models.ValidateProjectAlias admits from then on. Both the load and the save
// path reach it, so neither a hand-edited catalog nor an edit surface can store
// an alias the other would refuse.
func validateCatalogProjectAlias(schema int, p CatalogProject) error {
	if p.Alias == "" {
		return nil
	}
	if schema < V25ConfigSchemaVersion {
		return fmt.Errorf(
			"workspace catalog: project %q carries an alias, which requires schema version %d, got %d",
			p.ID, V25ConfigSchemaVersion, schema,
		)
	}
	if err := models.ValidateProjectAlias(p.Alias); err != nil {
		return fmt.Errorf("workspace catalog: project %q has invalid alias: %w", p.ID, err)
	}
	return nil
}

type v25MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v25RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v25RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v25MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV25 advances a schema-24 catalog to 25, which admits the
// optional project alias. No project carries one yet, so the catalog's bytes
// change at the schema number alone and no task document is touched.
//
// The boundary still publishes a byte-exact rollback archive and audit before
// it activates, as every boundary that changes retained bytes does, and the
// catalog publishes in one transaction with its expected revision beside the
// undo-ledger cleanup.
func (m *Manager) MigrateWorkspaceToV25() error {
	return m.migrateWorkspaceToV25(nil)
}

func (m *Manager) migrateWorkspaceToV25(hooks v25MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-25 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V24ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV24(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 24: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-24 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V25ConfigSchemaVersion {
		if schema <= SupportedConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade Docket", schema, SupportedConfigVersion)
	}
	if schema != V24ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V25ConfigSchemaVersion)
	}

	if _, err = buildV25MigrationPlan(dataDir); err != nil {
		return fmt.Errorf("v25 migration validation failed; workspace was not changed: %w", err)
	}
	return withV25ActivationLock(dataDir, func() error {
		// A concurrent opener may have activated the boundary while this one
		// waited for the lock. Its archive and catalog are this one's too, and a
		// second archive minted in the same second would collide with its name.
		if current, _, schemaErr := workspaceCatalogPayloadSchema(dataDir); schemaErr == nil && current == V25ConfigSchemaVersion {
			return nil
		}
		snapshot, snapshotErr := createPreProjectAliasV25Snapshot(dataDir)
		if snapshotErr != nil {
			return snapshotErr
		}
		if hooks != nil {
			hooks.didPublishSnapshot(snapshot.archivePath, snapshot.auditPath)
		}
		if commitErr := commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
			live, inventoryErr := inventoryPreScopeV7(dataDir, false)
			if inventoryErr != nil {
				return nil, nil, inventoryErr
			}
			if compareErr := compareV25ArchivedInventory(snapshot.inventory, live); compareErr != nil {
				return nil, nil, compareErr
			}
			current, planErr := buildV25MigrationPlan(dataDir)
			if planErr != nil {
				return nil, nil, planErr
			}
			return current.cleanup, current.artifacts, nil
		}); commitErr != nil {
			return fmt.Errorf("v25 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
		}
		published, publishedExists, publishedErr := workspaceCatalogPayloadSchema(dataDir)
		if publishedErr != nil || !publishedExists || published != V25ConfigSchemaVersion {
			return fmt.Errorf("v25 migration did not publish schema %d after rollback snapshot %s: %w", V25ConfigSchemaVersion, snapshot.archivePath, publishedErr)
		}
		return nil
	})
}

// withV25ActivationLock serializes the schema-25 activation across processes
// from before its snapshot until its catalog is published. The workspace commit
// lock cannot serve, because the snapshot and the activation each take it in
// turn. The lock file lives beside the journal, which neither sync nor any
// migration snapshot carries.
func withV25ActivationLock(dataDir string, fn func() error) error {
	if err := os.MkdirAll(TransactionsDir(dataDir), DirPermissions); err != nil {
		return fmt.Errorf("prepare %s: %w", TransactionsDir(dataDir), err)
	}
	return util.WithFileLock(filepath.Join(TransactionsDir(dataDir), v25ActivationLockName), true, fn)
}

// compareV25ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV25ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v25 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v25 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV25MigrationPlan strictly reads and validates the schema-24 catalog
// before it plans a byte, so a catalog this build cannot read is refused rather
// than advanced. Only the schema number changes: a schema-24 catalog cannot
// hold an alias, and the field is omitted when empty, so the published catalog
// is the source apart from that number. Task Markdown is not read or rewritten,
// since its schema does not move here.
//
// This boundary stages no lens artifact: no lens can name an alias.
func buildV25MigrationPlan(dataDir string) (v25MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V24ConfigSchemaVersion)
	if err != nil {
		return v25MigrationPlan{}, err
	}
	if validationErr := catalog.validateAtSchema(V24ConfigSchemaVersion); validationErr != nil {
		return v25MigrationPlan{}, fmt.Errorf("validate schema-24 catalog: %w", validationErr)
	}
	catalog.SchemaVersion = V25ConfigSchemaVersion
	published, err := encodeMigrationCatalogEnvelope(&catalog, V25ConfigSchemaVersion)
	if err != nil {
		return v25MigrationPlan{}, fmt.Errorf("encode schema-25 catalog: %w", err)
	}
	revision := storage.RawArtifactRevision(sourceBytes)
	plan := v25MigrationPlan{artifacts: []storage.ArtifactChange{{
		Path: catalogPath, Data: published, Mode: FilePermissions, ExpectedRevision: &revision,
	}}}
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: an entry replayed after it would put back schema-24 catalog
	// bytes, rolling the catalog behind a boundary it has already crossed.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V25ConfigSchemaVersion)
	if err != nil {
		return v25MigrationPlan{}, err
	}
	return plan, nil
}

func createPreProjectAliasV25Snapshot(dataDir string) (v25RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v25RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preProjectAliasV25SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v25RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v25ProjectAliasAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preProjectAliasV25SnapshotPrefix+"audit.md")
	if err != nil {
		return v25RollbackSnapshot{}, err
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
		return v25RollbackSnapshot{}, fmt.Errorf("pre-project-alias v25 snapshot failed: %w", err)
	}
	return v25RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v25ProjectAliasAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-project-alias schema-25 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Source catalog schema: %d\n", V24ConfigSchemaVersion)
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary advances the workspace catalog to schema 25, which admits an optional " +
		"`alias` on each project: a short display label drawn in place of the slug. No project carries one " +
		"yet, so the catalog changes at its schema number alone. Task Markdown stays at schema 20 and no " +
		"project document is rewritten.\n\n")
	report.WriteString("The `.tt/` tombstone keeps declaring schema 24. It only has to be newer than every " +
		"Docket that still reads `.tt/`, and those all stop at schema 23.\n\n")
	report.WriteString("Saved lens definitions are not rewritten because no lens can name an alias.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw schema-24 " +
		"catalog bytes, so replaying one would roll the catalog back behind a boundary it has already " +
		"crossed. This report is the only place that can explain the empty history.\n\n")
	report.WriteString("Conflict bundles under `.docket/conflicts/` are not rewritten, here or at any " +
		"boundary.\n")
	return report.String()
}
