package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
	"github.com/sglavoie/docket/internal/util"
)

// V24ConfigSchemaVersion is the first schema whose internal directory is
// .docket/ rather than .tt/. It changes the layout alone: no task grammar,
// catalog field, or other payload changes at this boundary.
const V24ConfigSchemaVersion = 24

const (
	// legacyInternalDirName is the internal directory every workspace kept
	// before schema 24. After the boundary it holds only the tombstone.
	legacyInternalDirName = ".tt"

	// v24StageDirName is where the new internal directory is assembled. It
	// becomes .docket/ in one rename, which is the boundary's commit point, so
	// a stage left by a crash is never authoritative and is removed on sight.
	v24StageDirName = ".docket-v24-stage"

	// v24TombstoneStageDirName is where the tombstone is assembled before it
	// replaces the retired .tt/ in one rename.
	v24TombstoneStageDirName = ".tt-v24-tombstone-stage"

	// v24RetiredDirName is the verified pre-v24 .tt/ between the moment it is
	// renamed aside and the moment it is deleted. Only an activation that has
	// already proven .docket/ holds its bytes produces it.
	v24RetiredDirName = ".tt-v24-retired"

	// v24KeptDirPrefix names a live .tt/ found beside .docket/ holding bytes
	// .docket/ does not, kept under backups/. It is not a snapshot prefix, so
	// the snapshot diagnostic does not read it as a malformed pair.
	v24KeptDirPrefix = "kept-pre-v24-internal-"

	// v24ActivationLockName is the lock inside .tt/ that one relocating process
	// holds from before its snapshot until the tombstone is in place.
	v24ActivationLockName = "v24-activation.lock"

	// v24TombstoneSchema is the catalog schema the tombstone declares. It is
	// fixed: a tombstone only has to be newer than every Docket that still
	// reads .tt/, and those all stop at schema 23.
	v24TombstoneSchema = V24ConfigSchemaVersion

	// v24TombstoneNote is the planning.json body of the tombstone. Older
	// builds refuse the catalog before they read it; it is there for a person.
	v24TombstoneNote = `{"tombstone":"Docket schema 24 moved this workspace's internal state to .docket/. ` +
		`This directory only makes older versions refuse the workspace instead of creating an empty one; ` +
		`do not delete it."}` + "\n"
)

// LegacyInternalDir returns the pre-v24 internal directory inside a data
// directory: the .tt/ the v24 activation moves from and then replaces with the
// tombstone that makes every older build refuse the workspace.
func LegacyInternalDir(dataDir string) string {
	return filepath.Join(dataDir, legacyInternalDirName)
}

// WorkspaceDirMarkers returns the internal directories whose presence marks a
// workspace at dataDir: .docket/, or the .tt/ of a workspace that predates
// schema 24 (or the tombstone left where one was). Opening the latter is what
// migrates or refuses it, so discovery must not mistake it for no workspace.
func WorkspaceDirMarkers(dataDir string) []string {
	return []string{InternalDir(dataDir), LegacyInternalDir(dataDir)}
}

// WorkspaceMovedElsewhereError reports a workspace whose .tt/ is the schema-24
// tombstone while .docket/ is absent: another machine moved the workspace's
// internal state, and the move has not arrived here yet.
type WorkspaceMovedElsewhereError struct {
	DataDir string
}

func (e *WorkspaceMovedElsewhereError) Error() string {
	return fmt.Sprintf(
		"workspace %s was moved to %s by a newer Docket on another machine, but %s is not here yet; "+
			"wait for it to finish syncing and open the workspace again (%s is only a tombstone)",
		e.DataDir, InternalDir(e.DataDir), InternalDir(e.DataDir), LegacyInternalDir(e.DataDir),
	)
}

type v24LegacyState int

const (
	v24LegacyAbsent v24LegacyState = iota
	v24LegacyTombstone
	v24LegacyLive
)

