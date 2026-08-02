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

	"github.com/sglavoie/tt/internal/storage"
)

const (
	preConsolidationV6SnapshotPrefix = "pre-consolidation-v6-"
	preConsolidationV6Timestamp      = "20060102T150405Z"
	preConsolidationV6ArchiveSuffix  = ".tar.gz"
	preConsolidationV6AuditSuffix    = ".audit.md"
	preConsolidationV6AuditEntry     = "pre-consolidation-v6-audit.md"

	preConsolidationV6Permissions fs.FileMode = 0o400
)

// ErrImmutablePreConsolidationV6Snapshot classifies attempts to mutate a
// canonical pre-v6 rollback artifact.
var ErrImmutablePreConsolidationV6Snapshot = errors.New("config: pre-consolidation v6 snapshots are immutable")

// PreConsolidationV6SnapshotFile is one byte-exact source in the pre-v6
// archive. Path is portable and source-relative rather than machine-absolute.
type PreConsolidationV6SnapshotFile struct {
	Path   string
	Kind   string
	Size   int64
	SHA256 string
}

// PreConsolidationV6Snapshot describes one atomically published rollback
// archive and its matching migration audit.
type PreConsolidationV6Snapshot struct {
	CreatedAt     time.Time
	ArchivePath   string
	AuditPath     string
	ArchiveSHA256 string
	Files         []PreConsolidationV6SnapshotFile
}

// PreConsolidationV6SnapshotPair is one canonical rollback archive and its
// matching human-readable migration audit.
type PreConsolidationV6SnapshotPair struct {
	CreatedAt   time.Time
	ArchivePath string
	AuditPath   string
}

// PreConsolidationV6SnapshotInspection is the read-only diagnostics view of
// the protected backup directory. Pairs contains only complete canonical
// archive/audit pairs; Issues describes malformed, empty, or orphaned
// application-owned artifacts.
type PreConsolidationV6SnapshotInspection struct {
	HasArtifacts bool
	Pairs        []PreConsolidationV6SnapshotPair
	Issues       []string
}

// PreConsolidationV6SnapshotOptions identifies the schema-5 workspace and an
// optional isolated machine-state source. It has the same source semantics as
// V6WorkspaceDryRunOptions.
type PreConsolidationV6SnapshotOptions struct {
	DataDir          string
	MachineStatePath string
}

type preConsolidationV6SnapshotHooks struct {
	now            time.Time
	afterInventory func()
	beforePublish  func(archivePath, auditPath string) error
}

type preConsolidationV6Source struct {
	PreConsolidationV6SnapshotFile
	sourcePath string
	data       []byte
}

type preConsolidationV6Inventory struct {
	files []preConsolidationV6Source
}

type preConsolidationV6TaskFact struct {
	projectID string
	taskID    string
	state     string
	priority  int
	decisions int
}

// PreConsolidationV6BackupsDir returns the application-owned directory for
// immutable pre-v6 rollback artifacts.
func PreConsolidationV6BackupsDir(dataDir string) string {
	return filepath.Join(InternalDir(filepath.Clean(dataDir)), "backups")
}

// IsPreConsolidationV6SnapshotPath reports whether path is one canonical
// timestamped pre-v6 archive or audit sidecar.
func IsPreConsolidationV6SnapshotPath(dataDir, path string) bool {
	backupsDir := filepath.Clean(PreConsolidationV6BackupsDir(dataDir))
	candidate := filepath.Clean(path)
	if filepath.Dir(candidate) != backupsDir {
		return false
	}
	name := filepath.Base(candidate)
	if !strings.HasPrefix(name, preConsolidationV6SnapshotPrefix) {
		return false
	}
	stampAndSuffix := strings.TrimPrefix(name, preConsolidationV6SnapshotPrefix)
	var stamp string
	switch {
	case strings.HasSuffix(stampAndSuffix, preConsolidationV6ArchiveSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preConsolidationV6ArchiveSuffix)
	case strings.HasSuffix(stampAndSuffix, preConsolidationV6AuditSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preConsolidationV6AuditSuffix)
	default:
		return false
	}
	_, err := time.Parse(preConsolidationV6Timestamp, stamp)
	return err == nil
}

