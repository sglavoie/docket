package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
		return 0, false, fmt.Errorf(
			"workspace envelope schema %d is newer than supported schema %d; upgrade tt",
			env.SchemaVersion,
			CurrentDomainSchemaVersion,
		)
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

//nolint:gocyclo // The activation plan validates and assembles one indivisible migration transaction.
func buildV5MigrationActivationPlan(dataDir string, location *time.Location) (v5MigrationActivationPlan, error) {
	if location == nil {
		return v5MigrationActivationPlan{}, fmt.Errorf("v5 migration requires a local timezone")
	}
	domainPlan, err := DryRunV5WorkspaceConversion(V5WorkspaceDryRunOptions{DataDir: dataDir})
	if err != nil {
		return v5MigrationActivationPlan{}, err
	}

	reintegrationInput := storage.V5TaskReintegrationInput{
		Projects: make([]storage.V5ProjectTaskReintegrationInput, 0, len(domainPlan.Domains.Domains.Catalog.Projects)),
	}
	var blockerPaths []string
	var taskAudit []v5MigrationTaskAuditRow
	for _, project := range domainPlan.Domains.Domains.Catalog.Projects {
		projectDir := filepath.Join(dataDir, "projects", project.Slug)
		activePath := storage.TasksFilePath(projectDir)
		active, conversionErr := convertLegacyV4TaskFile(activePath, location)
		if conversionErr != nil {
			return v5MigrationActivationPlan{}, fmt.Errorf("convert active tasks for project %q: %w", project.ID, conversionErr)
		}
		if active.ProjectName != project.Name {
			return v5MigrationActivationPlan{}, fmt.Errorf(
				"project %q task document name %q does not match catalog name %q",
				project.ID,
				active.ProjectName,
				project.Name,
			)
		}
		activeSourceID, relErr := filepath.Rel(dataDir, activePath)
		if relErr != nil {
			return v5MigrationActivationPlan{}, relErr
		}
		taskAudit = appendV5MigrationTaskAudit(
			taskAudit,
			project.ID,
			filepath.ToSlash(activeSourceID),
			active.Audit,
		)

		blockersPath := filepath.Join(projectDir, legacyV4BlockersFileName)
		if blockersData, found, readErr := readOptionalV5MigrationFile(blockersPath); readErr != nil {
			return v5MigrationActivationPlan{}, readErr
		} else if found {
			blockers, decodeErr := storage.DecodeLegacyV4Blockers(string(blockersData))
			if decodeErr != nil {
				return v5MigrationActivationPlan{}, fmt.Errorf("decode project %q blockers: %w", project.ID, decodeErr)
			}
			active.Tasks = append(active.Tasks, storage.ConvertLegacyV4Blockers(blockers)...)
			blockerSourceID, relErr := filepath.Rel(dataDir, blockersPath)
			if relErr != nil {
				return v5MigrationActivationPlan{}, relErr
			}
			for _, blocker := range blockers {
				entry := format.V5TaskMigrationAuditEntry{
					TaskID: blocker.ID,
					Field:  "standalone_blocker",
				}
				if blocker.ResolvedAt == nil {
					entry.Action = "converted"
					entry.From = "unresolved blocker"
					entry.To = "Inbox task + tag:blocker"
					entry.Reason = "v5 represents unresolved blockers as ordinary tasks"
				} else {
					entry.Action = "snapshot-only"
					entry.From = "resolved blocker"
					entry.Reason = "v5 has no standalone blocker storage"
				}
				taskAudit = append(taskAudit, v5MigrationTaskAuditRow{
					ProjectID: project.ID,
					SourceID:  filepath.ToSlash(blockerSourceID),
					Entry:     entry,
				})
			}
			blockerPaths = append(blockerPaths, blockersPath)
		}

		archives, indexPath, archiveErr := legacyV4ProjectArchives(dataDir, projectDir, location)
		if archiveErr != nil {
			return v5MigrationActivationPlan{}, fmt.Errorf("read project %q archives: %w", project.ID, archiveErr)
		}
		for _, archive := range archives {
			taskAudit = appendV5MigrationTaskAudit(
				taskAudit,
				project.ID,
				archive.SourceID,
				archive.Conversion.Audit,
			)
		}
		reintegrationInput.Projects = append(reintegrationInput.Projects, storage.V5ProjectTaskReintegrationInput{
			ProjectID:        project.ID,
			Active:           active,
			Archives:         archives,
			ArchiveIndexPath: indexPath,
		})
	}

	reintegration, err := storage.PlanV5ArchiveReintegration(reintegrationInput)
	if err != nil {
		return v5MigrationActivationPlan{}, err
	}
	if reconcileErr := reconcileV5MigrationPlanning(&domainPlan.Domains.Domains, reintegration.Projects); reconcileErr != nil {
		return v5MigrationActivationPlan{}, reconcileErr
	}

	planningBytes, err := encodeV5DomainEnvelope(
		domainPlanning.name(),
		domainPlan.Domains.Domains.Catalog.WorkspaceID,
		domainPlan.Domains.Domains.Planning,
	)
	if err != nil {
		return v5MigrationActivationPlan{}, err
	}

	builder := newV5ActivationBuilder(dataDir)
	for _, candidate := range domainPlan.Artifacts {
		data := candidate.Data
		if candidate.Kind == domainPlanning.name() {
			data = planningBytes
		}
		var addErr error
		if candidate.Kind == domainMachine.name() {
			addErr = builder.addMachineWrite(candidate.Path, data)
		} else {
			addErr = builder.addWrite(candidate.Path, data)
		}
		if addErr != nil {
			return v5MigrationActivationPlan{}, addErr
		}
	}

	projectByID := make(map[string]storage.V5ProjectTaskCandidate, len(reintegration.Projects))
	for _, project := range reintegration.Projects {
		projectByID[project.ProjectID] = project
	}
	for _, project := range domainPlan.Domains.Domains.Catalog.Projects {
		candidate, ok := projectByID[project.ID]
		if !ok {
			return v5MigrationActivationPlan{}, fmt.Errorf("v5 migration produced no task document for project %q", project.ID)
		}
		content, encodeErr := storage.WriteV5ProjectDocument(storage.V5ProjectDocument{
			ProjectName: candidate.ProjectName,
			Tasks:       candidate.Tasks,
		})
		if encodeErr != nil {
			return v5MigrationActivationPlan{}, encodeErr
		}
		if _, parseErr := storage.ParseV5ProjectDocument(content); parseErr != nil {
			return v5MigrationActivationPlan{}, fmt.Errorf("validate v5 project %q output: %w", project.ID, parseErr)
		}
		if addErr := builder.addWrite(
			storage.TasksFilePath(filepath.Join(dataDir, "projects", project.Slug)),
			[]byte(content),
		); addErr != nil {
			return v5MigrationActivationPlan{}, addErr
		}
	}

	for _, cleanup := range reintegration.Cleanup {
		if deleteErr := builder.addFileDelete(cleanup.Path); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
	}
	for _, path := range blockerPaths {
		if deleteErr := builder.addFileDelete(path); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
	}
	for _, path := range domainPlan.RetiredDomainPaths {
		if deleteErr := builder.addOptionalFileDelete(path); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
		if deleteErr := builder.addOptionalFileDelete(path + ".bak"); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
	}
	for _, dir := range domainPlan.SnapshotOnlyProjectDirs {
		if cleanupErr := builder.addCleanupTree(dir); cleanupErr != nil {
			return v5MigrationActivationPlan{}, cleanupErr
		}
	}
	if cleanupErr := builder.addCleanupTree(filepath.Join(InternalDir(dataDir), "trash")); cleanupErr != nil {
		return v5MigrationActivationPlan{}, cleanupErr
	}
	for _, dir := range []string{
		filepath.Join(InternalDir(dataDir), "themes"),
		filepath.Join(dataDir, "themes"),
	} {
		if cleanupErr := builder.addCleanupTree(dir); cleanupErr != nil {
			return v5MigrationActivationPlan{}, cleanupErr
		}
	}
	for _, project := range domainPlan.Domains.Domains.Catalog.Projects {
		if cleanupErr := builder.addCleanupTree(
			filepath.Join(dataDir, "projects", project.Slug, legacyV4ProjectUndoDirName),
		); cleanupErr != nil {
			return v5MigrationActivationPlan{}, cleanupErr
		}
		if deleteErr := builder.addOptionalFileDelete(
			filepath.Join(dataDir, "projects", project.Slug, legacyV4ProjectBackupName),
		); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
	}

	plan, err := builder.finish()
	if err != nil {
		return v5MigrationActivationPlan{}, err
	}
	plan.audit = renderV5MigrationConversionAudit(
		domainPlan.Domains.Audit,
		domainPlan.NoteAudit,
		taskAudit,
		reintegration.Report,
	)
	return plan, nil
}