// inspectV24Legacy classifies dataDir/.tt: absent, the tombstone (a catalog at
// schema 24 or later), or live pre-v24 state that still has to move. A .tt/
// without a readable catalog is live: a workspace from before the split
// domains, or the empty directory that marks where one belongs.
func inspectV24Legacy(dataDir string) (v24LegacyState, error) {
	legacy := LegacyInternalDir(dataDir)
	info, err := os.Lstat(legacy)
	if errors.Is(err, fs.ErrNotExist) {
		return v24LegacyAbsent, nil
	}
	if err != nil {
		return 0, fmt.Errorf("inspect %s: %w", legacy, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return 0, fmt.Errorf("pre-v24 internal directory %s is not a directory", legacy)
	}
	schema, exists, err := catalogPayloadSchemaAt(filepath.Join(legacy, "workspace.json"))
	if err == nil && exists && schema >= v24TombstoneSchema {
		return v24LegacyTombstone, nil
	}
	return v24LegacyLive, nil
}

// legacyWorkspaceCatalogSchema is WorkspaceCatalogSchema for a data directory
// with no .docket/ catalog: the schema a pre-v24 .tt/ catalog declares, which
// is older than this build and so pending migration, or the refusal a lone
// tombstone earns.
func legacyWorkspaceCatalogSchema(dataDir string) (int, bool, error) {
	state, err := inspectV24Legacy(dataDir)
	if err != nil {
		return 0, false, err
	}
	switch state {
	case v24LegacyTombstone:
		if !dirExists(InternalDir(dataDir)) {
			return 0, false, &WorkspaceMovedElsewhereError{DataDir: dataDir}
		}
		return 0, false, nil
	case v24LegacyLive:
		schema, exists, schemaErr := catalogPayloadSchemaAt(filepath.Join(LegacyInternalDir(dataDir), "workspace.json"))
		if schemaErr != nil || !exists {
			return 0, false, schemaErr
		}
		return schema, true, nil
	default:
		return 0, false, nil
	}
}

// pendingV24LayoutError refuses a read that would otherwise see no workspace
// where a pre-v24 one is still waiting to move, or where only the tombstone
// arrived. Only an opening that migrates may proceed past either.
func pendingV24LayoutError(dataDir string) error {
	if dirExists(InternalDir(dataDir)) {
		return nil
	}
	state, err := inspectV24Legacy(dataDir)
	if err != nil {
		return err
	}
	switch state {
	case v24LegacyTombstone:
		return &WorkspaceMovedElsewhereError{DataDir: dataDir}
	case v24LegacyLive:
		return fmt.Errorf("workspace %s still keeps its internal state in %s; open it with Docket once so it migrates to schema %d",
			dataDir, LegacyInternalDir(dataDir), V24ConfigSchemaVersion)
	default:
		return nil
	}
}

func dirExists(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}

func statIsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// v24HoldsBytes reports whether path holds exactly the bytes source was
// fingerprinted with; an unreadable path holds nothing.
func v24HoldsBytes(path string, source preScopeV7Source) bool {
	current, err := readPreScopeV7Source(path, source.Path, source.Kind, false)
	return err == nil && current.SHA256 == source.SHA256
}

type v24LayoutActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
	// interrupt is consulted after each durable step of the activation. A
	// non-nil error stops the activation right there, as a crash would.
	interrupt(step v24LayoutStep) error
}

type v24LayoutStep string

const (
	v24StepStaged          v24LayoutStep = "staged"
	v24StepActivated       v24LayoutStep = "activated"
	v24StepLegacyRetired   v24LayoutStep = "legacy-retired"
	v24StepTombstonePlaced v24LayoutStep = "tombstone-placed"
)

func (s v24LayoutStep) interruptWith(hooks v24LayoutActivationHooks) error {
	if hooks == nil {
		return nil
	}
	return hooks.interrupt(s)
}

// ActivateV24Layout moves a pre-v24 workspace's internal .tt/ to .docket/, or
// finishes a move a crash interrupted. It runs before anything else reads the
// workspace, because every reader now looks under .docket/. The sequence is the
// chain's: validate the candidate, publish an immutable archive/audit pair into
// .tt/backups/, then activate.
//
// Activation assembles .docket/ in a stage beside the workspace, verifies the
// stage holds every byte the archive does, and renames it into place. That
// rename is the commit point: before it the stage is discarded and .tt/ is
// untouched; after it the move is finished forward. Finishing replaces .tt/
// with a tombstone — a catalog declaring schema 24 beside a planning file and
// the commit lock — so a Docket that still reads .tt/ refuses the workspace as
// newer than it supports, byte-identically, instead of finding nothing there and
// creating an empty workspace over synced data.
//
// The catalog keeps its schema here. Whatever schema it holds is migrated by the
// chain inside .docket/, and MigrateWorkspaceToV24 advances it to 24 last.
func (m *Manager) ActivateV24Layout() error {
	return m.activateV24Layout(nil)
}

