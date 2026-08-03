package config

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

const (
	preSimplificationV5SnapshotPrefix = "pre-simplification-v5-"
	preSimplificationV5Timestamp      = "20060102T150405Z"
	preSimplificationV5ArchiveSuffix  = ".tar.gz"
	preSimplificationV5AuditSuffix    = ".audit.md"
	preSimplificationV5AuditEntry     = "pre-simplification-v5-audit.md"

	// Snapshot artifacts are application-owned rollback sources. Keeping them
	// owner-readable but not writable makes accidental ordinary-file replacement
	// fail even outside the higher-level immutable-path guard below.
	preSimplificationV5Permissions fs.FileMode = 0o400
)

// ErrImmutablePreSimplificationV5Snapshot classifies an application request to
// overwrite or delete a published pre-simplification snapshot artifact.
var ErrImmutablePreSimplificationV5Snapshot = errors.New("config: pre-simplification v5 snapshots are immutable")

// PreSimplificationV5SnapshotFile is one byte-exact source recorded by a
// pre-simplification snapshot. Path is the portable path used in the archive
// and audit report rather than an absolute machine path.
type PreSimplificationV5SnapshotFile struct {
	Path   string
	Kind   string
	Size   int64
	SHA256 string
}

// PreSimplificationV5Snapshot describes one atomically published rollback
// snapshot and its attachment inventory.
type PreSimplificationV5Snapshot struct {
	CreatedAt     time.Time
	ArchivePath   string
	AuditPath     string
	ArchiveSHA256 string
	ArchivedFiles []PreSimplificationV5SnapshotFile
	Attachments   []PreSimplificationV5SnapshotFile
}

// PreSimplificationV5SnapshotPair is one canonical rollback archive and its
// matching human-readable migration audit.
type PreSimplificationV5SnapshotPair struct {
	CreatedAt   time.Time
	ArchivePath string
	AuditPath   string
}

// PreSimplificationV5SnapshotInspection is the read-only result used by
// diagnostics. HasArtifacts becomes true for any direct backup-directory entry
// that looks application-owned by prefix, including malformed or orphaned
// names. Pairs contains only matching canonical archive/audit names.
type PreSimplificationV5SnapshotInspection struct {
	HasArtifacts bool
	Pairs        []PreSimplificationV5SnapshotPair
	Issues       []string
}

type preSimplificationV5SnapshotCallbacks interface {
	snapshotCreatedAt() time.Time
	didInventorySnapshot()
	willPublishSnapshot(archivePath, auditPath string)
}

type preSimplificationV5Source struct {
	PreSimplificationV5SnapshotFile
	sourcePath string
	data       []byte
}

type preSimplificationV5Inventory struct {
	archived    []preSimplificationV5Source
	attachments []preSimplificationV5Source
}

// PreSimplificationV5BackupsDir returns the application-owned directory that
// holds immutable v5 rollback snapshots.
func PreSimplificationV5BackupsDir(dataDir string) string {
	return filepath.Join(InternalDir(filepath.Clean(dataDir)), "backups")
}

// IsPreSimplificationV5SnapshotPath reports whether path names one direct,
// timestamped archive or audit artifact in the workspace's protected backup
// directory. It recognizes only the canonical naming grammar.
func IsPreSimplificationV5SnapshotPath(dataDir, path string) bool {
	backupsDir := filepath.Clean(PreSimplificationV5BackupsDir(dataDir))
	candidate := filepath.Clean(path)
	if filepath.Dir(candidate) != backupsDir {
		return false
	}
	name := filepath.Base(candidate)
	if !strings.HasPrefix(name, preSimplificationV5SnapshotPrefix) {
		return false
	}
	stampAndSuffix := strings.TrimPrefix(name, preSimplificationV5SnapshotPrefix)
	stamp := ""
	switch {
	case strings.HasSuffix(stampAndSuffix, preSimplificationV5ArchiveSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preSimplificationV5ArchiveSuffix)
	case strings.HasSuffix(stampAndSuffix, preSimplificationV5AuditSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preSimplificationV5AuditSuffix)
	default:
		return false
	}
	_, err := time.Parse(preSimplificationV5Timestamp, stamp)
	return err == nil
}