func convertLegacyV4TaskFile(path string, location *time.Location) (format.V5TaskConversionResult, error) {
	data, err := readV5DryRunSource(path)
	if err != nil {
		return format.V5TaskConversionResult{}, err
	}
	document, err := format.DecodeLegacyV4TaskDocument(string(data), location)
	if err != nil {
		return format.V5TaskConversionResult{}, err
	}
	return format.ConvertLegacyV4TaskDocument(document, location)
}

func legacyV4ProjectArchives(
	dataDir string,
	projectDir string,
	location *time.Location,
) ([]storage.V5ArchiveTaskSource, string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, "", err
	}
	var archives []storage.V5ArchiveTaskSource
	var indexPath string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(projectDir, entry.Name())
		switch {
		case legacyV4ArchiveFileName.MatchString(entry.Name()):
			conversion, err := convertLegacyV4TaskFile(path, location)
			if err != nil {
				return nil, "", err
			}
			sourceID, err := filepath.Rel(dataDir, path)
			if err != nil {
				return nil, "", err
			}
			archives = append(archives, storage.V5ArchiveTaskSource{
				SourceID:   filepath.ToSlash(sourceID),
				Path:       path,
				Conversion: conversion,
			})
		case entry.Name() == legacyV4ArchiveIndexFileName:
			if _, err := readV5DryRunSource(path); err != nil {
				return nil, "", err
			}
			indexPath = path
		}
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].SourceID < archives[j].SourceID })
	return archives, indexPath, nil
}

