package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// MoveReport summarizes the outcome of a workspace move so the caller can print
// an accurate, non-destructive summary.
type MoveReport struct {
	Source         string
	Destination    string
	FilesCopied    int
	FilesSkipped   int // already present at the destination and byte-identical
	DirsCreated    int
	SourceRetained bool
}

// workspaceEntry is one durable source path collected during inventory.
type workspaceEntry struct {
	src  string
	rel  string
	dir  bool
	perm fs.FileMode
	hash string
}

type workspaceMoveHooks interface {
	beforeWorkspaceLocksAcquired()
	didInventoryWorkspace()
	didPrecheckDestination()
	didCopyWorkspace()
}

// MoveWorkspace relocates a workspace's durable data directory from srcDataDir
// to dstDataDir using a staged, verified, resumable flow:
//
//  1. Exclude active TUIs from both paths, then acquire and recover both
//     workspaces under deterministic commit-lock ordering.
//  2. Inventory the durable source tree, excluding machine-local runtime state
//     and refusing symlinks / non-regular files.
//  3. Pre-check every destination file: absent, or byte-identical (an
//     interrupted move re-run is idempotent). A divergent existing file aborts
//     the whole move before anything is written.
//  4. Bind the catalog identity and import any former path-local machine state
//     before the data-directory path changes.
//  5. Copy each missing file through a temp file with fsync and a sha256 read-
//     back verification, publishing without replacement.
//  6. Revalidate source hashes and destination occupancy.
//  7. Switch the machine-local active-workspace pointer to the destination
//     (the atomic "the workspace now lives here" commit).
//
// The source is retained by default (SourceRetained is always true here); it is
// never deleted, so the operation is safe to interrupt and re-run. Callers that
// want to reclaim the old space remove the source themselves after verifying.
func MoveWorkspace(srcDataDir, dstDataDir string) (*MoveReport, error) {
	return moveWorkspace(srcDataDir, dstDataDir, nil)
}

// ValidateWorkspaceMovePaths refuses a source/destination pair no move could
// ever accept — either path missing, the same directory, or one nested inside
// the other — without touching either tree. MoveWorkspace runs it first; a
// frontend runs it before asking for confirmation, so a pair that can only fail
// never asks.
func ValidateWorkspaceMovePaths(srcDataDir, dstDataDir string) error {
	src := filepath.Clean(srcDataDir)
	dst := filepath.Clean(dstDataDir)
	if src == "" || src == "." || dst == "" || dst == "." {
		return fmt.Errorf("workspace move requires both a source and a destination data dir")
	}
	if src == dst {
		return fmt.Errorf("workspace move: source and destination are the same directory: %s", src)
	}
	if err := ensureNotNested(src, dst); err != nil {
		return fmt.Errorf("workspace move aborted: %w", err)
	}
	return nil
}