// InspectPreConsolidationV6Snapshots validates immutable pre-v6 rollback
// artifacts without changing them. Unrelated files in the shared backups
// directory are ignored.
func InspectPreConsolidationV6Snapshots(dataDir string) (PreConsolidationV6SnapshotInspection, error) {
	var result PreConsolidationV6SnapshotInspection
	backupsDir := PreConsolidationV6BackupsDir(dataDir)
	dirInfo, err := os.Lstat(backupsDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("inspect pre-consolidation v6 backups directory: %w", err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return result, fmt.Errorf("pre-consolidation v6 backups path is not a regular directory: %s", backupsDir)
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return result, fmt.Errorf("read pre-consolidation v6 backups directory: %w", err)
	}

	type pairState struct {
		createdAt   time.Time
		archivePath string
		auditPath   string
	}
	states := make(map[string]*pairState)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, preConsolidationV6SnapshotPrefix) {
			continue
		}
		if strings.HasSuffix(name, ".lock") {
			target := strings.TrimSuffix(filepath.Join(backupsDir, name), ".lock")
			if IsPreConsolidationV6SnapshotPath(dataDir, target) {
				continue
			}
		}
		result.HasArtifacts = true
		path := filepath.Join(backupsDir, name)
		if !IsPreConsolidationV6SnapshotPath(dataDir, path) {
			result.Issues = append(result.Issues, fmt.Sprintf("malformed pre-consolidation v6 snapshot artifact %q", name))
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return result, fmt.Errorf("inspect pre-consolidation v6 snapshot artifact %q: %w", name, infoErr)
		}
		if !info.Mode().IsRegular() {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-consolidation v6 snapshot artifact %q is not a regular file", name))
		} else if info.Size() == 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-consolidation v6 snapshot artifact %q is empty", name))
		}

		isArchive := strings.HasSuffix(name, preConsolidationV6ArchiveSuffix)
		stem := strings.TrimSuffix(name, preConsolidationV6AuditSuffix)
		if isArchive {
			stem = strings.TrimSuffix(name, preConsolidationV6ArchiveSuffix)
		}
		state := states[stem]
		if state == nil {
			stamp := strings.TrimPrefix(stem, preConsolidationV6SnapshotPrefix)
			createdAt, parseErr := time.Parse(preConsolidationV6Timestamp, stamp)
			if parseErr != nil {
				return result, fmt.Errorf("parse canonical pre-consolidation v6 snapshot timestamp %q: %w", stamp, parseErr)
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
			result.Issues = append(result.Issues, fmt.Sprintf("pre-consolidation v6 snapshot %q is missing its archive", stem))
		case state.auditPath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("pre-consolidation v6 snapshot %q is missing its audit report", stem))
		default:
			result.Pairs = append(result.Pairs, PreConsolidationV6SnapshotPair{
				CreatedAt: state.createdAt, ArchivePath: state.archivePath, AuditPath: state.auditPath,
			})
		}
	}
	sort.Strings(result.Issues)
	return result, nil
}

// RefusePreConsolidationV6SnapshotMutation is the application-level immutable
// path guard for future migration and cleanup code.
func RefusePreConsolidationV6SnapshotMutation(dataDir, path string) error {
	if !IsPreConsolidationV6SnapshotPath(dataDir, path) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrImmutablePreConsolidationV6Snapshot, filepath.Clean(path))
}

// CreatePreConsolidationV6Snapshot first constructs and validates the complete
// schema-6 candidate, then publishes a byte-exact immutable archive and audit.
// It does not activate schema 6 or change any live source artifact.
func CreatePreConsolidationV6Snapshot(options PreConsolidationV6SnapshotOptions) (*PreConsolidationV6Snapshot, error) {
	return createPreConsolidationV6Snapshot(options, nil)
}