func (m *Manager) activateV24Layout(hooks v24LayoutActivationHooks) error {
	if m == nil {
		return fmt.Errorf("configuration manager is required")
	}
	if !statIsDir(m.workspaceDataDir()) {
		// Nothing to move; the load path names a data directory that is missing
		// or is not a directory.
		return nil
	}
	dataDir, err := validateV5DryRunRoot(m.workspaceDataDir())
	if err != nil {
		return err
	}
	state, err := inspectV24Legacy(dataDir)
	if err != nil {
		return err
	}
	if dirExists(InternalDir(dataDir)) {
		return lockedSettleV24Layout(dataDir, state)
	}
	switch state {
	case v24LegacyLive:
		return m.relocateV24Layout(dataDir, hooks)
	case v24LegacyTombstone:
		return &WorkspaceMovedElsewhereError{DataDir: dataDir}
	default:
		return nil
	}
}

// withV24DocketLock runs fn holding .docket/'s workspace commit lock, which is
// what serializes every write to the tombstone and the retired directory: the
// relocating process holds the same lock from before its commit point, so no
// other process settles a move that is still finishing.
func withV24DocketLock(dataDir string, fn func() error) error {
	if err := os.MkdirAll(TransactionsDir(dataDir), DirPermissions); err != nil {
		return fmt.Errorf("prepare %s: %w", TransactionsDir(dataDir), err)
	}
	return util.WithFileLock(filepath.Join(TransactionsDir(dataDir), storage.WorkspaceCommitLockName), true, fn)
}

// lockedSettleV24Layout is settleV24Layout under the .docket/ lock, re-reading
// the legacy state there. A finished workspace with nothing to settle takes no
// lock at all, which is every opening after the first.
func lockedSettleV24Layout(dataDir string, state v24LegacyState) error {
	if state == v24LegacyTombstone && !dirExists(filepath.Join(dataDir, v24RetiredDirName)) {
		return nil
	}
	return withV24DocketLock(dataDir, func() error {
		current, err := inspectV24Legacy(dataDir)
		if err != nil {
			return err
		}
		return settleV24Layout(dataDir, current)
	})
}

// settleV24Layout finishes a workspace whose .docket/ already exists. A missing
// tombstone is written; a retired .tt/ this machine verified and renamed aside
// is deleted. A live .tt/ beside .docket/ is either a crash after the commit
// point, in which case .docket/ still holds every byte it does and it is
// retired like any verified original, or a .tt/ an older build kept writing
// while the move synced from another machine: that one is kept, whole, under
// .docket/backups/ rather than deleted, and the tombstone takes its place.
func settleV24Layout(dataDir string, state v24LegacyState) error {
	retired := filepath.Join(dataDir, v24RetiredDirName)
	switch state {
	case v24LegacyTombstone:
		return removeV24Retired(retired)
	case v24LegacyAbsent:
		if !fileExists(WorkspaceCatalogPath(dataDir)) {
			// A .docket/ with no catalog yet is a workspace being created; the
			// bootstrap writes the tombstone once there is an identity to give it.
			return nil
		}
		if err := placeV24Tombstone(dataDir); err != nil {
			return err
		}
		return removeV24Retired(retired)
	default:
		if !fileExists(WorkspaceCatalogPath(dataDir)) {
			return fmt.Errorf(
				"workspace %s holds both a pre-v24 %s and a %s with no catalog; move %s aside and open the workspace again",
				dataDir, LegacyInternalDir(dataDir), InternalDir(dataDir), InternalDir(dataDir),
			)
		}
		carried, err := v24LegacyCarried(dataDir)
		if err != nil {
			return err
		}
		aside := retired
		if !carried {
			aside = filepath.Join(PreScopeV7BackupsDir(dataDir),
				v24KeptDirPrefix+time.Now().UTC().Format(preScopeV7Timestamp))
			if err = os.MkdirAll(filepath.Dir(aside), DirPermissions); err != nil {
				return fmt.Errorf("prepare %s: %w", filepath.Dir(aside), err)
			}
		}
		if err = os.Rename(LegacyInternalDir(dataDir), aside); err != nil {
			return fmt.Errorf("move the pre-v24 %s aside to %s: %w", LegacyInternalDir(dataDir), aside, err)
		}
		if err = placeV24Tombstone(dataDir); err != nil {
			return err
		}
		return removeV24Retired(retired)
	}
}