func readOptionalV5MigrationFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect v5 migration source %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("v5 migration source is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read v5 migration source %s: %w", path, err)
	}
	return data, true, nil
}

func reconcileV5MigrationPlanning(
	domains *V5DomainCandidates,
	projects []storage.V5ProjectTaskCandidate,
) error {
	if domains == nil {
		return fmt.Errorf("v5 migration domains are required")
	}
	type taskIdentity struct {
		projectID string
		taskID    string
	}
	states := make(map[taskIdentity]string)
	var nextInDocumentOrder []V5PlanningTaskRef
	for _, project := range projects {
		for _, task := range project.Tasks {
			identity := taskIdentity{projectID: project.ProjectID, taskID: task.ID}
			states[identity] = task.State
			if task.State == "next" {
				nextInDocumentOrder = append(nextInDocumentOrder, V5PlanningTaskRef{
					ProjectID: project.ProjectID,
					TaskID:    task.ID,
				})
			}
		}
	}

	var today []V5PlanningTaskRef
	for _, ref := range domains.Planning.Today {
		state, exists := states[taskIdentity{projectID: ref.ProjectID, taskID: ref.TaskID}]
		if !exists || state == "inbox" || state == "next" || state == "done" || state == "canceled" {
			continue
		}
		today = append(today, ref)
	}
	domains.Planning.Today = today

	seenNext := make(map[taskIdentity]bool)
	var nextOrder []V5PlanningTaskRef
	for _, ref := range domains.Planning.NextOrder {
		identity := taskIdentity{projectID: ref.ProjectID, taskID: ref.TaskID}
		if states[identity] != "next" || seenNext[identity] {
			continue
		}
		seenNext[identity] = true
		nextOrder = append(nextOrder, ref)
	}
	for _, ref := range nextInDocumentOrder {
		identity := taskIdentity{projectID: ref.ProjectID, taskID: ref.TaskID}
		if seenNext[identity] {
			continue
		}
		seenNext[identity] = true
		nextOrder = append(nextOrder, ref)
	}
	domains.Planning.NextOrder = nextOrder
	return domains.Validate()
}

type v5ActivationBuilder struct {
	dataDir      string
	artifacts    []storage.ArtifactChange
	artifactKind map[string]bool
	cleanupFiles map[string]bool
	cleanupDirs  map[string]bool
}

func newV5ActivationBuilder(dataDir string) *v5ActivationBuilder {
	return &v5ActivationBuilder{
		dataDir:      dataDir,
		artifactKind: make(map[string]bool),
		cleanupFiles: make(map[string]bool),
		cleanupDirs:  make(map[string]bool),
	}
}