func createPreConsolidationV6Snapshot(
	options PreConsolidationV6SnapshotOptions,
	hooks *preConsolidationV6SnapshotHooks,
) (*PreConsolidationV6Snapshot, error) {
	root, err := validateV5DryRunRoot(options.DataDir)
	if err != nil {
		return nil, err
	}
	options.DataDir = root
	dryRunOptions := V6WorkspaceDryRunOptions{
		DataDir: root, MachineStatePath: options.MachineStatePath,
	}

	// This validation happens before the commit service can create a lock or
	// transaction artifact. Invalid candidates therefore leave no snapshot state.
	validatedPlan, err := DryRunV6WorkspaceConversion(dryRunOptions)
	if err != nil {
		return nil, fmt.Errorf("pre-consolidation v6 candidate validation failed; workspace was not changed: %w", err)
	}
	if err := ValidateV6WorkspaceConversionPlan(validatedPlan); err != nil {
		return nil, fmt.Errorf("pre-consolidation v6 candidate validation failed; workspace was not changed: %w", err)
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	if hooks != nil && !hooks.now.IsZero() {
		createdAt = hooks.now.UTC().Truncate(time.Second)
	}
	stem := preConsolidationV6SnapshotPrefix + createdAt.Format(preConsolidationV6Timestamp)
	archivePath := filepath.Join(PreConsolidationV6BackupsDir(root), stem+preConsolidationV6ArchiveSuffix)
	auditPath := filepath.Join(PreConsolidationV6BackupsDir(root), stem+preConsolidationV6AuditSuffix)
	if err := precheckPreConsolidationV6Targets(archivePath, auditPath); err != nil {
		return nil, err
	}
	machinePath, err := preConsolidationV6MachineSource(validatedPlan)
	if err != nil {
		return nil, err
	}
	// Validate the complete snapshot topology before taking the workspace lock.
	// The locked inventory below remains authoritative.
	if _, err := inventoryPreConsolidationV6(root, machinePath, false); err != nil {
		return nil, err
	}

	var result *PreConsolidationV6Snapshot
	commit := storage.NewCommitService(TransactionsDir(root), []string{root})
	_, err = commit.CommitPlannedArtifacts(func() ([]storage.ArtifactChange, error) {
		if err := precheckPreConsolidationV6Targets(archivePath, auditPath); err != nil {
			return nil, err
		}
		// Recovery, if any, has completed under the workspace lock. Rebuild the
		// candidate from that exact coherent source state before capturing bytes.
		plan, err := DryRunV6WorkspaceConversion(dryRunOptions)
		if err != nil {
			return nil, fmt.Errorf("pre-consolidation v6 candidate changed before snapshot publication: %w", err)
		}
		if err := ValidateV6WorkspaceConversionPlan(plan); err != nil {
			return nil, err
		}
		machinePath, err := preConsolidationV6MachineSource(plan)
		if err != nil {
			return nil, err
		}
		inventory, err := inventoryPreConsolidationV6(root, machinePath, true)
		if err != nil {
			return nil, err
		}
		if hooks != nil && hooks.afterInventory != nil {
			hooks.afterInventory()
		}
		confirmedPlan, err := DryRunV6WorkspaceConversion(dryRunOptions)
		if err != nil {
			return nil, fmt.Errorf("pre-consolidation v6 candidate changed while snapshotting: %w", err)
		}
		if !reflect.DeepEqual(plan, confirmedPlan) {
			return nil, fmt.Errorf("pre-consolidation v6 candidate changed while snapshotting")
		}
		current, err := inventoryPreConsolidationV6(root, machinePath, false)
		if err != nil {
			return nil, err
		}
		if err := comparePreConsolidationV6Inventories(inventory, current); err != nil {
			return nil, err
		}
		facts, err := collectPreConsolidationV6TaskFacts(plan, inventory)
		if err != nil {
			return nil, err
		}
		audit := renderPreConsolidationV6Audit(createdAt, plan, inventory, facts)
		archive, err := buildPreConsolidationV6Archive(inventory.files, audit)
		if err != nil {
			return nil, err
		}
		if hooks != nil && hooks.beforePublish != nil {
			if err := hooks.beforePublish(archivePath, auditPath); err != nil {
				return nil, err
			}
		}
		archiveSum := sha256.Sum256(archive)
		absent := ""
		result = &PreConsolidationV6Snapshot{
			CreatedAt: createdAt, ArchivePath: archivePath, AuditPath: auditPath,
			ArchiveSHA256: hex.EncodeToString(archiveSum[:]),
			Files:         publicPreConsolidationV6Files(inventory.files),
		}
		return []storage.ArtifactChange{
			{Path: archivePath, Data: archive, Mode: preConsolidationV6Permissions, ExpectedRevision: &absent},
			{Path: auditPath, Data: audit, Mode: preConsolidationV6Permissions, ExpectedRevision: &absent},
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("pre-consolidation v6 snapshot failed: %w", err)
	}
	return result, nil
}

func precheckPreConsolidationV6Targets(paths ...string) error {
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("inspect pre-consolidation v6 snapshot destination %s: %w", path, err)
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("pre-consolidation v6 snapshot destination is a symlink, refusing to replace: %s", path)
		default:
			return fmt.Errorf("pre-consolidation v6 snapshot destination already exists, refusing to overwrite: %s", path)
		}
	}
	return nil
}

func preConsolidationV6MachineSource(plan V6WorkspaceConversionPlan) (string, error) {
	for _, artifact := range plan.Artifacts {
		if artifact.Kind == "machine" {
			return filepath.Clean(artifact.SourcePath), nil
		}
	}
	return "", fmt.Errorf("validated schema-6 candidate has no machine-state artifact")
}

func inventoryPreConsolidationV6(
	root string,
	machinePath string,
	captureData bool,
) (*preConsolidationV6Inventory, error) {
	inventory := &preConsolidationV6Inventory{}
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
			return fmt.Errorf("unsafe pre-consolidation v6 source path %s: %w", path, err)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pre-consolidation v6 snapshot refuses symlink source: %s", path)
		}
		if isPreConsolidationV6Excluded(rel) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect pre-consolidation v6 source %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pre-consolidation v6 snapshot refuses non-regular source: %s", path)
		}
		source, err := readPreConsolidationV6Source(
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
		return nil, fmt.Errorf("inventory workspace for pre-consolidation v6 snapshot: %w", err)
	}

	workspaceID, err := preSimplificationV5WorkspaceID(root)
	if err != nil {
		return nil, err
	}
	var machineCandidates []string
	if strings.TrimSpace(machinePath) != "" {
		machineCandidates = []string{machinePath, machinePath + ".bak"}
	}
	for _, candidate := range machineCandidates {
		info, statErr := os.Lstat(candidate)
		if errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return nil, fmt.Errorf("inspect pre-consolidation v6 machine source %s: %w", candidate, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("pre-consolidation v6 machine source is not a regular file: %s", candidate)
		}
		archiveName := machineStateFileName
		if strings.HasSuffix(candidate, ".bak") {
			archiveName += ".bak"
		}
		source, err := readPreConsolidationV6Source(
			candidate,
			filepath.ToSlash(filepath.Join("machine", runtimeDirName, workspaceID, archiveName)),
			"machine state",
			captureData,
		)
		if err != nil {
			return nil, err
		}
		inventory.files = append(inventory.files, source)
	}

	sort.Slice(inventory.files, func(left, right int) bool {
		if inventory.files[left].Path != inventory.files[right].Path {
			return inventory.files[left].Path < inventory.files[right].Path
		}
		return inventory.files[left].sourcePath < inventory.files[right].sourcePath
	})
	for index := 1; index < len(inventory.files); index++ {
		if inventory.files[index-1].Path == inventory.files[index].Path {
			return nil, fmt.Errorf(
				"pre-consolidation v6 archive path collision %q from %s and %s",
				inventory.files[index].Path,
				inventory.files[index-1].sourcePath,
				inventory.files[index].sourcePath,
			)
		}
	}
	return inventory, nil
}