func moveWorkspace(srcDataDir, dstDataDir string, hooks workspaceMoveHooks) (*MoveReport, error) {
	if err := ValidateWorkspaceMovePaths(srcDataDir, dstDataDir); err != nil {
		return nil, err
	}
	src := filepath.Clean(srcDataDir)
	dst := filepath.Clean(dstDataDir)

	srcInfo, statErr := os.Stat(src)
	if statErr != nil {
		return nil, fmt.Errorf("workspace move: source data dir %s is not accessible: %w", src, statErr)
	}
	if !srcInfo.IsDir() {
		return nil, fmt.Errorf("workspace move: source data dir %s is not a directory", src)
	}

	var report *MoveReport
	err := withWorkspaceTUILocks(src, dst, func() error {
		if hooks != nil {
			hooks.beforeWorkspaceLocksAcquired()
		}

		sourceCommits := storage.NewCommitService(TransactionsDir(src), []string{src})
		destinationCommits := storage.NewCommitService(TransactionsDir(dst), []string{dst})
		return storage.WithWorkspaceCommitLocks([]*storage.CommitService{sourceCommits, destinationCommits}, func() error {
			entries, invErr := inventoryWorkspace(src)
			if invErr != nil {
				return invErr
			}
			if hooks != nil {
				hooks.didInventoryWorkspace()
			}
			if err := precheckDestination(entries, dst); err != nil {
				return err
			}
			if hooks != nil {
				hooks.didPrecheckDestination()
			}
			if err := prepareWorkspaceMachineStateForMove(src); err != nil {
				return err
			}

			if err := os.MkdirAll(dst, util.DirPermissions); err != nil {
				return fmt.Errorf("workspace move: failed to create destination %s: %w", dst, err)
			}

			report = &MoveReport{Source: src, Destination: dst, SourceRetained: true}
			if err := createDestinationDirs(entries, dst, report); err != nil {
				return err
			}
			if err := copyDestinationFiles(entries, dst, report); err != nil {
				return err
			}
			if hooks != nil {
				hooks.didCopyWorkspace()
			}

			// A non-cooperating writer may bypass the workspace commit lock. Detect
			// that before publication by proving the copied source snapshot is still
			// current and every destination target still contains the expected bytes.
			if err := validateSourceSnapshot(entries); err != nil {
				return err
			}
			if err := precheckDestination(entries, dst); err != nil {
				return err
			}

			// Atomic pointer switch: from here on this machine opens the destination.
			if err := WriteActiveWorkspacePointer(dst, readWorkspaceID(dst)); err != nil {
				return fmt.Errorf("workspace move: failed to switch active workspace pointer: %w", err)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

type workspaceTUILockTarget struct {
	path string
	role string
}

// withWorkspaceTUILocks excludes live TUIs from both the retained source and
// the destination before any durable workspace locks are taken. Sorting gives
// opposite-direction moves one global acquisition order.
func withWorkspaceTUILocks(src, dst string, fn func() error) error {
	targets := []workspaceTUILockTarget{
		{path: filepath.Clean(TUIProcessLockPath(src)), role: "source"},
		{path: filepath.Clean(TUIProcessLockPath(dst)), role: "destination"},
	}
	sort.SliceStable(targets, func(i, j int) bool { return targets[i].path < targets[j].path })
	if targets[0].path == targets[1].path {
		targets = targets[:1]
		targets[0].role = "source/destination"
	}
	return acquireWorkspaceTUILocks(targets, fn)
}

func acquireWorkspaceTUILocks(targets []workspaceTUILockTarget, fn func() error) error {
	if len(targets) == 0 {
		return fn()
	}
	target := targets[0]
	if err := os.MkdirAll(filepath.Dir(target.path), util.DirPermissions); err != nil {
		return fmt.Errorf("workspace move: failed to create %s TUI runtime directory: %w", target.role, err)
	}
	lock, err := util.AcquirePIDLock(target.path)
	if err != nil {
		return fmt.Errorf("workspace move blocked by %s TUI safety lock: %w; close the active tt TUI using this workspace and retry", target.role, err)
	}
	defer lock.Release()
	return acquireWorkspaceTUILocks(targets[1:], fn)
}

// prepareWorkspaceMachineStateForMove resolves an existing catalog before the
// source data-directory path changes. This gives the machine store one chance
// to import the source's former path-derived file into the stable workspace-ID
// location. Workspaces without split-domain state are left unchanged.
func prepareWorkspaceMachineStateForMove(dataDir string) error {
	store := NewDomainFileStore(dataDir)
	_, catalogRev, err := store.LoadCatalogChecked()
	if err != nil {
		return fmt.Errorf("workspace move: failed to load workspace identity: %w", err)
	}
	if catalogRev == "" {
		return nil
	}
	if _, err := store.LoadMachine(); err != nil {
		return fmt.Errorf("workspace move: failed to bind machine state before moving: %w", err)
	}
	return nil
}

// inventoryWorkspace walks a data directory and returns its durable entries.
// Machine-local runtime state is excluded; symlinks and other non-regular files
// are refused so a move never follows a link out of the workspace root or
// silently drops special files.
func inventoryWorkspace(root string) ([]workspaceEntry, error) {
	var entries []workspaceEntry
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if isWorkspaceMoveExcluded(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("workspace move aborted: source contains a symlink, resolve it before moving: %s", path)
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("workspace move aborted: source contains a non-regular file, resolve it before moving: %s", path)
		}
		entry := workspaceEntry{src: path, rel: rel, dir: d.IsDir(), perm: info.Mode().Perm()}
		if !entry.dir {
			entry.hash, infoErr = hashFile(path)
			if infoErr != nil {
				return fmt.Errorf("failed to hash source file %s: %w", path, infoErr)
			}
		}
		entries = append(entries, entry)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("workspace move: failed to inventory source %s: %w", root, walkErr)
	}
	return entries, nil
}

func isWorkspaceMoveExcluded(rel string) bool {
	if isMigrationRootExcluded(rel) {
		return true
	}
	return filepath.Clean(rel) == filepath.Join(dataDirName, "transactions", "commit.lock")
}

// ensureNotNested refuses a source and destination that overlap, which a
// recursive copy could otherwise loop on or corrupt. Both the lexical paths and
// their symlink-resolved forms (where they exist) are checked. The wording
// names no command: a move and a backup each say which one refused.
func ensureNotNested(src, dst string) error {
	if isSubpath(src, dst) {
		return fmt.Errorf("destination %s is inside the source %s", dst, src)
	}
	if isSubpath(dst, src) {
		return fmt.Errorf("source %s is inside the destination %s", src, dst)
	}
	rs, err1 := filepath.EvalSymlinks(src)
	rd, err2 := filepath.EvalSymlinks(dst)
	if err1 == nil && err2 == nil && (isSubpath(rs, rd) || isSubpath(rd, rs)) {
		return fmt.Errorf("source and destination overlap after resolving symlinks (%s, %s)", rs, rd)
	}
	return nil
}

// isSubpath reports whether child is parent or lives beneath it.
func isSubpath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// readWorkspaceID best-effort reads the workspace ID from a data dir's synced
// workspace catalog so the active-workspace pointer can carry it. A missing or
// unreadable catalog yields an empty ID (the pointer's authoritative field is
// its data dir).
func readWorkspaceID(dataDir string) string {
	data, err := os.ReadFile(WorkspaceCatalogPath(dataDir))
	if err != nil {
		return ""
	}
	var env struct {
		WorkspaceID string `json:"workspace_id"`
		Data        struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &env) != nil {
		return ""
	}
	if env.WorkspaceID != "" {
		return env.WorkspaceID
	}
	return env.Data.WorkspaceID
}

// StorageLocationLabel returns a human-readable label for where a data dir
// lives: an iCloud Drive path is labeled as synced, everything else as local.
func StorageLocationLabel(dataDir string) string {
	if root, ok := iCloudDataRoot(); ok && isSubpath(root, filepath.Clean(dataDir)) {
		return "iCloud Drive (synced)"
	}
	return "local"
}