// InspectPreSimplificationV5Snapshots validates the names, file types, sizes,
// and pairing of application-owned rollback artifacts without creating,
// changing, or deleting anything. Ordinary files in the backups directory are
// ignored; a pre-simplification-v5-* entry is always treated as migration
// evidence and reported when it is malformed or incomplete.
func InspectPreSimplificationV5Snapshots(dataDir string) (PreSimplificationV5SnapshotInspection, error) {
	var result PreSimplificationV5SnapshotInspection
	backupsDir := PreSimplificationV5BackupsDir(dataDir)
	dirInfo, err := os.Lstat(backupsDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("inspect pre-simplification v5 backups directory: %w", err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return result, fmt.Errorf("pre-simplification v5 backups path is not a regular directory: %s", backupsDir)
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return result, fmt.Errorf("read pre-simplification v5 backups directory: %w", err)
	}

	type pairState struct {
		createdAt   time.Time
		archivePath string
		auditPath   string
	}
	states := make(map[string]*pairState)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, preSimplificationV5SnapshotPrefix) {
			continue
		}
		// Artifact commits serialize publication with an adjacent advisory lock.
		// A lock is coordination state, not rollback evidence, but only ignore it
		// when its target is itself a canonical snapshot artifact.
		if strings.HasSuffix(name, ".lock") {
			target := strings.TrimSuffix(filepath.Join(backupsDir, name), ".lock")
			if IsPreSimplificationV5SnapshotPath(dataDir, target) {
				continue
			}
		}
		result.HasArtifacts = true
		path := filepath.Join(backupsDir, name)
		if !IsPreSimplificationV5SnapshotPath(dataDir, path) {
			result.Issues = append(result.Issues, fmt.Sprintf("malformed pre-simplification snapshot artifact %q", name))
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return result, fmt.Errorf("inspect pre-simplification snapshot artifact %q: %w", name, infoErr)
		}
		if !info.Mode().IsRegular() {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-simplification snapshot artifact %q is not a regular file", name))
		} else if info.Size() == 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-simplification snapshot artifact %q is empty", name))
		}

		stem := ""
		isArchive := strings.HasSuffix(name, preSimplificationV5ArchiveSuffix)
		if isArchive {
			stem = strings.TrimSuffix(name, preSimplificationV5ArchiveSuffix)
		} else {
			stem = strings.TrimSuffix(name, preSimplificationV5AuditSuffix)
		}
		state := states[stem]
		if state == nil {
			stamp := strings.TrimPrefix(stem, preSimplificationV5SnapshotPrefix)
			createdAt, parseErr := time.Parse(preSimplificationV5Timestamp, stamp)
			if parseErr != nil {
				// IsPreSimplificationV5SnapshotPath already checked this, so
				// reaching this branch would indicate an internal grammar drift.
				return result, fmt.Errorf("parse canonical pre-simplification snapshot timestamp %q: %w", stamp, parseErr)
			}
			state = &pairState{createdAt: createdAt}
			states[stem] = state
		}
		if isArchive {
			state.archivePath = path
		} else {
			state.auditPath = path
		}
	}

	stems := make([]string, 0, len(states))
	for stem := range states {
		stems = append(stems, stem)
	}
	sort.Strings(stems)
	for _, stem := range stems {
		state := states[stem]
		switch {
		case state.archivePath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("pre-simplification snapshot %q is missing its archive", stem))
		case state.auditPath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("pre-simplification snapshot %q is missing its audit report", stem))
		default:
			result.Pairs = append(result.Pairs, PreSimplificationV5SnapshotPair{
				CreatedAt:   state.createdAt,
				ArchivePath: state.archivePath,
				AuditPath:   state.auditPath,
			})
		}
	}
	sort.Strings(result.Issues)
	return result, nil
}

// RefusePreSimplificationV5SnapshotMutation is the shared application guard for
// any future cleanup or migration code that accepts a filesystem target.
func RefusePreSimplificationV5SnapshotMutation(dataDir, path string) error {
	if !IsPreSimplificationV5SnapshotPath(dataDir, path) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrImmutablePreSimplificationV5Snapshot, filepath.Clean(path))
}

func createPreSimplificationV5SnapshotWithMigrationAudit(
	dataDir string,
	migrationAudit []byte,
) (*PreSimplificationV5Snapshot, error) {
	return createPreSimplificationV5SnapshotWithAudit(dataDir, nil, migrationAudit)
}

