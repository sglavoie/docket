package config

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const (
	preScopeV7SnapshotPrefix = "pre-scope-v7-"
	preScopeV7Timestamp      = "20060102T150405Z"
	preScopeV7ArchiveSuffix  = ".tar.gz"
	preScopeV7AuditSuffix    = ".audit.md"
	preScopeV7AuditEntry     = "pre-scope-v7-audit.md"

	preScopeV7Permissions fs.FileMode = 0o400
)

// ErrImmutablePreScopeV7Snapshot classifies attempts to mutate a canonical
// pre-v7 rollback artifact.
var ErrImmutablePreScopeV7Snapshot = errors.New("config: pre-scope v7 snapshots are immutable")

// PreScopeV7SnapshotFile is one byte-exact source in the pre-v7 archive. Path
// is portable and source-relative rather than machine-absolute.
type PreScopeV7SnapshotFile struct {
	Path   string
	Kind   string
	Size   int64
	SHA256 string
}

// PreScopeV7Snapshot describes one atomically published rollback archive and
// its matching migration audit.
type PreScopeV7Snapshot struct {
	CreatedAt     time.Time
	ArchivePath   string
	AuditPath     string
	ArchiveSHA256 string
	Files         []PreScopeV7SnapshotFile
}

// PreScopeV7SnapshotPair is one canonical rollback archive and its matching
// human-readable migration audit.
type PreScopeV7SnapshotPair struct {
	CreatedAt   time.Time
	ArchivePath string
	AuditPath   string
}

// PreScopeV7SnapshotInspection is the read-only diagnostics view of the
// protected backup directory. Pairs contains only complete canonical
// archive/audit pairs; Issues describes malformed, empty, or orphaned
// application-owned artifacts.
type PreScopeV7SnapshotInspection struct {
	HasArtifacts bool
	Pairs        []PreScopeV7SnapshotPair
	Issues       []string
}

// PreScopeV7SnapshotOptions identifies the schema-6 workspace being converted.
// Machine-local state carries only the theme and is untouched by schema 7.
type PreScopeV7SnapshotOptions struct {
	DataDir string
}

type preScopeV7SnapshotCallbacks interface {
	snapshotCreatedAt() time.Time
	didInventorySnapshot()
	willPublishSnapshot(archivePath, auditPath string) error
}

type preScopeV7Source struct {
	PreScopeV7SnapshotFile
	sourcePath string
	data       []byte
}

type preScopeV7Inventory struct {
	files []preScopeV7Source
}

// PreScopeV7BackupsDir returns the application-owned directory for immutable
// pre-v7 rollback artifacts.
func PreScopeV7BackupsDir(dataDir string) string {
	return filepath.Join(InternalDir(filepath.Clean(dataDir)), "backups")
}

// IsPreScopeV7SnapshotPath reports whether path is one canonical timestamped
// pre-v7 archive or audit sidecar.
func IsPreScopeV7SnapshotPath(dataDir, path string) bool {
	backupsDir := filepath.Clean(PreScopeV7BackupsDir(dataDir))
	candidate := filepath.Clean(path)
	if filepath.Dir(candidate) != backupsDir {
		return false
	}
	name := filepath.Base(candidate)
	if !strings.HasPrefix(name, preScopeV7SnapshotPrefix) {
		return false
	}
	stampAndSuffix := strings.TrimPrefix(name, preScopeV7SnapshotPrefix)
	var stamp string
	switch {
	case strings.HasSuffix(stampAndSuffix, preScopeV7ArchiveSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preScopeV7ArchiveSuffix)
	case strings.HasSuffix(stampAndSuffix, preScopeV7AuditSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preScopeV7AuditSuffix)
	default:
		return false
	}
	_, err := time.Parse(preScopeV7Timestamp, stamp)
	return err == nil
}