func (builder *v5ActivationBuilder) addWrite(path string, data []byte) error {
	return builder.addWriteCandidate(path, data, false)
}

func (builder *v5ActivationBuilder) addMachineWrite(path string, data []byte) error {
	return builder.addWriteCandidate(path, data, true)
}

func (builder *v5ActivationBuilder) addWriteCandidate(path string, data []byte, allowCreate bool) error {
	path = filepath.Clean(path)
	if builder.artifactKind[path] || builder.cleanupFiles[path] {
		return fmt.Errorf("v5 migration repeats artifact target %s", path)
	}
	if err := RefusePreSimplificationV5SnapshotMutation(builder.dataDir, path); err != nil {
		return err
	}
	source, found, err := readOptionalV5DryRunSource(path)
	if err != nil {
		return err
	}
	if !found && !allowCreate {
		return fmt.Errorf("inspect v5 conversion source %s: %w", path, fs.ErrNotExist)
	}
	expectedRevision := ""
	if found {
		expectedRevision = storage.RawArtifactRevision(source)
	}
	builder.artifactKind[path] = true
	builder.artifacts = append(builder.artifacts, storage.ArtifactChange{
		Path:             path,
		Data:             append([]byte(nil), data...),
		Mode:             FilePermissions,
		ExpectedRevision: &expectedRevision,
	})
	return nil
}

func (builder *v5ActivationBuilder) addFileDelete(path string) error {
	path = filepath.Clean(path)
	if builder.artifactKind[path] {
		return fmt.Errorf("v5 migration cannot write and delete %s", path)
	}
	if builder.cleanupFiles[path] {
		return nil
	}
	if err := RefusePreSimplificationV5SnapshotMutation(builder.dataDir, path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect v5 migration cleanup file %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("v5 migration cleanup target is not a regular file: %s", path)
	}
	builder.cleanupFiles[path] = true
	return nil
}

func (builder *v5ActivationBuilder) addOptionalFileDelete(path string) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect optional v5 migration cleanup file %s: %w", path, err)
	}
	return builder.addFileDelete(path)
}

func (builder *v5ActivationBuilder) addCleanupTree(root string) error {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect v5 migration cleanup tree %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("v5 migration refuses symlink cleanup tree: %s", root)
	}
	if !info.IsDir() {
		return builder.addFileDelete(root)
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		path = filepath.Clean(path)
		if err := RefusePreSimplificationV5SnapshotMutation(builder.dataDir, path); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("v5 migration refuses symlink cleanup target: %s", path)
		}
		if entry.IsDir() {
			builder.cleanupDirs[path] = true
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("v5 migration cleanup target is not regular: %s", path)
		}
		if builder.artifactKind[path] {
			return fmt.Errorf("v5 migration cannot write and delete %s", path)
		}
		builder.cleanupFiles[path] = true
		return nil
	})
}

func (builder *v5ActivationBuilder) finish() (v5MigrationActivationPlan, error) {
	sort.Slice(builder.artifacts, func(i, j int) bool {
		return builder.artifacts[i].Path < builder.artifacts[j].Path
	})
	files := make([]string, 0, len(builder.cleanupFiles))
	for path := range builder.cleanupFiles {
		files = append(files, path)
	}
	sort.Strings(files)
	dirs := make([]string, 0, len(builder.cleanupDirs))
	for path := range builder.cleanupDirs {
		dirs = append(dirs, path)
	}
	sort.Slice(dirs, func(i, j int) bool {
		leftDepth := strings.Count(filepath.Clean(dirs[i]), string(os.PathSeparator))
		rightDepth := strings.Count(filepath.Clean(dirs[j]), string(os.PathSeparator))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return dirs[i] < dirs[j]
	})

	ops := make([]transaction.Op, 0, len(files)+len(dirs))
	for _, path := range files {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
	}
	for _, path := range dirs {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
	}
	if len(builder.artifacts) == 0 {
		return v5MigrationActivationPlan{}, fmt.Errorf("v5 migration produced no canonical artifacts")
	}
	return v5MigrationActivationPlan{artifacts: builder.artifacts, cleanupOps: ops}, nil
}

func appendV5MigrationTaskAudit(
	target []v5MigrationTaskAuditRow,
	projectID string,
	sourceID string,
	entries []format.V5TaskMigrationAuditEntry,
) []v5MigrationTaskAuditRow {
	for _, entry := range entries {
		target = append(target, v5MigrationTaskAuditRow{
			ProjectID: projectID,
			SourceID:  sourceID,
			Entry:     entry,
		})
	}
	return target
}

