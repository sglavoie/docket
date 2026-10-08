package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// Enrollment turns a desktop workspace into the first device of a synced one.
// It never converts the workspace in place. The original stays exactly where
// and as it is, a verified snapshot of it is kept beside this machine's other
// local state, and a new working copy is built from it outside iCloud: that
// copy is the enrolled workspace, the one its sync history and every later
// device descend from. The working copy's catalog is at
// SyncEnrolledConfigVersion and carries the sync block, so a tt that predates
// sync refuses it as newer than it supports rather than writing around a
// history it cannot see. A workspace that is never enrolled keeps its schema
// and its behavior.
//
// Each step is idempotent or resumable. The working copy is assembled in a
// staging directory and published by one rename, so an interrupted enrollment
// leaves either no working copy at all or a complete one; running enrollment
// again discards a stale staging directory or picks up a published copy where
// it stopped. Publishing the working copy's initial history is the caller's,
// through the sync engine, and it is resumable by the same rule.

// Enrollment record and snapshot formats.
const (
	EnrollmentRecordFormat   = "tt-sync-enrollment"
	enrollmentSnapshotFormat = "tt-enrollment-snapshot"
	enrollmentFormatVersion  = 1
	// enrollStagingMarker is the file that proves a staging directory is
	// enrollment's own, so a re-run removes nothing it did not create.
	enrollStagingMarker = ".tt-enrolling"
	enrollStagingSuffix = ".enrolling"
	snapshotPartialExt  = ".partial"
	snapshotManifest    = "snapshot.json"
)

// ErrAlreadyEnrolled refuses to enroll a workspace that is already an enrolled
// working copy.
var ErrAlreadyEnrolled = errors.New("config: workspace is already enrolled in sync")

// EnrollOptions names the three places enrollment touches.
type EnrollOptions struct {
	// Original is the data directory of the workspace being enrolled. It is
	// read, and never written beyond completing a crashed commit.
	Original string
	// WorkingCopy is the new local data directory that becomes the enrolled
	// workspace. It must not exist yet, or be empty, and it may not be inside
	// iCloud Drive, the original, or the exchange folder.
	WorkingCopy string
	// Exchange is the folder the devices exchange history through, typically in
	// iCloud Drive. Enrollment only records it for this device.
	Exchange string
	// SnapshotRoot is where the original's snapshot is kept. Empty is the
	// machine's local data root, under enrollments/.
	SnapshotRoot string
	// MachineHome is the home the iCloud placement check and the default
	// snapshot root resolve against; empty is the process home.
	MachineHome string
	// Now is the enrollment clock; nil is the system clock.
	Now func() time.Time
}

// EnrollmentRecord is the device-local record of how this device's working
// copy was enrolled, kept under the working copy's .tt/sync/, which is never
// exchanged. It is where this device's exchange folder is configured.
type EnrollmentRecord struct {
	Format         string    `json:"format"`
	FormatVersion  int       `json:"format_version"`
	WorkspaceID    string    `json:"workspace_id"`
	ExchangeFolder string    `json:"exchange_folder"`
	Original       string    `json:"original"`
	Snapshot       string    `json:"snapshot"`
	EnrolledAt     time.Time `json:"enrolled_at"`
}

// EnrollReport is what an enrollment did.
type EnrollReport struct {
	WorkspaceID string
	Original    string
	WorkingCopy string
	Snapshot    string
	Exchange    string
	FilesCopied int
	// Resumed reports that the working copy was already published by an
	// earlier, interrupted run, so this one copied nothing.
	Resumed bool
}

// enrollHook is the interruption seam tests stop an enrollment at; production
// leaves it nil.
var enrollHook func(stage string) error

func enrollStage(stage string) error {
	if enrollHook == nil {
		return nil
	}
	return enrollHook(stage)
}

// EnrollmentRecordPath is where a working copy keeps its enrollment record.
func EnrollmentRecordPath(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "sync", "enrollment.json")
}