func createPreSimplificationV5SnapshotWithAudit(
	dataDir string,
	callbacks preSimplificationV5SnapshotCallbacks,
	migrationAudit []byte,
) (*PreSimplificationV5Snapshot, error) {
	root, err := validatePreSimplificationV5Root(dataDir)
	if err != nil {
		return nil, err
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	if callbacks != nil {
		if override := callbacks.snapshotCreatedAt(); !override.IsZero() {
			createdAt = override.UTC().Truncate(time.Second)
		}
	}
	stem := preSimplificationV5SnapshotPrefix + createdAt.Format(preSimplificationV5Timestamp)
	archivePath := filepath.Join(PreSimplificationV5BackupsDir(root), stem+preSimplificationV5ArchiveSuffix)
	auditPath := filepath.Join(PreSimplificationV5BackupsDir(root), stem+preSimplificationV5AuditSuffix)

	if precheckErr := precheckPreSimplificationV5Targets(archivePath, auditPath); precheckErr != nil {
		return nil, precheckErr
	}
	// Validate the complete source shape before taking the workspace lock. Common
	// failures such as an unsafe symlink therefore leave neither a backup
	// artifact nor a transaction marker behind. The locked planner inventories
	// again and remains authoritative.
	if _, inventoryErr := inventoryPreSimplificationV5(root, false); inventoryErr != nil {
		return nil, inventoryErr
	}

	var result *PreSimplificationV5Snapshot
	commit := storage.NewCommitService(TransactionsDir(root), []string{root})
	_, err = commit.CommitPlannedArtifacts(func() ([]storage.ArtifactChange, error) {
		if precheckErr := precheckPreSimplificationV5Targets(archivePath, auditPath); precheckErr != nil {
			return nil, precheckErr
		}
		inventory, inventoryErr := inventoryPreSimplificationV5(root, true)
		if inventoryErr != nil {
			return nil, inventoryErr
		}
		if callbacks != nil {
			callbacks.didInventorySnapshot()
		}
		current, inventoryErr := inventoryPreSimplificationV5(root, false)
		if inventoryErr != nil {
			return nil, inventoryErr
		}
		if compareErr := comparePreSimplificationV5Inventories(inventory, current); compareErr != nil {
			return nil, compareErr
		}

		audit := renderPreSimplificationV5Audit(createdAt, inventory)
		audit = append(audit, migrationAudit...)
		archive, archiveErr := buildPreSimplificationV5Archive(inventory.archived, audit)
		if archiveErr != nil {
			return nil, archiveErr
		}
		archiveSum := sha256.Sum256(archive)
		absent := ""
		result = &PreSimplificationV5Snapshot{
			CreatedAt:     createdAt,
			ArchivePath:   archivePath,
			AuditPath:     auditPath,
			ArchiveSHA256: hex.EncodeToString(archiveSum[:]),
			ArchivedFiles: publicPreSimplificationV5Files(inventory.archived),
			Attachments:   publicPreSimplificationV5Files(inventory.attachments),
		}
		if callbacks != nil {
			callbacks.willPublishSnapshot(archivePath, auditPath)
		}
		return []storage.ArtifactChange{
			{
				Path:             archivePath,
				Data:             archive,
				Mode:             preSimplificationV5Permissions,
				ExpectedRevision: &absent,
			},
			{
				Path:             auditPath,
				Data:             audit,
				Mode:             preSimplificationV5Permissions,
				ExpectedRevision: &absent,
			},
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("pre-simplification v5 snapshot failed: %w", err)
	}
	return result, nil
}

func validatePreSimplificationV5Root(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", fmt.Errorf("pre-simplification v5 snapshot requires a workspace data dir")
	}
	root, err := filepath.Abs(filepath.Clean(dataDir))
	if err != nil {
		return "", fmt.Errorf("resolve workspace data dir: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("inspect workspace data dir %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("pre-simplification v5 snapshot refuses a symlink workspace root: %s", root)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("pre-simplification v5 snapshot workspace root is not a directory: %s", root)
	}
	return root, nil
}

func precheckPreSimplificationV5Targets(paths ...string) error {
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("inspect snapshot destination %s: %w", path, err)
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("pre-simplification v5 snapshot destination is a symlink, refusing to replace: %s", path)
		default:
			return fmt.Errorf("pre-simplification v5 snapshot destination already exists, refusing to overwrite: %s", path)
		}
	}
	return nil
}

func inventoryPreSimplificationV5(root string, captureData bool) (*preSimplificationV5Inventory, error) {
	inventory := &preSimplificationV5Inventory{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if validateErr := validatePreSimplificationV5RelativePath(rel); validateErr != nil {
			return fmt.Errorf("unsafe workspace path %s: %w", path, validateErr)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pre-simplification v5 snapshot refuses symlink source: %s", path)
		}
		if isPreSimplificationV5Excluded(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("inspect snapshot source %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pre-simplification v5 snapshot refuses non-regular source: %s", path)
		}
		archivePath := filepath.ToSlash(filepath.Join("workspace", rel))
		source, err := readPreSimplificationV5Source(path, archivePath, preSimplificationV5Kind(rel), captureData)
		if err != nil {
			return err
		}
		if isPreSimplificationV5Attachment(rel) {
			inventory.attachments = append(inventory.attachments, source)
		} else {
			inventory.archived = append(inventory.archived, source)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory workspace for pre-simplification v5 snapshot: %w", err)
	}

	machineSources, err := inventoryPreSimplificationV5MachineState(root, captureData)
	if err != nil {
		return nil, err
	}
	inventory.archived = append(inventory.archived, machineSources...)
	sortPreSimplificationV5Sources(inventory.archived)
	sortPreSimplificationV5Sources(inventory.attachments)
	if err := rejectDuplicatePreSimplificationV5Paths(inventory); err != nil {
		return nil, err
	}
	return inventory, nil
}

func isPreSimplificationV5Excluded(rel string) bool {
	clean := filepath.Clean(rel)
	if isMigrationRootExcluded(clean) {
		return true
	}
	switch clean {
	case filepath.Join(dataDirName, "backups"),
		filepath.Join(dataDirName, "transactions"):
		return true
	}
	// Advisory locks and interrupted atomic-write temporaries are coordination
	// state, not rollback data. Excluding them also keeps repeated snapshots
	// stable after ordinary tt operations.
	base := filepath.Base(clean)
	return strings.HasSuffix(base, ".lock") ||
		strings.Contains(base, ".tmp.") ||
		strings.HasPrefix(base, ".tt-move-")
}

func isPreSimplificationV5Attachment(rel string) bool {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(rel)), "/")
	if len(parts) < 4 || parts[0] != projectsDirName {
		return false
	}
	for _, part := range parts[2:] {
		if part == ".attachments" {
			return true
		}
	}
	return false
}

