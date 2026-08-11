package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

const V8ConfigSchemaVersion = 8

type v8TagAuditEntry struct {
	ProjectID string
	TaskID    string
	From      string
	To        string
	Collision bool
}

type v8MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
	audit     []v8TagAuditEntry
}

// MigrateWorkspaceToV8 canonicalizes every project and task tag at one guarded
// schema boundary. Catalog and active/archived project documents publish in a
// single transaction after a byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV8() error {
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
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir, LocalDataRoot()})
	if _, recoverErr := commit.RecoverWorkspace(); recoverErr != nil {
		return fmt.Errorf("recover workspace before schema-8 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V7ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV7(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 7: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-7 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema == V8ConfigSchemaVersion {
		return nil
	}
	if schema > V8ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, V8ConfigSchemaVersion)
	}
	if schema != V7ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V8ConfigSchemaVersion)
	}

	plan, err := buildV8MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v8 migration validation failed; workspace was not changed: %w", err)
	}
	archivePath, auditPath, err := createPreTagV8Snapshot(dataDir, plan.audit)
	if err != nil {
		return err
	}
	if commitErr := commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
		current, planErr := buildV8MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v8 migration failed after creating rollback snapshot %s and audit %s: %w", archivePath, auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V8ConfigSchemaVersion {
		return fmt.Errorf("v8 migration did not publish schema %d after rollback snapshot %s: %w", V8ConfigSchemaVersion, archivePath, err)
	}
	return nil
}