// LoadEnrollmentRecord reads a working copy's enrollment record, reporting
// (nil, nil) for a workspace that has none.
func LoadEnrollmentRecord(dataDir string) (*EnrollmentRecord, error) {
	data, err := os.ReadFile(EnrollmentRecordPath(dataDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading enrollment record: %w", err)
	}
	var record EnrollmentRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decoding enrollment record: %w", err)
	}
	if record.Format != EnrollmentRecordFormat || record.FormatVersion != enrollmentFormatVersion {
		return nil, fmt.Errorf("enrollment record %s is %s v%d, not %s v%d",
			EnrollmentRecordPath(dataDir), record.Format, record.FormatVersion, EnrollmentRecordFormat, enrollmentFormatVersion)
	}
	return &record, nil
}

// SetEnrollmentExchangeFolder points this device's working copy at another
// exchange folder, for example after the iCloud folder moved.
func SetEnrollmentExchangeFolder(dataDir, folder string) error {
	record, err := LoadEnrollmentRecord(dataDir)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("%s is not an enrolled working copy", dataDir)
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	if err := validateExchangePlacement(dataDir, record.Original, abs); err != nil {
		return err
	}
	record.ExchangeFolder = abs
	return writeEnrollmentRecord(dataDir, record)
}

func writeEnrollmentRecord(dataDir string, record *EnrollmentRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := EnrollmentRecordPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		return fmt.Errorf("creating the sync directory: %w", err)
	}
	if err := util.AtomicWriteFile(path, append(data, '\n'), FilePermissions); err != nil {
		return fmt.Errorf("writing enrollment record: %w", err)
	}
	return nil
}

// EnrollWorkspace snapshots the original, builds the enrolled working copy
// from it, and makes the working copy this machine's active workspace. The
// working copy's history is not recorded here: the first sync runtime opened
// over it records the genesis transaction (see storage.CommitSyncGenesis).
func EnrollWorkspace(opts EnrollOptions) (*EnrollReport, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	original, workingCopy, exchange, err := enrollPaths(opts)
	if err != nil {
		return nil, err
	}
	catalog, err := NewDomainFileStore(original).LoadCatalogReadOnly()
	if err != nil {
		return nil, fmt.Errorf("enroll: reading the original workspace %s: %w", original, err)
	}
	if catalog.Enrolled() {
		return nil, fmt.Errorf("%w: %s is a working copy; enroll the workspace it came from, or keep using it", ErrAlreadyEnrolled, original)
	}
	if catalog.SchemaVersion != CurrentConfigVersion || catalog.WorkspaceID == "" {
		return nil, fmt.Errorf("enroll: %s is at schema %d; open it with this tt once so it migrates to schema %d, then enroll", original, catalog.SchemaVersion, CurrentConfigVersion)
	}
	if resumed, resumeErr := resumeEnrollment(workingCopy, catalog.WorkspaceID, original); resumeErr != nil || resumed != nil {
		if resumed != nil {
			resumed.Exchange = exchange
		}
		return resumed, resumeErr
	}
	if emptyErr := requireEmptyWorkingCopy(workingCopy); emptyErr != nil {
		return nil, emptyErr
	}
	snapshotRoot := opts.SnapshotRoot
	if snapshotRoot == "" {
		snapshotRoot = filepath.Join(localDataRootOn(opts.MachineHome), "enrollments")
	}
	if isSubpath(workingCopy, snapshotRoot) || isSubpath(original, snapshotRoot) {
		return nil, fmt.Errorf("enroll: the snapshot root %s must be outside the original and the working copy", snapshotRoot)
	}

	report := &EnrollReport{WorkspaceID: catalog.WorkspaceID, Original: original, WorkingCopy: workingCopy, Exchange: exchange}
	err = withWorkspaceTUILocks(original, workingCopy, func() error {
		commits := storage.NewCommitService(TransactionsDir(original), []string{original})
		return storage.WithWorkspaceCommitLocks([]*storage.CommitService{commits}, func() error {
			entries, invErr := inventoryWorkspace(original)
			if invErr != nil {
				return invErr
			}
			snapshot, snapErr := snapshotOriginal(entries, original, filepath.Join(snapshotRoot, catalog.WorkspaceID), now())
			if snapErr != nil {
				return snapErr
			}
			report.Snapshot = snapshot
			if hookErr := enrollStage("snapshot"); hookErr != nil {
				return hookErr
			}
			staging, copied, stageErr := stageWorkingCopy(entries, workingCopy)
			if stageErr != nil {
				return stageErr
			}
			report.FilesCopied = copied
			enrolledAt := now().UTC()
			if catalogErr := enrollStagedCatalog(staging, enrolledAt); catalogErr != nil {
				return catalogErr
			}
			record := &EnrollmentRecord{
				Format:         EnrollmentRecordFormat,
				FormatVersion:  enrollmentFormatVersion,
				WorkspaceID:    catalog.WorkspaceID,
				ExchangeFolder: exchange,
				Original:       original,
				Snapshot:       snapshot,
				EnrolledAt:     enrolledAt,
			}
			if recordErr := writeEnrollmentRecord(staging, record); recordErr != nil {
				return recordErr
			}
			if validateErr := validateSourceSnapshot(entries); validateErr != nil {
				return validateErr
			}
			if hookErr := enrollStage("staged"); hookErr != nil {
				return hookErr
			}
			return publishWorkingCopy(staging, workingCopy)
		})
	})
	if err != nil {
		return nil, err
	}
	if err := enrollStage("published"); err != nil {
		return nil, err
	}
	if err := WriteActiveWorkspacePointer(workingCopy, catalog.WorkspaceID); err != nil {
		return nil, fmt.Errorf("enroll: the working copy is complete at %s but could not be selected: %w", workingCopy, err)
	}
	return report, nil
}