func isPreConsolidationV6Excluded(rel string) bool {
	clean := filepath.Clean(rel)
	if isMigrationRootExcluded(clean) {
		return true
	}
	if clean == filepath.Join(dataDirName, "backups") {
		return true
	}
	base := filepath.Base(clean)
	return strings.HasSuffix(base, ".lock") ||
		strings.Contains(base, ".tmp.") ||
		strings.HasPrefix(base, ".tt-move-")
}

func preConsolidationV6Kind(rel string) string {
	clean := filepath.ToSlash(filepath.Clean(rel))
	base := filepath.Base(clean)
	switch {
	case clean == ".tt/workspace.json", clean == ".tt/planning.json":
		return "synced domain"
	case strings.HasPrefix(clean, ".tt/transactions/"):
		return "transaction state"
	case strings.HasPrefix(clean, ".tt/undo/"):
		return "undo ledger"
	case strings.HasPrefix(clean, ".tt/global-notes/"), strings.Contains(clean, "/notes/"):
		return "note"
	case strings.Contains(clean, "/.attachments/"):
		return "attachment"
	case base == "tasks.md":
		return "project document"
	default:
		return "workspace data"
	}
}

func readPreConsolidationV6Source(
	path string,
	archivePath string,
	kind string,
	captureData bool,
) (preConsolidationV6Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return preConsolidationV6Source{}, fmt.Errorf("read pre-consolidation v6 source %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	source := preConsolidationV6Source{
		PreConsolidationV6SnapshotFile: PreConsolidationV6SnapshotFile{
			Path: archivePath, Kind: kind, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]),
		},
		sourcePath: filepath.Clean(path),
	}
	if captureData {
		source.data = data
	}
	return source, nil
}