// InspectPreScopeV7Snapshots validates immutable pre-v7 rollback artifacts
// without changing them. Unrelated files in the shared backups directory are
// ignored.
func InspectPreScopeV7Snapshots(dataDir string) (PreScopeV7SnapshotInspection, error) {
	var result PreScopeV7SnapshotInspection
	backupsDir := PreScopeV7BackupsDir(dataDir)
	dirInfo, err := os.Lstat(backupsDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("inspect pre-scope v7 backups directory: %w", err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return result, fmt.Errorf("pre-scope v7 backups path is not a regular directory: %s", backupsDir)
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return result, fmt.Errorf("read pre-scope v7 backups directory: %w", err)
	}

	type pairState struct {
		createdAt   time.Time
		archivePath string
		auditPath   string
	}
	states := make(map[string]*pairState)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, preScopeV7SnapshotPrefix) {
			continue
		}
		if strings.HasSuffix(name, ".lock") {
			target := strings.TrimSuffix(filepath.Join(backupsDir, name), ".lock")
			if IsPreScopeV7SnapshotPath(dataDir, target) {
				continue
			}
		}
		result.HasArtifacts = true
		path := filepath.Join(backupsDir, name)
		if !IsPreScopeV7SnapshotPath(dataDir, path) {
			result.Issues = append(result.Issues, fmt.Sprintf("malformed pre-scope v7 snapshot artifact %q", name))
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return result, fmt.Errorf("inspect pre-scope v7 snapshot artifact %q: %w", name, infoErr)
		}
		if !info.Mode().IsRegular() {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-scope v7 snapshot artifact %q is not a regular file", name))
		} else if info.Size() == 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-scope v7 snapshot artifact %q is empty", name))
		}

		isArchive := strings.HasSuffix(name, preScopeV7ArchiveSuffix)
		stem := strings.TrimSuffix(name, preScopeV7AuditSuffix)
		if isArchive {
			stem = strings.TrimSuffix(name, preScopeV7ArchiveSuffix)
		}
		state := states[stem]
		if state == nil {
			stamp := strings.TrimPrefix(stem, preScopeV7SnapshotPrefix)
			createdAt, parseErr := time.Parse(preScopeV7Timestamp, stamp)
			if parseErr != nil {
				return result, fmt.Errorf("parse canonical pre-scope v7 snapshot timestamp %q: %w", stamp, parseErr)
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
			result.Issues = append(result.Issues, fmt.Sprintf("pre-scope v7 snapshot %q is missing its archive", stem))
		case state.auditPath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("pre-scope v7 snapshot %q is missing its audit report", stem))
		default:
			result.Pairs = append(result.Pairs, PreScopeV7SnapshotPair{
				CreatedAt: state.createdAt, ArchivePath: state.archivePath, AuditPath: state.auditPath,
			})
		}
	}
	sort.Strings(result.Issues)
	return result, nil
}

// RefusePreScopeV7SnapshotMutation is the application-level immutable path
// guard for future migration and cleanup code.
func RefusePreScopeV7SnapshotMutation(dataDir, path string) error {
	if !IsPreScopeV7SnapshotPath(dataDir, path) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrImmutablePreScopeV7Snapshot, filepath.Clean(path))
}