// v24LegacyCarried reports whether .docket/ still holds every byte the move
// carries out of a live .tt/, which is what an activation interrupted after its
// commit point leaves. That includes backups/, which the inventory leaves out:
// a backup an older build or a sync added to .tt/ afterwards exists nowhere
// else, so it keeps .tt/ from being discarded.
func v24LegacyCarried(dataDir string) (bool, error) {
	inventory, err := inventoryV24Legacy(dataDir, false)
	if err != nil {
		return false, err
	}
	legacy := LegacyInternalDir(dataDir)
	for _, file := range inventory.files {
		rel, relErr := filepath.Rel(legacy, file.sourcePath)
		if relErr != nil {
			return false, relErr
		}
		if v24LegacyDropped(rel) {
			continue
		}
		if !v24HoldsBytes(filepath.Join(InternalDir(dataDir), rel), file) {
			return false, nil
		}
	}
	return v24LegacyBackupsCarried(dataDir)
}

// v24LegacyBackupsCarried reports whether every backup under .tt/backups/ the
// move copies, which is all of them but lock files, sits byte for byte at the
// same place under .docket/. Anything it cannot read or compare counts as not
// carried, so the caller keeps .tt/ rather than deleting it.
func v24LegacyBackupsCarried(dataDir string) (bool, error) {
	legacy := LegacyInternalDir(dataDir)
	backups := filepath.Join(legacy, "backups")
	carried := true
	err := filepath.WalkDir(backups, func(path string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) && path == backups {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".lock") {
			return nil
		}
		if !entry.Type().IsRegular() {
			carried = false
			return fs.SkipAll
		}
		rel, err := filepath.Rel(legacy, path)
		if err != nil {
			return err
		}
		source, err := readPreScopeV7Source(path, filepath.ToSlash(rel), "pre-v24 backup", false)
		if err != nil {
			return err
		}
		if !v24HoldsBytes(filepath.Join(InternalDir(dataDir), rel), source) {
			carried = false
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("compare %s with %s: %w", backups, PreScopeV7BackupsDir(dataDir), err)
	}
	return carried, nil
}

func removeV24Retired(retired string) error {
	if err := os.RemoveAll(retired); err != nil {
		return fmt.Errorf("remove the retired pre-v24 internal directory %s: %w", retired, err)
	}
	return nil
}

// relocateV24Layout runs the move under an activation lock inside .tt/ that a
// second process opening the same workspace waits on, so only one of them
// snapshots and stages; the other then finds .docket/ and settles. A stage a
// crash left behind is discarded under that lock, never while another process
// may still be writing it.
func (m *Manager) relocateV24Layout(dataDir string, hooks v24LayoutActivationHooks) error {
	settle := false
	err := util.WithFileLock(filepath.Join(LegacyInternalDir(dataDir), v24ActivationLockName), true, func() error {
		if dirExists(InternalDir(dataDir)) {
			settle = true
			return nil
		}
		if state, inspectErr := inspectV24Legacy(dataDir); inspectErr != nil || state != v24LegacyLive {
			return inspectErr
		}
		if removeErr := os.RemoveAll(filepath.Join(dataDir, v24StageDirName)); removeErr != nil {
			return fmt.Errorf("remove the interrupted schema-24 stage: %w", removeErr)
		}
		return m.relocateV24LayoutLocked(dataDir, hooks)
	})
	if err != nil || !settle {
		return err
	}
	state, err := inspectV24Legacy(dataDir)
	if err != nil {
		return err
	}
	return lockedSettleV24Layout(dataDir, state)
}

func (m *Manager) relocateV24LayoutLocked(dataDir string, hooks v24LayoutActivationHooks) error {
	legacy := LegacyInternalDir(dataDir)
	legacyTransactions := filepath.Join(legacy, "transactions")
	roots := []string{dataDir, m.localDataRoot()}
	commit := storage.NewCommitService(legacyTransactions, roots).OnMachine(m.machineID())
	if _, err := commit.RecoverWorkspace(); err != nil {
		return fmt.Errorf("recover the pre-v24 workspace before schema-24 activation: %w", err)
	}
	source, err := catalogSchemaForV24Audit(dataDir)
	if err != nil {
		return fmt.Errorf("v24 migration validation failed; workspace was not changed: %w", err)
	}
	if _, err = inventoryV24Legacy(dataDir, false); err != nil {
		return fmt.Errorf("v24 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreDocketLayoutV24Snapshot(dataDir, source)
	if err != nil {
		return err
	}
	if hooks != nil {
		hooks.didPublishSnapshot(snapshot.archivePath, snapshot.auditPath)
	}
	moved := false
	lockErr := storage.WithWorkspaceCommitLocks([]*storage.CommitService{commit}, func() error {
		inventory, inspectErr := transaction.Inspect(legacyTransactions, roots, m.machineID())
		if inspectErr != nil {
			return inspectErr
		}
		if len(inventory.Pending) > 0 || len(inventory.Quarantined) > 0 {
			return fmt.Errorf(
				"%s holds %d unfinished and %d quarantined transaction(s); open the workspace on the machine that owns them, "+
					"or resolve the quarantine, before it can move to %s",
				legacyTransactions, len(inventory.Pending), len(inventory.Quarantined), InternalDir(dataDir),
			)
		}
		live, inventoryErr := inventoryV24Legacy(dataDir, false)
		if inventoryErr != nil {
			return inventoryErr
		}
		if compareErr := compareV24ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return compareErr
		}
		if stageErr := stageV24Layout(dataDir, live); stageErr != nil {
			return stageErr
		}
		if stepErr := v24StepStaged.interruptWith(hooks); stepErr != nil {
			return stepErr
		}
		// The stage's commit lock becomes .docket/'s in the rename, so holding
		// it from here keeps every settling process out until the move is done.
		stageLock := filepath.Join(dataDir, v24StageDirName, "transactions", storage.WorkspaceCommitLockName)
		return util.WithFileLock(stageLock, true, func() error {
			return activateV24Stage(dataDir, hooks, &moved)
		})
	})
	if lockErr != nil {
		if moved {
			return fmt.Errorf("v24 migration activated %s but did not finish replacing %s; opening the workspace again finishes it: %w",
				InternalDir(dataDir), legacy, lockErr)
		}
		return fmt.Errorf("v24 migration failed after creating rollback snapshot %s and audit %s: %w",
			snapshot.archivePath, snapshot.auditPath, lockErr)
	}
	return nil
}

// activateV24Stage renames the verified stage into place, which is the commit
// point, then retires .tt/ and puts the tombstone where it was.
func activateV24Stage(dataDir string, hooks v24LayoutActivationHooks, moved *bool) error {
	legacy := LegacyInternalDir(dataDir)
	if err := os.Rename(filepath.Join(dataDir, v24StageDirName), InternalDir(dataDir)); err != nil {
		return fmt.Errorf("activate %s: %w", InternalDir(dataDir), err)
	}
	if err := util.SyncDir(dataDir); err != nil {
		return err
	}
	*moved = true
	if err := v24StepActivated.interruptWith(hooks); err != nil {
		return err
	}
	retired := filepath.Join(dataDir, v24RetiredDirName)
	if err := os.Rename(legacy, retired); err != nil {
		return fmt.Errorf("retire %s: %w", legacy, err)
	}
	if err := v24StepLegacyRetired.interruptWith(hooks); err != nil {
		return err
	}
	if err := placeV24Tombstone(dataDir); err != nil {
		return err
	}
	if err := v24StepTombstonePlaced.interruptWith(hooks); err != nil {
		return err
	}
	return removeV24Retired(retired)
}

// catalogSchemaForV24Audit reads the schema the pre-v24 catalog declares, or 0
// for a .tt/ that holds no catalog yet.
func catalogSchemaForV24Audit(dataDir string) (int, error) {
	schema, exists, err := catalogPayloadSchemaAt(filepath.Join(LegacyInternalDir(dataDir), "workspace.json"))
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	if schema > V23ConfigSchemaVersion {
		return 0, fmt.Errorf("pre-v24 catalog declares schema %d, which only a tombstone may", schema)
	}
	return schema, nil
}

// v24LegacyExcluded reports the .tt/-relative paths that neither the archive
// nor the move carries: the transaction journal and its locks (recovery has
// emptied it, and a fresh one starts under .docket/), and lock files and
// interrupted temporaries anywhere. backups/ is excluded from the inventory but
// copied as it is, archives included, because nothing may make them mutable.
func v24LegacyExcluded(rel string) bool {
	top := strings.SplitN(rel, string(filepath.Separator), 2)[0]
	if top == "transactions" || top == "backups" {
		return true
	}
	base := filepath.Base(rel)
	return strings.HasSuffix(base, ".lock") || strings.Contains(base, ".tmp.")
}

// v24LegacyDropped reports the .tt/-relative paths the archive keeps but the
// move does not carry. Undo entries name the absolute paths they restore, all of
// them under .tt/, so a replay after the move would write into the tombstone.
func v24LegacyDropped(rel string) bool {
	return strings.SplitN(rel, string(filepath.Separator), 2)[0] == "undo"
}

// inventoryV24Legacy fingerprints every file under .tt/ the boundary archives.
// It refuses symlinks and other non-regular files instead of following or
// skipping them, so nothing the archive does not hold can ride the move.
func inventoryV24Legacy(dataDir string, captureData bool) (*preScopeV7Inventory, error) {
	legacy := LegacyInternalDir(dataDir)
	inventory := &preScopeV7Inventory{}
	err := filepath.WalkDir(legacy, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(legacy, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("v24 migration refuses symlink source: %s", path)
		}
		if v24LegacyExcluded(rel) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("v24 migration refuses non-regular source: %s", path)
		}
		source, err := readPreScopeV7Source(
			path,
			filepath.ToSlash(filepath.Join("workspace", legacyInternalDirName, rel)),
			"pre-v24 internal state",
			captureData,
		)
		if err != nil {
			return err
		}
		inventory.files = append(inventory.files, source)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory %s for the schema-24 move: %w", legacy, err)
	}
	sort.Slice(inventory.files, func(i, j int) bool { return inventory.files[i].Path < inventory.files[j].Path })
	return inventory, nil
}

// compareV24ArchivedInventory refuses activation whenever .tt/ no longer
// matches the exact bytes the rollback archive holds.
func compareV24ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v24 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf("v24 migration aborted: %s changed after the rollback snapshot was published", expected.sourcePath)
		}
	}
	return nil
}