func comparePreConsolidationV6Inventories(
	expected *preConsolidationV6Inventory,
	current *preConsolidationV6Inventory,
) error {
	if len(expected.files) != len(current.files) {
		return fmt.Errorf("pre-consolidation v6 snapshot aborted: source inventory changed while snapshotting")
	}
	for index := range expected.files {
		left, right := expected.files[index], current.files[index]
		if left.Path != right.Path || left.sourcePath != right.sourcePath ||
			left.Size != right.Size || left.SHA256 != right.SHA256 {
			return fmt.Errorf("pre-consolidation v6 snapshot aborted: source changed while snapshotting: %s", left.sourcePath)
		}
	}
	return nil
}

func collectPreConsolidationV6TaskFacts(
	plan V6WorkspaceConversionPlan,
	inventory *preConsolidationV6Inventory,
) ([]preConsolidationV6TaskFact, error) {
	bySourcePath := make(map[string][]byte, len(inventory.files))
	for _, source := range inventory.files {
		bySourcePath[source.sourcePath] = source.data
	}
	var facts []preConsolidationV6TaskFact
	for _, project := range plan.Projects {
		if !project.SourceExists {
			continue
		}
		data, exists := bySourcePath[filepath.Clean(project.SourcePath)]
		if !exists {
			return nil, fmt.Errorf("validated project %q is missing from the captured snapshot inventory", project.ProjectID)
		}
		document, err := storage.ParseV5ProjectDocument(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse captured v5 project %q for migration audit: %w", project.ProjectID, err)
		}
		for _, task := range document.Tasks {
			facts = append(facts, preConsolidationV6TaskFact{
				projectID: project.ProjectID,
				taskID:    task.ID,
				state:     task.State,
				priority:  task.Priority,
				decisions: len(task.Decisions),
			})
		}
	}
	return facts, nil
}