//nolint:govet // Scoped errors keep each snapshot validation and transaction stage local.
func createPreScopeV7Snapshot(
	options PreScopeV7SnapshotOptions,
	callbacks preScopeV7SnapshotCallbacks,
) (*PreScopeV7Snapshot, error) {
	root, err := validateV5DryRunRoot(options.DataDir)
	if err != nil {
		return nil, err
	}
	dryRunOptions := V7WorkspaceDryRunOptions{DataDir: root}

	// This validation happens before the commit service can create a lock or
	// transaction artifact. Invalid candidates therefore leave no snapshot state.
	validatedPlan, err := DryRunV7WorkspaceConversion(dryRunOptions)
	if err != nil {
		return nil, fmt.Errorf("pre-scope v7 candidate validation failed; workspace was not changed: %w", err)
	}
	if err := ValidateV7WorkspaceConversionPlan(validatedPlan); err != nil {
		return nil, fmt.Errorf("pre-scope v7 candidate validation failed; workspace was not changed: %w", err)
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	if callbacks != nil {
		if override := callbacks.snapshotCreatedAt(); !override.IsZero() {
			createdAt = override.UTC().Truncate(time.Second)
		}
	}
	stem := preScopeV7SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(root), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(root), stem+preScopeV7AuditSuffix)
	if err := precheckPreScopeV7Targets(archivePath, auditPath); err != nil {
		return nil, err
	}
	// Validate the complete snapshot topology before taking the workspace lock.
	// The locked inventory below remains authoritative.
	if _, err := inventoryPreScopeV7(root, false); err != nil {
		return nil, err
	}

	var result *PreScopeV7Snapshot
	commit := storage.NewCommitService(TransactionsDir(root), []string{root})
	_, err = commit.CommitPlannedArtifacts(func() ([]storage.ArtifactChange, error) {
		if err := precheckPreScopeV7Targets(archivePath, auditPath); err != nil {
			return nil, err
		}
		// Recovery, if any, has completed under the workspace lock. Rebuild the
		// candidate from that exact coherent source state before capturing bytes.
		plan, err := DryRunV7WorkspaceConversion(dryRunOptions)
		if err != nil {
			return nil, fmt.Errorf("pre-scope v7 candidate changed before snapshot publication: %w", err)
		}
		if err := ValidateV7WorkspaceConversionPlan(plan); err != nil {
			return nil, err
		}
		inventory, err := inventoryPreScopeV7(root, true)
		if err != nil {
			return nil, err
		}
		if callbacks != nil {
			callbacks.didInventorySnapshot()
		}
		confirmedPlan, err := DryRunV7WorkspaceConversion(dryRunOptions)
		if err != nil {
			return nil, fmt.Errorf("pre-scope v7 candidate changed while snapshotting: %w", err)
		}
		if !reflect.DeepEqual(plan, confirmedPlan) {
			return nil, fmt.Errorf("pre-scope v7 candidate changed while snapshotting")
		}
		current, err := inventoryPreScopeV7(root, false)
		if err != nil {
			return nil, err
		}
		if err := comparePreScopeV7Inventories(inventory, current); err != nil {
			return nil, err
		}
		audit := renderPreScopeV7Audit(createdAt, plan, inventory)
		archive, err := buildPreScopeV7Archive(inventory.files, audit)
		if err != nil {
			return nil, err
		}
		if callbacks != nil {
			if err := callbacks.willPublishSnapshot(archivePath, auditPath); err != nil {
				return nil, err
			}
		}
		archiveSum := sha256.Sum256(archive)
		absent := ""
		result = &PreScopeV7Snapshot{
			CreatedAt: createdAt, ArchivePath: archivePath, AuditPath: auditPath,
			ArchiveSHA256: hex.EncodeToString(archiveSum[:]),
			Files:         publicPreScopeV7Files(inventory.files),
		}
		return []storage.ArtifactChange{
			{Path: archivePath, Data: archive, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
			{Path: auditPath, Data: audit, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("pre-scope v7 snapshot failed: %w", err)
	}
	return result, nil
}

func precheckPreScopeV7Targets(paths ...string) error {
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("inspect pre-scope v7 snapshot destination %s: %w", path, err)
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("pre-scope v7 snapshot destination is a symlink, refusing to replace: %s", path)
		default:
			return fmt.Errorf("pre-scope v7 snapshot destination already exists, refusing to overwrite: %s", path)
		}
	}
	return nil
}

//nolint:govet // Walk callbacks use scoped errors independently of the outer traversal result.
func inventoryPreScopeV7(root string, captureData bool) (*preScopeV7Inventory, error) {
	inventory := &preScopeV7Inventory{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if err := validatePreSimplificationV5RelativePath(rel); err != nil {
			return fmt.Errorf("unsafe pre-scope v7 source path %s: %w", path, err)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pre-scope v7 snapshot refuses symlink source: %s", path)
		}
		if isPreConsolidationV6Excluded(rel) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect pre-scope v7 source %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pre-scope v7 snapshot refuses non-regular source: %s", path)
		}
		source, err := readPreScopeV7Source(
			path,
			filepath.ToSlash(filepath.Join("workspace", rel)),
			preConsolidationV6Kind(rel),
			captureData,
		)
		if err != nil {
			return err
		}
		inventory.files = append(inventory.files, source)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory workspace for pre-scope v7 snapshot: %w", err)
	}

	sort.Slice(inventory.files, func(left, right int) bool {
		return inventory.files[left].Path < inventory.files[right].Path
	})
	for index := 1; index < len(inventory.files); index++ {
		if inventory.files[index-1].Path == inventory.files[index].Path {
			return nil, fmt.Errorf(
				"pre-scope v7 archive path collision %q from %s and %s",
				inventory.files[index].Path,
				inventory.files[index-1].sourcePath,
				inventory.files[index].sourcePath,
			)
		}
	}
	return inventory, nil
}

func readPreScopeV7Source(
	path string,
	archivePath string,
	kind string,
	captureData bool,
) (preScopeV7Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return preScopeV7Source{}, fmt.Errorf("read pre-scope v7 source %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	source := preScopeV7Source{
		PreScopeV7SnapshotFile: PreScopeV7SnapshotFile{
			Path: archivePath, Kind: kind, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]),
		},
		sourcePath: filepath.Clean(path),
	}
	if captureData {
		source.data = data
	}
	return source, nil
}