// enrollPaths resolves the three paths and refuses any placement enrollment
// could not keep safe: a working copy inside iCloud Drive (iCloud would sync
// the live files the exchange exists to keep out of it), inside or around the
// original, or overlapping the exchange folder.
func enrollPaths(opts EnrollOptions) (original, workingCopy, exchange string, err error) {
	for _, required := range []struct{ name, value string }{
		{"the original workspace", opts.Original},
		{"the working copy", opts.WorkingCopy},
		{"the exchange folder", opts.Exchange},
	} {
		if strings.TrimSpace(required.value) == "" {
			return "", "", "", fmt.Errorf("enroll: %s is required", required.name)
		}
	}
	if original, err = filepath.Abs(opts.Original); err != nil {
		return "", "", "", err
	}
	if workingCopy, err = filepath.Abs(opts.WorkingCopy); err != nil {
		return "", "", "", err
	}
	if exchange, err = filepath.Abs(opts.Exchange); err != nil {
		return "", "", "", err
	}
	if info, statErr := os.Stat(InternalDir(original)); statErr != nil || !info.IsDir() {
		return "", "", "", fmt.Errorf("enroll: no tt workspace at %s", original)
	}
	if err := ValidateWorkspaceMovePaths(original, workingCopy); err != nil {
		return "", "", "", fmt.Errorf("enroll: %w", err)
	}
	if InICloudDrive(workingCopy, opts.MachineHome) {
		return "", "", "", fmt.Errorf("enroll: the working copy %s is inside iCloud Drive; choose a local folder, and keep only the exchange folder in iCloud", workingCopy)
	}
	if err := validateExchangePlacement(workingCopy, original, exchange); err != nil {
		return "", "", "", err
	}
	return original, workingCopy, exchange, nil
}

// validateExchangePlacement keeps the exchange folder apart from both
// workspaces: inside one of them it would be read as workspace content, and
// containing one it would expose live files to the exchange.
func validateExchangePlacement(workingCopy, original, exchange string) error {
	for _, workspace := range []string{workingCopy, original} {
		if workspace == "" {
			continue
		}
		if isSubpath(workspace, exchange) || isSubpath(exchange, workspace) {
			return fmt.Errorf("enroll: the exchange folder %s overlaps the workspace %s", exchange, workspace)
		}
	}
	return nil
}

// InICloudDrive reports whether path is inside this machine's iCloud Drive,
// resolving symbolic links where the path exists.
func InICloudDrive(path, machineHome string) bool {
	home, ok := resolveMachineHome(machineHome)
	if !ok {
		return false
	}
	drive := filepath.Join(home, "Library", "Mobile Documents")
	candidates := []string{filepath.Clean(path)}
	if resolved, err := filepath.EvalSymlinks(nearestExisting(path)); err == nil {
		candidates = append(candidates, resolved)
	}
	resolvedDrive, err := filepath.EvalSymlinks(drive)
	for _, candidate := range candidates {
		if isSubpath(drive, candidate) || (err == nil && isSubpath(resolvedDrive, candidate)) {
			return true
		}
	}
	return false
}