func renderPreConsolidationV6Audit(
	createdAt time.Time,
	plan V6WorkspaceConversionPlan,
	inventory *preConsolidationV6Inventory,
	facts []preConsolidationV6TaskFact,
) []byte {
	var totalBytes int64
	kindCounts := make(map[string]int)
	for _, source := range inventory.files {
		totalBytes += source.Size
		kindCounts[source.Kind]++
	}
	stateCounts := make(map[string]int)
	priorityCounts := make(map[int]int)
	decisionCount := 0
	for _, fact := range facts {
		stateCounts[fact.state]++
		priorityCounts[fact.priority]++
		decisionCount += fact.decisions
	}
	dependencyCount := 0
	recurrenceCount := 0
	for _, entry := range plan.TaskAudit {
		switch {
		case strings.HasPrefix(entry.Field, "depends_on["):
			dependencyCount++
		case entry.Field == "recurrence":
			recurrenceCount++
		}
	}
	planningRepairs := filterV6DomainAudit(plan.Domains.Audit, "planning", "")
	unfiledChanges := filterV6DomainAudit(plan.Domains.Audit, "workspace", "")
	customContexts := filterV6DomainAudit(plan.Domains.Audit, "machine", "custom_context")

	var out strings.Builder
	out.WriteString("# Pre-consolidation schema-6 migration audit\n\n")
	fmt.Fprintf(&out, "- Created at: `%s`\n", createdAt.UTC().Format(time.RFC3339))
	out.WriteString("- Schema-6 candidate validation: complete\n")
	fmt.Fprintf(&out, "- Snapshotted files: %d\n", len(inventory.files))
	fmt.Fprintf(&out, "- Snapshotted source bytes: %d\n", totalBytes)
	fmt.Fprintf(&out, "- Project task documents: %d\n", kindCounts["project document"])
	fmt.Fprintf(&out, "- Notes: %d\n", kindCounts["note"])
	fmt.Fprintf(&out, "- Attachments: %d\n", kindCounts["attachment"])
	fmt.Fprintf(&out, "- Transaction-state files: %d\n", kindCounts["transaction state"])
	fmt.Fprintf(&out, "- Undo-ledger files: %d\n", kindCounts["undo ledger"])
	fmt.Fprintf(&out, "- Dependency references discarded: %d\n", dependencyCount)
	fmt.Fprintf(&out, "- Recurrences discarded: %d\n", recurrenceCount)
	fmt.Fprintf(&out, "- Decisions converted to comments: %d\n", decisionCount)
	fmt.Fprintf(&out, "- Planning repairs: %d\n", len(planningRepairs))
	fmt.Fprintf(&out, "- Unfiled catalog changes: %d\n", len(unfiledChanges))
	fmt.Fprintf(&out, "- Custom contexts discarded: %d\n", len(customContexts))

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

	out.WriteString("\n## Lifecycle state mappings\n\n")
	out.WriteString("| Schema 5 | Schema 6 lifecycle | Planning effect | Tasks |\n")
	out.WriteString("| --- | --- | --- | ---: |\n")
	stateMappings := []struct{ from, to, planning string }{
		{"inbox", "inbox", "none"},
		{"ready", "backlog", "none"},
		{"next", "backlog", "add to Next"},
		{"in_progress", "doing", "none"},
		{"waiting", "waiting", "none"},
		{"paused", "paused", "none"},
		{"someday", "someday", "none"},
		{"done", "done", "none"},
		{"canceled", "canceled", "none"},
	}
	for _, mapping := range stateMappings {
		fmt.Fprintf(&out, "| `%s` | `%s` | %s | %d |\n", mapping.from, mapping.to, mapping.planning, stateCounts[mapping.from])
	}

	out.WriteString("\n## Priority mappings\n\n")
	out.WriteString("| Schema 5 | Schema 6 | Label | Tasks |\n")
	out.WriteString("| ---: | ---: | --- | ---: |\n")
	priorityMappings := []struct {
		from, to int
		label    string
	}{
		{0, 0, "Unset"}, {1, 1, "High"}, {2, 1, "High"},
		{3, 0, "Unset"}, {4, 3, "Low"}, {5, 3, "Low"},
	}
	for _, mapping := range priorityMappings {
		fmt.Fprintf(&out, "| %d | %d | %s | %d |\n", mapping.from, mapping.to, mapping.label, priorityCounts[mapping.from])
	}

	renderPreConsolidationV6TaskAudit(&out, plan, "discarded", "## Retired task values")
	renderPreConsolidationV6TaskAudit(&out, plan, "converted", "## Decision conversions")
	renderPreConsolidationV6DomainAudit(&out, "## Planning repairs", planningRepairs)
	renderPreConsolidationV6DomainAudit(&out, "## Unfiled changes", unfiledChanges)
	renderPreConsolidationV6DomainAudit(&out, "## Custom-context removals", customContexts)
	return []byte(out.String())
}