func preSimplificationV5Kind(rel string) string {
	clean := filepath.ToSlash(filepath.Clean(rel))
	base := filepath.Base(clean)
	switch {
	case clean == configFileName,
		clean == ".tt/workspace.json",
		clean == ".tt/planning.json",
		clean == ".tt/settings.json",
		clean == ".tt/automation.json",
		strings.HasPrefix(clean, ".tt/themes/"),
		strings.HasPrefix(clean, "themes/"):
		return "portable configuration"
	case strings.HasPrefix(clean, ".tt/global-notes/"),
		strings.Contains(clean, "/notes/"):
		return "note"
	case strings.HasPrefix(clean, ".tt/trash/"):
		return "project trash"
	case strings.HasPrefix(base, "completed") && strings.HasSuffix(base, ".md"):
		return "archived task Markdown"
	case base == "tasks.md":
		return "active task Markdown"
	case isPreSimplificationV5Attachment(rel):
		return "attachment retained in place"
	case base == "blockers.md" || base == ".completed.index.json":
		return "project metadata"
	default:
		return "workspace data"
	}
}

func readPreSimplificationV5Source(path, archivePath, kind string, captureData bool) (preSimplificationV5Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return preSimplificationV5Source{}, fmt.Errorf("read snapshot source %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	source := preSimplificationV5Source{
		PreSimplificationV5SnapshotFile: PreSimplificationV5SnapshotFile{
			Path:   archivePath,
			Kind:   kind,
			Size:   int64(len(data)),
			SHA256: hex.EncodeToString(sum[:]),
		},
		sourcePath: path,
	}
	if captureData {
		source.data = data
	}
	return source, nil
}