func comparePreScopeV7Inventories(expected *preScopeV7Inventory, current *preScopeV7Inventory) error {
	if len(expected.files) != len(current.files) {
		return fmt.Errorf("pre-scope v7 snapshot aborted: source inventory changed while snapshotting")
	}
	for index := range expected.files {
		left, right := expected.files[index], current.files[index]
		if left.Path != right.Path || left.sourcePath != right.sourcePath ||
			left.Size != right.Size || left.SHA256 != right.SHA256 {
			return fmt.Errorf("pre-scope v7 snapshot aborted: source changed while snapshotting: %s", left.sourcePath)
		}
	}
	return nil
}

func renderPreScopeV7Audit(
	createdAt time.Time,
	plan V7WorkspaceConversionPlan,
	inventory *preScopeV7Inventory,
) []byte {
	var totalBytes int64
	kindCounts := make(map[string]int)
	for _, source := range inventory.files {
		totalBytes += source.Size
		kindCounts[source.Kind]++
	}
	actionCounts := make(map[string]int)
	droppedScopes := 0
	for _, entry := range plan.Audit {
		actionCounts[entry.Action]++
		if entry.Field == "scope_definition" {
			droppedScopes++
		}
	}
	parked := 0
	tagged := 0
	for _, project := range plan.Catalog.Projects {
		if len(project.Tags) > 0 {
			tagged++
		}
		for _, tag := range project.Tags {
			if tag == models.ParkedProjectTag {
				parked++
				break
			}
		}
	}

	var out strings.Builder
	out.WriteString("# Pre-scope schema-7 migration audit\n\n")
	fmt.Fprintf(&out, "- Created at: `%s`\n", createdAt.UTC().Format(time.RFC3339))
	out.WriteString("- Schema-7 candidate validation: complete\n")
	fmt.Fprintf(&out, "- Snapshotted files: %d\n", len(inventory.files))
	fmt.Fprintf(&out, "- Snapshotted source bytes: %d\n", totalBytes)
	fmt.Fprintf(&out, "- Project task documents: %d\n", kindCounts["project document"])
	fmt.Fprintf(&out, "- Notes: %d\n", kindCounts["note"])
	fmt.Fprintf(&out, "- Attachments: %d\n", kindCounts["attachment"])
	fmt.Fprintf(&out, "- Cataloged projects: %d\n", len(plan.Catalog.Projects))
	fmt.Fprintf(&out, "- Projects carrying at least one tag: %d\n", tagged)
	fmt.Fprintf(&out, "- Projects parked by the reserved `%s` tag: %d\n", models.ParkedProjectTag, parked)
	fmt.Fprintf(&out, "- Retired grouping definitions discarded: %d\n", droppedScopes)
	fmt.Fprintf(&out, "- Retired names renamed to avoid the reserved tag: %d\n", actionCounts["renamed"])
	fmt.Fprintf(&out, "- Retired names rewritten to canonical tag bytes: %d\n", actionCounts["sanitized"])
	fmt.Fprintf(&out, "- Retired names that folded onto one tag: %d\n", actionCounts["deduped"]+actionCounts["merged"])

	out.WriteString("\n## Source-relative SHA-256 inventory\n\n")
	out.WriteString("| Archive path | Kind | Bytes | SHA-256 |\n")
	out.WriteString("| --- | --- | ---: | --- |\n")
	for _, source := range inventory.files {
		fmt.Fprintf(&out, "| `%s` | %s | %d | `%s` |\n",
			escapePreSimplificationV5AuditCell(source.Path),
			escapePreSimplificationV5AuditCell(source.Kind),
			source.Size,
			source.SHA256,
		)
	}

	out.WriteString("\n## Schema-7 project tags\n\n")
	out.WriteString("| Project | Name | Archived | Tags |\n")
	out.WriteString("| --- | --- | --- | --- |\n")
	for _, project := range plan.Catalog.Projects {
		fmt.Fprintf(&out, "| `%s` | %s | %t | %s |\n",
			escapePreSimplificationV5AuditCell(project.ID),
			escapePreSimplificationV5AuditCell(project.Name),
			project.Archived,
			escapePreSimplificationV5AuditCell(strings.Join(project.Tags, " ")),
		)
	}

	out.WriteString("\n## Conversion decisions\n\n")
	if len(plan.Audit) == 0 {
		out.WriteString("None.\n")
		return []byte(out.String())
	}
	out.WriteString("| Project | Field | Action | From | To | Reason |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range plan.Audit {
		fmt.Fprintf(&out, "| `%s` | `%s` | %s | %s | %s | %s |\n",
			escapePreSimplificationV5AuditCell(entry.ProjectID),
			escapePreSimplificationV5AuditCell(entry.Field),
			escapePreSimplificationV5AuditCell(entry.Action),
			escapePreSimplificationV5AuditCell(entry.From),
			escapePreSimplificationV5AuditCell(entry.To),
			escapePreSimplificationV5AuditCell(entry.Reason),
		)
	}
	return []byte(out.String())
}

func buildPreScopeV7Archive(sources []preScopeV7Source, audit []byte) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create pre-scope v7 snapshot compressor: %w", err)
	}
	gzipWriter.ModTime = time.Time{}
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	writeEntry := func(name string, data []byte) error {
		header := &tar.Header{
			Name: name, Mode: int64(FilePermissions), Size: int64(len(data)),
			ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatPAX, Typeflag: tar.TypeReg,
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
			return nil, fmt.Errorf("write pre-scope v7 archive entry %s: %w", source.Path, err)
		}
	}
	if err := writeEntry(preScopeV7AuditEntry, audit); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("write pre-scope v7 audit entry: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("finalize pre-scope v7 snapshot tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("finalize pre-scope v7 snapshot gzip: %w", err)
	}
	return buffer.Bytes(), nil
}

func publicPreScopeV7Files(sources []preScopeV7Source) []PreScopeV7SnapshotFile {
	files := make([]PreScopeV7SnapshotFile, len(sources))
	for index := range sources {
		files[index] = sources[index].PreScopeV7SnapshotFile
	}
	return files
}