// nearestExisting is path, or its closest ancestor that exists.
func nearestExisting(path string) string {
	current := filepath.Clean(path)
	for {
		if _, err := os.Lstat(current); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return current
		}
		current = parent
	}
}

// readableEnrollmentRecord is the enrollment record at workingCopy, or nil
// when there is none or it cannot be read: resumeEnrollment leaves such a path
// to requireEmptyWorkingCopy rather than failing on it.
func readableEnrollmentRecord(workingCopy string) *EnrollmentRecord {
	record, err := LoadEnrollmentRecord(workingCopy)
	if err != nil {
		return nil
	}
	return record
}

// resumeEnrollment recognizes a working copy an interrupted enrollment of this
// same original already published, and reports it complete after selecting it
// again. Anything else at that path is left to requireEmptyWorkingCopy.
func resumeEnrollment(workingCopy, workspaceID, original string) (*EnrollReport, error) {
	record := readableEnrollmentRecord(workingCopy)
	if record == nil {
		return nil, nil
	}
	if record.WorkspaceID != workspaceID || record.Original != original {
		return nil, fmt.Errorf("enroll: %s is the working copy of another enrollment (%s from %s)", workingCopy, record.WorkspaceID, record.Original)
	}
	catalog, err := NewDomainFileStore(workingCopy).LoadCatalogReadOnly()
	if err != nil {
		return nil, fmt.Errorf("enroll: the published working copy %s does not load: %w", workingCopy, err)
	}
	if !catalog.Enrolled() {
		return nil, fmt.Errorf("enroll: %s has an enrollment record but its catalog is not enrolled", workingCopy)
	}
	if err := WriteActiveWorkspacePointer(workingCopy, workspaceID); err != nil {
		return nil, fmt.Errorf("enroll: selecting the working copy: %w", err)
	}
	return &EnrollReport{
		WorkspaceID: workspaceID,
		Original:    original,
		WorkingCopy: workingCopy,
		Snapshot:    record.Snapshot,
		Resumed:     true,
	}, nil
}

// requireEmptyWorkingCopy refuses a working-copy path that holds anything: an
// enrollment never writes over a directory it did not create.
func requireEmptyWorkingCopy(workingCopy string) error {
	entries, err := os.ReadDir(workingCopy)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("enroll: inspecting the working copy path %s: %w", workingCopy, err)
	case len(entries) > 0:
		return fmt.Errorf("enroll: the working copy path %s is not empty", workingCopy)
	}
	return nil
}