func renderV5MigrationConversionAudit(
	domainAudit []V5DomainMigrationAuditEntry,
	noteAudit []storage.V5NoteMigrationAuditEntry,
	taskAudit []v5MigrationTaskAuditRow,
	reintegration storage.V5TaskReintegrationReport,
) []byte {
	var out strings.Builder
	out.WriteString("\n## Conversion and repair report\n")

	out.WriteString("\n### Configuration\n\n")
	if len(domainAudit) == 0 {
		out.WriteString("No configuration conversions or repairs were required.\n")
	} else {
		out.WriteString("| Domain | Entity | Action | Field | From | To | Reason |\n")
		out.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, entry := range domainAudit {
			fmt.Fprintf(
				&out,
				"| %s | %s | %s | %s | %s | %s | %s |\n",
				escapePreSimplificationV5AuditCell(entry.Domain),
				escapePreSimplificationV5AuditCell(entry.EntityID),
				escapePreSimplificationV5AuditCell(entry.Action),
				escapePreSimplificationV5AuditCell(entry.Field),
				escapePreSimplificationV5AuditCell(entry.From),
				escapePreSimplificationV5AuditCell(entry.To),
				escapePreSimplificationV5AuditCell(entry.Reason),
			)
		}
	}

	out.WriteString("\n### Notes\n\n")
	if len(noteAudit) == 0 {
		out.WriteString("No note fields were removed.\n")
	} else {
		out.WriteString("| Note | Action | Field | From | Reason |\n")
		out.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, entry := range noteAudit {
			fmt.Fprintf(
				&out,
				"| %s | %s | %s | %s | %s |\n",
				escapePreSimplificationV5AuditCell(entry.Filename),
				escapePreSimplificationV5AuditCell(entry.Action),
				escapePreSimplificationV5AuditCell(entry.Field),
				escapePreSimplificationV5AuditCell(entry.From),
				escapePreSimplificationV5AuditCell(entry.Reason),
			)
		}
	}

	out.WriteString("\n### Task fields and standalone blockers\n\n")
	if len(taskAudit) == 0 {
		out.WriteString("No task-field conversions were required.\n")
	} else {
		out.WriteString("| Project | Source | Task | Action | Field | From | To | Reason |\n")
		out.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
		for _, row := range taskAudit {
			fmt.Fprintf(
				&out,
				"| %s | %s | %s | %s | %s | %s | %s | %s |\n",
				escapePreSimplificationV5AuditCell(row.ProjectID),
				escapePreSimplificationV5AuditCell(row.SourceID),
				escapePreSimplificationV5AuditCell(row.Entry.TaskID),
				escapePreSimplificationV5AuditCell(row.Entry.Action),
				escapePreSimplificationV5AuditCell(row.Entry.Field),
				escapePreSimplificationV5AuditCell(row.Entry.From),
				escapePreSimplificationV5AuditCell(row.Entry.To),
				escapePreSimplificationV5AuditCell(row.Entry.Reason),
			)
		}
	}

	out.WriteString("\n### Archive reintegration\n\n")
	fmt.Fprintf(&out, "- Input tasks: %d\n", reintegration.InputTasks)
	fmt.Fprintf(&out, "- Retained tasks: %d\n", reintegration.RetainedTasks)
	fmt.Fprintf(&out, "- Omitted duplicate tasks: %d\n", reintegration.OmittedTasks)
	if len(reintegration.Entries) == 0 {
		out.WriteString("\nNo identity or reference repairs were required.\n")
		return []byte(out.String())
	}
	out.WriteString("\n| Project | Task | Action | Field | Source | From | To | Reason |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range reintegration.Entries {
		fmt.Fprintf(
			&out,
			"| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			escapePreSimplificationV5AuditCell(entry.ProjectID),
			escapePreSimplificationV5AuditCell(entry.TaskID),
			escapePreSimplificationV5AuditCell(entry.Action),
			escapePreSimplificationV5AuditCell(entry.Field),
			escapePreSimplificationV5AuditCell(entry.Source),
			escapePreSimplificationV5AuditCell(entry.From),
			escapePreSimplificationV5AuditCell(entry.To),
			escapePreSimplificationV5AuditCell(entry.Reason),
		)
	}
	return []byte(out.String())
}