// stageV24Layout assembles the new internal directory: every inventoried file
// but the undo ledger, byte for byte, plus backups/ as it stands, then proves
// the stage holds exactly the inventoried bytes before anything activates it.
func stageV24Layout(dataDir string, inventory *preScopeV7Inventory) error {
	legacy := LegacyInternalDir(dataDir)
	stage := filepath.Join(dataDir, v24StageDirName)
	if err := os.Mkdir(stage, DirPermissions); err != nil {
		return fmt.Errorf("create schema-24 stage %s: %w", stage, err)
	}
	// The journal does not move, but its directory does: the stage's commit
	// lock is what the activating process holds across the rename.
	stageTransactions := filepath.Join(stage, "transactions")
	if err := os.Mkdir(stageTransactions, DirPermissions); err != nil {
		return fmt.Errorf("create schema-24 stage %s: %w", stageTransactions, err)
	}
	dirs := map[string]bool{stage: true, stageTransactions: true}
	err := filepath.WalkDir(legacy, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(legacy, path)
		if err != nil || rel == "." {
			return err
		}
		top := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		carried := top == "backups" || (!v24LegacyExcluded(rel) && !v24LegacyDropped(rel))
		if top == "backups" && strings.HasSuffix(filepath.Base(rel), ".lock") {
			carried = false
		}
		if !carried {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("v24 migration refuses symlink source: %s", path)
		}
		target := filepath.Join(stage, rel)
		if entry.IsDir() {
			dirs[target] = true
			return os.Mkdir(target, DirPermissions)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("v24 migration refuses non-regular source: %s", path)
		}
		return copyV24File(path, target)
	})
	if err != nil {
		return fmt.Errorf("stage %s: %w", stage, err)
	}
	staged := make([]string, 0, len(dirs))
	for dir := range dirs {
		staged = append(staged, dir)
	}
	sort.Strings(staged)
	for _, dir := range staged {
		if syncErr := util.SyncDir(dir); syncErr != nil {
			return syncErr
		}
	}
	for _, file := range inventory.files {
		rel, relErr := filepath.Rel(legacy, file.sourcePath)
		if relErr != nil {
			return relErr
		}
		if v24LegacyDropped(rel) {
			continue
		}
		copied, readErr := readPreScopeV7Source(filepath.Join(stage, rel), file.Path, file.Kind, false)
		if readErr != nil {
			return fmt.Errorf("verify schema-24 stage: %w", readErr)
		}
		if copied.Size != file.Size || copied.SHA256 != file.SHA256 {
			return fmt.Errorf("verify schema-24 stage: %s does not match %s", copied.sourcePath, file.sourcePath)
		}
	}
	return nil
}