func filterV6DomainAudit(entries []V6DomainMigrationAuditEntry, domain, field string) []V6DomainMigrationAuditEntry {
	filtered := make([]V6DomainMigrationAuditEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Domain == domain && (field == "" || entry.Field == field) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func renderPreConsolidationV6TaskAudit(out *strings.Builder, plan V6WorkspaceConversionPlan, action, heading string) {
	out.WriteString("\n" + heading + "\n\n")
	count := 0
	for _, entry := range plan.TaskAudit {
		if entry.Action == action {
			count++
		}
	}
	if count == 0 {
		out.WriteString("None.\n")
		return
	}
	out.WriteString("| Project | Task | Field | Action | From | To | Reason |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range plan.TaskAudit {
		if entry.Action != action {
			continue
		}
		fmt.Fprintf(out, "| `%s` | `%s` | `%s` | %s | %s | %s | %s |\n",
			escapePreSimplificationV5AuditCell(entry.ProjectID),
			escapePreSimplificationV5AuditCell(entry.TaskID),
			escapePreSimplificationV5AuditCell(entry.Field),
			escapePreSimplificationV5AuditCell(entry.Action),
			escapePreSimplificationV5AuditCell(entry.From),
			escapePreSimplificationV5AuditCell(entry.To),
			escapePreSimplificationV5AuditCell(entry.Reason),
		)
	}
}

func renderPreConsolidationV6DomainAudit(
	out *strings.Builder,
	heading string,
	entries []V6DomainMigrationAuditEntry,
) {
	out.WriteString("\n" + heading + "\n\n")
	if len(entries) == 0 {
		out.WriteString("None.\n")
		return
	}
	out.WriteString("| Domain | Project | Entity | Field | Action | From | To | Reason |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range entries {
		fmt.Fprintf(out, "| %s | `%s` | `%s` | `%s` | %s | %s | %s | %s |\n",
			escapePreSimplificationV5AuditCell(entry.Domain),
			escapePreSimplificationV5AuditCell(entry.ProjectID),
			escapePreSimplificationV5AuditCell(entry.EntityID),
			escapePreSimplificationV5AuditCell(entry.Field),
			escapePreSimplificationV5AuditCell(entry.Action),
			escapePreSimplificationV5AuditCell(entry.From),
			escapePreSimplificationV5AuditCell(entry.To),
			escapePreSimplificationV5AuditCell(entry.Reason),
		)
	}
}

func buildPreConsolidationV6Archive(sources []preConsolidationV6Source, audit []byte) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create pre-consolidation v6 snapshot compressor: %w", err)
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
			return nil, fmt.Errorf("write pre-consolidation v6 archive entry %s: %w", source.Path, err)
		}
	}
	if err := writeEntry(preConsolidationV6AuditEntry, audit); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("write pre-consolidation v6 audit entry: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("finalize pre-consolidation v6 snapshot tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("finalize pre-consolidation v6 snapshot gzip: %w", err)
	}
	return buffer.Bytes(), nil
}

func publicPreConsolidationV6Files(sources []preConsolidationV6Source) []PreConsolidationV6SnapshotFile {
	files := make([]PreConsolidationV6SnapshotFile, len(sources))
	for index := range sources {
		files[index] = sources[index].PreConsolidationV6SnapshotFile
	}
	return files
}