// snapshotOriginal copies every durable file of the original, verified by
// hash, into a new snapshot directory under root, and records what it holds
// last. The directory is assembled under a partial name and renamed into place
// once complete, so a snapshot that exists is a whole one; partials an earlier
// interrupted run left behind are removed first.
func snapshotOriginal(entries []workspaceEntry, original, root string, at time.Time) (string, error) {
	if err := os.MkdirAll(root, DirPermissions); err != nil {
		return "", fmt.Errorf("enroll: creating the snapshot root %s: %w", root, err)
	}
	stale, err := filepath.Glob(filepath.Join(root, "*"+snapshotPartialExt))
	if err != nil {
		return "", err
	}
	for _, partial := range stale {
		if removeErr := os.RemoveAll(partial); removeErr != nil {
			return "", fmt.Errorf("enroll: removing an interrupted snapshot %s: %w", partial, removeErr)
		}
	}
	// A snapshot is never replaced: an earlier run's complete snapshot of the
	// same instant keeps its name and this one takes the next.
	stamp := at.UTC().Format("20060102T150405.000000000Z")
	final := filepath.Join(root, stamp)
	for n := 2; ; n++ {
		if _, statErr := os.Lstat(final); errors.Is(statErr, fs.ErrNotExist) {
			break
		}
		final = filepath.Join(root, fmt.Sprintf("%s-%d", stamp, n))
	}
	partial := final + snapshotPartialExt
	report := &MoveReport{}
	if dirsErr := createDestinationDirs(entries, partial, report); dirsErr != nil {
		return "", dirsErr
	}
	if copyErr := copyDestinationFiles(entries, partial, report); copyErr != nil {
		return "", copyErr
	}
	type snapshotFile struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	manifest := struct {
		Format        string         `json:"format"`
		FormatVersion int            `json:"format_version"`
		Source        string         `json:"source"`
		CreatedAt     time.Time      `json:"created_at"`
		Files         []snapshotFile `json:"files"`
	}{Format: enrollmentSnapshotFormat, FormatVersion: enrollmentFormatVersion, Source: original, CreatedAt: at.UTC(), Files: []snapshotFile{}}
	for _, entry := range entries {
		if !entry.dir {
			manifest.Files = append(manifest.Files, snapshotFile{Path: filepath.ToSlash(entry.rel), SHA256: entry.hash})
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := util.AtomicWriteFile(filepath.Join(partial, snapshotManifest), append(data, '\n'), FilePermissions); err != nil {
		return "", fmt.Errorf("enroll: recording the snapshot: %w", err)
	}
	if err := os.Rename(partial, final); err != nil {
		return "", fmt.Errorf("enroll: publishing the snapshot: %w", err)
	}
	return final, nil
}

// workingCopyExcluded is what enrollment leaves behind in the original: the
// device-local history, undo ledger, journal, and migration snapshots, which
// describe the original's past rather than the enrolled workspace's, and the
// lock files and sidecars no workspace carries across a copy. Conflict bundles
// stay with the workspace whose files they are about.
func workingCopyExcluded(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) >= 2 && parts[0] == dataDirName {
		switch parts[1] {
		case "sync", "undo", "transactions", "backups":
			return true
		}
	}
	base := parts[len(parts)-1]
	return strings.HasSuffix(base, ".lock") || strings.HasSuffix(base, ".bak") || strings.HasPrefix(base, ".tt-stage-")
}

// stageWorkingCopy copies the original's durable files into a fresh staging
// directory beside the working copy, marked as enrollment's own.
func stageWorkingCopy(entries []workspaceEntry, workingCopy string) (string, int, error) {
	staging := workingCopy + enrollStagingSuffix
	if _, err := os.Lstat(staging); err == nil {
		if _, markerErr := os.Stat(filepath.Join(staging, enrollStagingMarker)); markerErr != nil {
			return "", 0, fmt.Errorf("enroll: %s exists and is not an interrupted enrollment's staging directory; remove it or choose another working copy", staging)
		}
		if err := os.RemoveAll(staging); err != nil {
			return "", 0, fmt.Errorf("enroll: removing an interrupted enrollment's staging directory: %w", err)
		}
	}
	if err := os.MkdirAll(staging, DirPermissions); err != nil {
		return "", 0, fmt.Errorf("enroll: creating the staging directory: %w", err)
	}
	if err := util.AtomicWriteFile(filepath.Join(staging, enrollStagingMarker), []byte("tt enrollment staging\n"), FilePermissions); err != nil {
		return "", 0, fmt.Errorf("enroll: marking the staging directory: %w", err)
	}
	kept := make([]workspaceEntry, 0, len(entries))
	for _, entry := range entries {
		if !workingCopyExcluded(entry.rel) {
			kept = append(kept, entry)
		}
	}
	report := &MoveReport{}
	if err := createDestinationDirs(kept, staging, report); err != nil {
		return "", 0, err
	}
	if err := copyDestinationFiles(kept, staging, report); err != nil {
		return "", 0, err
	}
	return staging, report.FilesCopied, nil
}

// enrollStagedCatalog advances the staged catalog to the enrolled schema with
// its sync block, through the checked domain store, and proves the staged
// domains load as the enrolled workspace.
func enrollStagedCatalog(staging string, enrolledAt time.Time) error {
	store := NewDomainFileStore(staging)
	catalog, rev, err := store.LoadCatalogChecked()
	if err != nil {
		return fmt.Errorf("enroll: reading the staged catalog: %w", err)
	}
	catalog.SchemaVersion = SyncEnrolledConfigVersion
	catalog.Sync = &CatalogSync{EnrolledAt: enrolledAt, Exchange: SyncExchangeFormat}
	if err := store.SaveCatalogChecked(catalog, rev); err != nil {
		return fmt.Errorf("enroll: writing the enrolled catalog: %w", err)
	}
	// The checked save leaves a sidecar of the original catalog; the working
	// copy starts without one, exactly as a copy excludes the original's.
	if err := os.Remove(WorkspaceCatalogPath(staging) + ".bak"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("enroll: removing the staged catalog sidecar: %w", err)
	}
	if _, err := NewDomainFileStore(staging).LoadCatalogReadOnly(); err != nil {
		return fmt.Errorf("enroll: the enrolled catalog does not load: %w", err)
	}
	if _, err := NewDomainFileStore(staging).LoadPlanning(); err != nil {
		return fmt.Errorf("enroll: the staged planning does not load: %w", err)
	}
	return nil
}

// publishWorkingCopy makes the staged working copy the working copy in one
// rename, after dropping the staging marker and an empty placeholder directory
// the user may have created for it.
func publishWorkingCopy(staging, workingCopy string) error {
	if err := os.Remove(filepath.Join(staging, enrollStagingMarker)); err != nil {
		return fmt.Errorf("enroll: unmarking the staging directory: %w", err)
	}
	if err := os.Remove(workingCopy); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("enroll: the working copy path %s is not empty: %w", workingCopy, err)
	}
	if err := os.MkdirAll(filepath.Dir(workingCopy), DirPermissions); err != nil {
		return fmt.Errorf("enroll: creating the working copy's parent: %w", err)
	}
	if err := os.Rename(staging, workingCopy); err != nil {
		return fmt.Errorf("enroll: publishing the working copy: %w", err)
	}
	return nil
}