func copyV24File(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err = util.SyncFile(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// placeV24Tombstone writes the tombstone where .tt/ was, assembling it in a
// stage first so .tt/ is never a partial tombstone. Its catalog names the
// workspace's identity when .docket/ holds a catalog, because an older build
// validates that identity before it compares schemas.
func placeV24Tombstone(dataDir string) error {
	stage := filepath.Join(dataDir, v24TombstoneStageDirName)
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	workspaceID := v24TombstoneWorkspaceID(dataDir)
	catalog, err := json.MarshalIndent(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		Kind:          domainCatalog.name(),
		Data: json.RawMessage(fmt.Sprintf(`{"schema_version":%d,"workspace_id":%q}`,
			v24TombstoneSchema, workspaceID)),
	}, "", "  ")
	if err != nil {
		return err
	}
	transactions := filepath.Join(stage, "transactions")
	if err = os.MkdirAll(transactions, DirPermissions); err != nil {
		return fmt.Errorf("stage the schema-24 tombstone: %w", err)
	}
	for _, file := range []struct {
		path string
		data []byte
	}{
		{filepath.Join(stage, "workspace.json"), append(catalog, '\n')},
		{filepath.Join(stage, "planning.json"), []byte(v24TombstoneNote)},
		// An older build opens the commit lock before it reads the catalog; one
		// already here is what keeps its refusal byte-identical.
		{filepath.Join(transactions, storage.WorkspaceCommitLockName), nil},
	} {
		if err = util.AtomicWriteFile(file.path, file.data, FilePermissions); err != nil {
			return fmt.Errorf("stage the schema-24 tombstone: %w", err)
		}
	}
	for _, dir := range []string{transactions, stage} {
		if err = util.SyncDir(dir); err != nil {
			return err
		}
	}
	if err = os.Rename(stage, LegacyInternalDir(dataDir)); err != nil {
		return fmt.Errorf("place the schema-24 tombstone at %s: %w", LegacyInternalDir(dataDir), err)
	}
	return util.SyncDir(dataDir)
}

func v24TombstoneWorkspaceID(dataDir string) string {
	data, err := os.ReadFile(WorkspaceCatalogPath(dataDir))
	if err == nil {
		if env, decodeErr := decodeEnvelope(data); decodeErr == nil && validateWorkspaceID(env.WorkspaceID) == nil {
			return env.WorkspaceID
		}
	}
	return uuid.NewString()
}

// EnsureV24Tombstone writes the tombstone into a workspace that has a .docket/
// catalog and no .tt/ at all, which is what a workspace created at schema 24
// looks like before its first tombstone. Anything already at .tt/ is left alone.
func EnsureV24Tombstone(dataDir string) error {
	if _, err := os.Lstat(LegacyInternalDir(dataDir)); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if !fileExists(WorkspaceCatalogPath(dataDir)) {
		return nil
	}
	return withV24DocketLock(dataDir, func() error {
		// Another process may have placed it while this one waited.
		if _, err := os.Lstat(LegacyInternalDir(dataDir)); !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return placeV24Tombstone(dataDir)
	})
}

var errV24CatalogAlreadyAdvanced = errors.New("the catalog is already at schema 24")

type v24RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

func createPreDocketLayoutV24Snapshot(dataDir string, source int) (v24RollbackSnapshot, error) {
	inventory, err := inventoryV24Legacy(dataDir, true)
	if err != nil {
		return v24RollbackSnapshot{}, err
	}
	backups := filepath.Join(LegacyInternalDir(dataDir), "backups")
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preDocketLayoutV24SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(backups, stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(backups, stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v24RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v24LayoutAuditReport(createdAt, len(inventory.files), source))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preDocketLayoutV24SnapshotPrefix+"audit.md")
	if err != nil {
		return v24RollbackSnapshot{}, err
	}
	for index := range inventory.files {
		inventory.files[index].data = nil
	}
	absent := ""
	commit := storage.NewCommitService(filepath.Join(LegacyInternalDir(dataDir), "transactions"), []string{dataDir})
	if _, err = commit.CommitArtifacts([]storage.ArtifactChange{
		{Path: archivePath, Data: archive, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
		{Path: auditPath, Data: audit, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
	}); err != nil {
		return v24RollbackSnapshot{}, fmt.Errorf("pre-docket-layout v24 snapshot failed: %w", err)
	}
	return v24RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v24LayoutAuditReport(createdAt time.Time, files, source int) string {
	var report strings.Builder
	report.WriteString("# Pre-docket-layout schema-24 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	if source > 0 {
		fmt.Fprintf(&report, "- Source catalog schema: %d\n", source)
	} else {
		report.WriteString("- Source catalog schema: none (no catalog under `.tt/` yet)\n")
	}
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary changes the workspace layout and nothing else. The internal directory " +
		"`.tt/` becomes `.docket/`: the catalog, planning, lenses, global notes, conflict bundles, sync history, " +
		"and backups move byte for byte, and no task document, catalog field, or conflict bundle is rewritten. " +
		"Once the chain has migrated the catalog to schema 23 inside `.docket/`, its schema number advances to 24, " +
		"which is the only byte the catalog changes.\n\n")
	report.WriteString("`.tt/` is replaced by a tombstone: a catalog declaring schema 24, a planning file " +
		"explaining itself, and the commit lock. A Docket that still reads `.tt/` refuses the workspace as newer " +
		"than it supports, instead of finding nothing there and creating an empty workspace over synced data. " +
		"Do not delete it.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries name the absolute paths " +
		"they restore, every one of them under `.tt/`, so replaying one after the move would write into the " +
		"tombstone. The archive beside this report keeps the ledger. The transaction journal is not carried " +
		"either: the move only proceeds once recovery has left it empty, and a fresh one starts under `.docket/`.\n\n")
	report.WriteString("To roll back, restore `workspace/.tt/` from the archive beside this report over the " +
		"tombstone and remove `.docket/`; the archive holds every byte `.tt/` held apart from the journal and the " +
		"backups directory, which is carried as it is.\n")
	return report.String()
}

// MigrateWorkspaceToV24 advances a schema-23 catalog to 24 once the layout
// activation has moved it into .docket/ and the chain has brought it to 23.
// Only the schema number changes, so the catalog publishes in one transaction
// with its expected revision, and the undo ledger goes with it, as at every
// boundary that changes retained bytes. The rollback archive for this boundary
// is the one ActivateV24Layout published.
func (m *Manager) MigrateWorkspaceToV24() error {
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
		return fmt.Errorf("recover workspace before schema-24 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V23ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV23(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 23: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-23 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V24ConfigSchemaVersion {
		if schema <= SupportedConfigVersion {
			return EnsureV24Tombstone(dataDir)
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade Docket", schema, SupportedConfigVersion)
	}
	if schema != V23ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V24ConfigSchemaVersion)
	}
	plan := func() ([]transaction.Op, []storage.ArtifactChange, error) {
		// A concurrent opener may have advanced it while this one waited for
		// the commit lock; the plan reruns under that lock.
		if current, _, schemaErr := workspaceCatalogPayloadSchema(dataDir); schemaErr == nil && current == V24ConfigSchemaVersion {
			return nil, nil, errV24CatalogAlreadyAdvanced
		}
		catalogPath := WorkspaceCatalogPath(dataDir)
		sourceBytes, catalog, readErr := readMigrationSourceCatalog(catalogPath, V23ConfigSchemaVersion)
		if readErr != nil {
			return nil, nil, readErr
		}
		catalog.SchemaVersion = V24ConfigSchemaVersion
		published, encodeErr := encodeMigrationCatalogEnvelope(&catalog, V24ConfigSchemaVersion)
		if encodeErr != nil {
			return nil, nil, fmt.Errorf("encode schema-24 catalog: %w", encodeErr)
		}
		cleanup, cleanupErr := undoHistoryCleanupOps(UndoLedgerDir(dataDir), V24ConfigSchemaVersion)
		if cleanupErr != nil {
			return nil, nil, cleanupErr
		}
		revision := storage.RawArtifactRevision(sourceBytes)
		return cleanup, []storage.ArtifactChange{{
			Path: catalogPath, Data: published, Mode: FilePermissions, ExpectedRevision: &revision,
		}}, nil
	}
	if _, _, err = plan(); errors.Is(err, errV24CatalogAlreadyAdvanced) {
		return EnsureV24Tombstone(dataDir)
	} else if err != nil {
		return fmt.Errorf("v24 migration validation failed; workspace was not changed: %w", err)
	}
	if err = commit.CommitPlannedOpsWithArtifacts(plan); err != nil && !errors.Is(err, errV24CatalogAlreadyAdvanced) {
		return fmt.Errorf("v24 migration failed to advance the catalog to schema %d: %w", V24ConfigSchemaVersion, err)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V24ConfigSchemaVersion {
		return fmt.Errorf("v24 migration did not publish schema %d: %w", V24ConfigSchemaVersion, err)
	}
	return EnsureV24Tombstone(dataDir)
}