func inventoryPreSimplificationV5MachineState(dataDir string, captureData bool) ([]preSimplificationV5Source, error) {
	workspaceID, err := preSimplificationV5WorkspaceID(dataDir)
	if err != nil {
		return nil, err
	}
	candidates := legacyMachineStatePaths(dataDir)
	if workspaceID != "" {
		machinePath, pathErr := MachineStatePath(workspaceID)
		if pathErr != nil {
			return nil, pathErr
		}
		candidates = append(candidates, machinePath)
	}

	localRoot, err := filepath.Abs(LocalDataRoot())
	if err != nil {
		return nil, fmt.Errorf("resolve machine-state root: %w", err)
	}
	seen := make(map[string]bool)
	var sources []preSimplificationV5Source
	for _, candidate := range candidates {
		for _, path := range []string{candidate, candidate + ".bak"} {
			path, err = filepath.Abs(filepath.Clean(path))
			if err != nil {
				return nil, fmt.Errorf("resolve machine-state source %s: %w", path, err)
			}
			if seen[path] {
				continue
			}
			seen[path] = true
			info, statErr := os.Lstat(path)
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			if statErr != nil {
				return nil, fmt.Errorf("inspect machine-state source %s: %w", path, statErr)
			}
			if err := validatePreSimplificationV5ContainedFile(localRoot, path, info); err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(localRoot, path)
			if err != nil {
				return nil, fmt.Errorf("relativize machine-state source %s: %w", path, err)
			}
			if validateErr := validatePreSimplificationV5RelativePath(rel); validateErr != nil {
				return nil, fmt.Errorf("unsafe machine-state path %s: %w", path, validateErr)
			}
			source, err := readPreSimplificationV5Source(
				path,
				filepath.ToSlash(filepath.Join("machine", rel)),
				"machine state",
				captureData,
			)
			if err != nil {
				return nil, err
			}
			sources = append(sources, source)
		}
	}
	return sources, nil
}

func preSimplificationV5WorkspaceID(dataDir string) (string, error) {
	path := WorkspaceCatalogPath(dataDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read workspace identity for snapshot: %w", err)
	}
	var envelope domainEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("decode workspace identity for snapshot: %w", err)
	}
	if envelope.Kind != domainCatalog.name() {
		return "", fmt.Errorf("workspace identity artifact kind is %q, want %q", envelope.Kind, domainCatalog.name())
	}
	if err := validateWorkspaceID(envelope.WorkspaceID); err != nil {
		return "", fmt.Errorf("workspace identity for snapshot: %w", err)
	}
	return envelope.WorkspaceID, nil
}

func validatePreSimplificationV5ContainedFile(root, path string, info fs.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("pre-simplification v5 snapshot refuses symlink source: %s", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("pre-simplification v5 snapshot refuses non-regular source: %s", path)
	}
	if !util.PathWithinRoot(root, path) {
		return fmt.Errorf("machine-state source escapes the local runtime root: %s", path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(filepath.Clean(rel), string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		partInfo, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect machine-state path component %s: %w", current, err)
		}
		if partInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("pre-simplification v5 snapshot refuses symlink source: %s", current)
		}
	}
	return nil
}

func validatePreSimplificationV5RelativePath(rel string) error {
	clean := filepath.Clean(rel)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path is not a safe relative child: %q", rel)
	}
	if strings.ContainsAny(clean, "\x00\r\n") {
		return fmt.Errorf("path contains a control character: %q", rel)
	}
	return nil
}

func sortPreSimplificationV5Sources(sources []preSimplificationV5Source) {
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Path != sources[j].Path {
			return sources[i].Path < sources[j].Path
		}
		return sources[i].sourcePath < sources[j].sourcePath
	})
}

func rejectDuplicatePreSimplificationV5Paths(inventory *preSimplificationV5Inventory) error {
	seen := make(map[string]string, len(inventory.archived)+len(inventory.attachments))
	for _, sources := range [][]preSimplificationV5Source{inventory.archived, inventory.attachments} {
		for _, source := range sources {
			if previous, ok := seen[source.Path]; ok && previous != source.sourcePath {
				return fmt.Errorf("snapshot archive path collision %q from %s and %s", source.Path, previous, source.sourcePath)
			}
			seen[source.Path] = source.sourcePath
		}
	}
	return nil
}

func comparePreSimplificationV5Inventories(expected, current *preSimplificationV5Inventory) error {
	if err := comparePreSimplificationV5SourceSet("archived", expected.archived, current.archived); err != nil {
		return err
	}
	return comparePreSimplificationV5SourceSet("attachment", expected.attachments, current.attachments)
}