// PrepareJoinedWorkingCopy checks and creates the directory a joining device
// builds its working copy in: absolute, empty or absent, outside iCloud Drive,
// and apart from the exchange folder.
func PrepareJoinedWorkingCopy(dataDir, exchange, machineHome string) (string, error) {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(exchange) == "" {
		return "", fmt.Errorf("join: the working copy and the exchange folder are required")
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	exchangeAbs, err := filepath.Abs(exchange)
	if err != nil {
		return "", err
	}
	if InICloudDrive(abs, machineHome) {
		return "", fmt.Errorf("join: the working copy %s is inside iCloud Drive; choose a local folder", abs)
	}
	if err := validateExchangePlacement(abs, "", exchangeAbs); err != nil {
		return "", err
	}
	if err := requireEmptyWorkingCopy(abs); err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, DirPermissions); err != nil {
		return "", fmt.Errorf("join: creating the working copy: %w", err)
	}
	return abs, nil
}

// RecordJoinedWorkingCopy records, for a device that joined rather than
// enrolled, the exchange folder it uses, and selects its working copy. The
// working copy must already hold the enrolled catalog its history delivered.
func RecordJoinedWorkingCopy(dataDir, workspaceID, exchange string) error {
	if err := RecordJoinedEnrollment(dataDir, workspaceID, exchange); err != nil {
		return err
	}
	return WriteActiveWorkspacePointer(dataDir, workspaceID)
}

// RecordJoinedEnrollment records the exchange folder a joined working copy
// uses and selects nothing: a mobile app keeps its own workspace selection, so
// it never writes the desktop's active-workspace pointer.
func RecordJoinedEnrollment(dataDir, workspaceID, exchange string) error {
	catalog, err := NewDomainFileStore(dataDir).LoadCatalogReadOnly()
	if err != nil {
		return fmt.Errorf("join: the exchange has not delivered the workspace yet: %w", err)
	}
	if !catalog.Enrolled() || catalog.WorkspaceID != workspaceID {
		return fmt.Errorf("join: the delivered workspace is not the enrolled workspace %s", workspaceID)
	}
	exchangeAbs, err := filepath.Abs(exchange)
	if err != nil {
		return err
	}
	record := &EnrollmentRecord{
		Format:         EnrollmentRecordFormat,
		FormatVersion:  enrollmentFormatVersion,
		WorkspaceID:    workspaceID,
		ExchangeFolder: exchangeAbs,
		EnrolledAt:     catalog.Sync.EnrolledAt,
	}
	return writeEnrollmentRecord(dataDir, record)
}