func buildV8MigrationPlan(dataDir string) (v8MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readV8SourceCatalog(catalogPath)
	if err != nil {
		return v8MigrationPlan{}, err
	}
	plan := v8MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		project.Tags, plan.audit, err = normalizeV8Tags(project.ID, "", project.Tags, true, plan.audit)
		if err != nil {
			return v8MigrationPlan{}, err
		}
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v8MigrationPlan{}, fmt.Errorf("read schema-6 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV6ProjectDocument(string(source))
		if parseErr != nil {
			return v8MigrationPlan{}, fmt.Errorf("parse schema-6 project document %s: %w", path, parseErr)
		}
		for taskIndex := range document.Tasks {
			task := &document.Tasks[taskIndex]
			task.Tags, plan.audit, err = normalizeV8Tags(project.ID, task.ID, task.Tags, false, plan.audit)
			if err != nil {
				return v8MigrationPlan{}, err
			}
		}
		published, writeErr := storage.WriteV7ProjectDocument(document)
		if writeErr != nil {
			return v8MigrationPlan{}, fmt.Errorf("encode schema-7 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V8ConfigSchemaVersion
	if validationErr := catalog.Validate(); validationErr != nil {
		return v8MigrationPlan{}, fmt.Errorf("validate schema-8 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeDomainEnvelope(domainCatalog, catalog.WorkspaceID, &catalog)
	if err != nil {
		return v8MigrationPlan{}, fmt.Errorf("encode schema-8 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	plan.cleanup, err = v8UndoCleanupOps(UndoLedgerDir(dataDir))
	if err != nil {
		return v8MigrationPlan{}, err
	}
	return plan, nil
}

func readV8SourceCatalog(path string) ([]byte, WorkspaceCatalog, error) {
	return readMigrationSourceCatalog(path, V7ConfigSchemaVersion)
}

// readMigrationSourceCatalog strictly decodes the catalog a migration consumes
// and refuses anything that is not exactly the expected predecessor schema.
func readMigrationSourceCatalog(path string, expected int) ([]byte, WorkspaceCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, WorkspaceCatalog{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope domainEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return nil, WorkspaceCatalog{}, fmt.Errorf("decode schema-%d workspace envelope: %w", expected, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, WorkspaceCatalog{}, err
	}
	if envelope.SchemaVersion != CurrentDomainSchemaVersion || envelope.Kind != domainCatalog.name() {
		return nil, WorkspaceCatalog{}, fmt.Errorf("schema-%d workspace envelope has invalid schema or kind", expected)
	}
	payload := json.NewDecoder(bytes.NewReader(envelope.Data))
	payload.DisallowUnknownFields()
	var catalog WorkspaceCatalog
	if err := payload.Decode(&catalog); err != nil {
		return nil, WorkspaceCatalog{}, fmt.Errorf("decode schema-%d workspace payload: %w", expected, err)
	}
	if err := requireJSONEOF(payload); err != nil {
		return nil, WorkspaceCatalog{}, err
	}
	if catalog.SchemaVersion != expected || catalog.WorkspaceID != envelope.WorkspaceID {
		return nil, WorkspaceCatalog{}, fmt.Errorf("schema-%d workspace payload has invalid schema or identity", expected)
	}
	return data, catalog, nil
}

func normalizeV8Tags(projectID, taskID string, tags []string, sorted bool, audit []v8TagAuditEntry) ([]string, []v8TagAuditEntry, error) {
	next := make([]string, 0, len(tags))
	seen := make(map[string]string, len(tags))
	for _, raw := range tags {
		normalized := models.NormalizeTag(raw)
		if err := models.ValidateTag(normalized); err != nil {
			return nil, audit, fmt.Errorf("normalize tag %q in project %q task %q: %w", raw, projectID, taskID, err)
		}
		previous, duplicate := seen[normalized]
		if raw != normalized || duplicate {
			audit = append(audit, v8TagAuditEntry{
				ProjectID: projectID, TaskID: taskID, From: raw, To: normalized,
				Collision: duplicate && previous != raw,
			})
		}
		if duplicate {
			continue
		}
		seen[normalized] = raw
		next = append(next, normalized)
	}
	if sorted {
		sort.Strings(next)
	}
	return next, audit, nil
}

func v8UndoCleanupOps(root string) ([]transaction.Op, error) {
	return undoHistoryCleanupOps(root, V8ConfigSchemaVersion)
}

// undoHistoryCleanupOps deletes the whole undo ledger. Every schema boundary
// that changes the retained artifact bytes has to drop it: an entry replayed
// after the boundary would restore a document the new loader rejects.
func undoHistoryCleanupOps(root string, schema int) ([]transaction.Op, error) {
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("inspect undo history before schema-%d migration: %w", schema, err)
	}
	var files, dirs []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("schema-%d migration refuses symlink in undo history: %s", schema, path)
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
		} else {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], string(os.PathSeparator)) > strings.Count(dirs[j], string(os.PathSeparator))
	})
	ops := make([]transaction.Op, 0, len(files)+len(dirs))
	for _, path := range append(files, dirs...) {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
	}
	return ops, nil
}

func createPreTagV8Snapshot(dataDir string, tagChanges []v8TagAuditEntry) (string, string, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return "", "", err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := "pre-tag-v8-" + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return "", "", precheckErr
	}
	var report strings.Builder
	report.WriteString("# Pre-tag schema-8 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n", len(inventory.files))
	fmt.Fprintf(&report, "- Renames and collisions: %d\n\n", len(tagChanges))
	report.WriteString("| Project | Task | From | To | Collision |\n| --- | --- | --- | --- | --- |\n")
	for _, entry := range tagChanges {
		fmt.Fprintf(&report, "| `%s` | `%s` | `%s` | `%s` | %t |\n", entry.ProjectID, entry.TaskID, entry.From, entry.To, entry.Collision)
	}
	audit := []byte(report.String())
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, "pre-tag-v8-audit.md")
	if err != nil {
		return "", "", err
	}
	absent := ""
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir})
	_, err = commit.CommitArtifacts([]storage.ArtifactChange{
		{Path: archivePath, Data: archive, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
		{Path: auditPath, Data: audit, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
	})
	if err != nil {
		return "", "", fmt.Errorf("pre-tag v8 snapshot failed: %w", err)
	}
	return archivePath, auditPath, nil
}