func comparePreSimplificationV5SourceSet(kind string, expected, current []preSimplificationV5Source) error {
	if len(expected) != len(current) {
		return fmt.Errorf("pre-simplification v5 snapshot aborted: %s inventory changed while snapshotting", kind)
	}
	for i := range expected {
		if expected[i].Path != current[i].Path ||
			expected[i].sourcePath != current[i].sourcePath ||
			expected[i].Size != current[i].Size ||
			expected[i].SHA256 != current[i].SHA256 {
			return fmt.Errorf("pre-simplification v5 snapshot aborted: source changed while snapshotting: %s", expected[i].sourcePath)
		}
	}
	return nil
}

func renderPreSimplificationV5Audit(createdAt time.Time, inventory *preSimplificationV5Inventory) []byte {
	var archivedBytes, attachmentBytes int64
	for _, source := range inventory.archived {
		archivedBytes += source.Size
	}
	for _, source := range inventory.attachments {
		attachmentBytes += source.Size
	}

	var out strings.Builder
	out.WriteString("# Pre-simplification v5 migration audit\n\n")
	fmt.Fprintf(&out, "- Created at: `%s`\n", createdAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&out, "- Archived files: %d\n", len(inventory.archived))
	fmt.Fprintf(&out, "- Archived source bytes: %d\n", archivedBytes)
	fmt.Fprintf(&out, "- Attachment binaries retained in place: %d\n", len(inventory.attachments))
	fmt.Fprintf(&out, "- Retained attachment bytes: %d\n", attachmentBytes)
	out.WriteString("\nThe archive is a byte-exact rollback source. Attachment binaries were checksummed but remain in their original workspace locations.\n")

	out.WriteString("\n## Archived files\n\n")
	out.WriteString("| Archive path | Kind | Bytes | SHA-256 |\n")
	out.WriteString("| --- | --- | ---: | --- |\n")
	for _, source := range inventory.archived {
		fmt.Fprintf(
			&out,
			"| `%s` | %s | %d | `%s` |\n",
			escapePreSimplificationV5AuditCell(source.Path),
			escapePreSimplificationV5AuditCell(source.Kind),
			source.Size,
			source.SHA256,
		)
	}

	out.WriteString("\n## Attachment binaries retained in place\n\n")
	if len(inventory.attachments) == 0 {
		out.WriteString("No attachment binaries were found.\n")
		return []byte(out.String())
	}
	out.WriteString("| Workspace path | Bytes | SHA-256 |\n")
	out.WriteString("| --- | ---: | --- |\n")
	for _, source := range inventory.attachments {
		fmt.Fprintf(
			&out,
			"| `%s` | %d | `%s` |\n",
			escapePreSimplificationV5AuditCell(source.Path),
			source.Size,
			source.SHA256,
		)
	}
	return []byte(out.String())
}

func escapePreSimplificationV5AuditCell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func buildPreSimplificationV5Archive(sources []preSimplificationV5Source, audit []byte) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create snapshot compressor: %w", err)
	}
	gzipWriter.ModTime = time.Time{}
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)

	writeEntry := func(name string, data []byte) error {
		header := &tar.Header{
			Name:       name,
			Mode:       int64(FilePermissions),
			Size:       int64(len(data)),
			ModTime:    time.Unix(0, 0).UTC(),
			AccessTime: time.Time{},
			ChangeTime: time.Time{},
			Typeflag:   tar.TypeReg,
			Format:     tar.FormatPAX,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		_, err := tarWriter.Write(data)
		return err
	}

	for _, source := range sources {
		if err := writeEntry(source.Path, source.data); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("write snapshot archive entry %s: %w", source.Path, err)
		}
	}
	if err := writeEntry(preSimplificationV5AuditEntry, audit); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("write snapshot audit entry: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("finalize snapshot tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("finalize snapshot gzip: %w", err)
	}
	return buffer.Bytes(), nil
}

func publicPreSimplificationV5Files(sources []preSimplificationV5Source) []PreSimplificationV5SnapshotFile {
	files := make([]PreSimplificationV5SnapshotFile, len(sources))
	for i := range sources {
		files[i] = sources[i].PreSimplificationV5SnapshotFile
	}
	return files
}
