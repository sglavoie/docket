package transaction

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/util"
)

// stagePrefix names staging files written beside their targets. The prefix is
// recognizable so orphaned staging files left by a crash before the manifest was
// written (i.e. with no manifest referencing them) can be identified and swept
// by crash recovery.
const stagePrefix = ".tt-stage-"

// manifestSuffix is the extension of a manifest file within the transactions
// directory.
const manifestSuffix = ".json"

// stageSuffix is the extension of a staging file written beside its target.
const stageSuffix = ".stage"

// Op is a requested file operation supplied to Prepare. For a write, Data holds
// the replacement bytes to stage and Mode the target's permission bits (a zero
// Mode defaults to owner-only 0600). For a move, Source is the existing path to
// rename onto Target. For a delete, only Target is used.
type Op struct {
	Kind   OpKind
	Target string
	Source string
	Data   []byte
	Mode   os.FileMode
}

// Journal prepares and persists transaction manifests under a durable
// transactions directory, validating every target against a set of approved
// workspace roots before staging any bytes. Construct it with NewJournal.
type Journal struct {
	dir   string
	roots []string

	// Injectable seams so tests can drive deterministic ids/timestamps and
	// simulate failures at every prepare, fsync, rename, manifest-update, and
	// cleanup boundary. writeFile covers staging and manifest updates; rename
	// covers commit/quarantine renames; remove covers cleanup; syncDir covers the
	// post-rename directory fsync.
	now       func() time.Time
	newID     func() string
	writeFile func(path string, data []byte, perm os.FileMode) error
	mkdirAll  func(path string, perm os.FileMode) error
	rename    func(oldpath, newpath string) error
	remove    func(path string) error
	syncDir   func(dir string) error
}

// NewJournal returns a Journal that stores manifests under dir and only permits
// operations whose targets resolve inside one of roots. Both dir and roots are
// cleaned; empty roots are dropped. A journal with no usable roots rejects every
// operation, failing closed.
func NewJournal(dir string, roots []string) *Journal {
	cleanRoots := make([]string, 0, len(roots))
	for _, r := range roots {
		if r != "" {
			cleanRoots = append(cleanRoots, filepath.Clean(r))
		}
	}
	return &Journal{
		dir:       filepath.Clean(dir),
		roots:     cleanRoots,
		now:       time.Now,
		newID:     func() string { return uuid.New().String() },
		writeFile: util.AtomicWriteFile,
		mkdirAll:  os.MkdirAll,
		rename:    os.Rename,
		remove:    os.Remove,
		syncDir:   fsyncDir,
	}
}

// fsyncDir fsyncs a directory so a create, rename, or removal within it is
// durably recorded, surviving a crash immediately after the operation returns.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}

// ManifestPath returns the durable path of the manifest for a transaction id.
func (j *Journal) ManifestPath(id string) string {
	return filepath.Join(j.dir, id+manifestSuffix)
}

// Prepare validates every operation against the approved workspace roots, stages
// and fsyncs the replacement bytes for each write, then writes the manifest in
// StatePrepared to the durable transactions directory. A write nested below an
// earlier directory-move target is staged at the corresponding source location,
// so preparation never occupies the move destination; that move carries the
// stage to the destination-side path recorded in the manifest.
// Writing the manifest is the preparation barrier: once it returns successfully
// every staged artifact it names is durably on disk. If validation, staging, or
// the manifest write fails, any staging files created by this call are removed
// and no prepared manifest is left behind.
func (j *Journal) Prepare(ops []Op) (*Manifest, error) {
	return j.prepareWithID(j.newID(), ops)
}

// PrepareWithID is Prepare with a caller-supplied transaction ID. It exists for
// commit coordinators that must embed the same ID in another operation staged in
// this manifest (for example a workspace-ledger entry). IDs are restricted to a
// single portable filename component and an existing manifest is never reused.
// Ordinary callers should use Prepare, which generates a fresh UUID.
func (j *Journal) PrepareWithID(id string, ops []Op) (*Manifest, error) {
	return j.prepareWithID(id, ops)
}

func (j *Journal) prepareWithID(id string, ops []Op) (*Manifest, error) {
	if len(ops) == 0 {
		return nil, ErrEmptyTransaction
	}
	if !validTransactionID(id) {
		return nil, fmt.Errorf("transaction id %q is invalid: %w", id, ErrInvalidManifest)
	}
	if _, err := os.Lstat(j.ManifestPath(id)); err == nil {
		return nil, fmt.Errorf("transaction id %q already exists: %w", id, ErrInvalidManifest)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("checking transaction id %q: %w", id, err)
	}

	// Validate every operation up front so a bad target is caught before any
	// bytes touch the filesystem.
	for i, op := range ops {
		if err := j.validateOp(op); err != nil {
			return nil, fmt.Errorf("op %d: %w", i, err)
		}
	}

	manifest := &Manifest{
		SchemaVersion: SchemaVersion,
		ID:            id,
		State:         StatePrepared,
		CreatedAt:     j.now().UTC().Format(time.RFC3339Nano),
		Ops:           make([]FileOp, 0, len(ops)),
	}

	// Track staging files so a later failure can roll back partial staging.
	var staged []string
	cleanup := func() {
		for _, p := range staged {
			_ = os.Remove(p)
		}
	}

	for i, op := range ops {
		fop := FileOp{Kind: op.Kind, Target: filepath.Clean(op.Target)}
		switch op.Kind {
		case OpWrite:
			mode := op.Mode
			if mode == 0 {
				mode = util.FilePermissions
			}
			stagePath := j.stagePath(op.Target, id, i)
			preparePath := j.prepareStagePath(stagePath, ops[:i])
			if err := j.stageBytes(preparePath, op.Data, mode); err != nil {
				cleanup()
				return nil, fmt.Errorf("op %d: staging write target %q: %w", i, op.Target, err)
			}
			staged = append(staged, preparePath)
			fop.Staged = stagePath
			fop.Hash = hashBytes(op.Data)
			fop.Mode = mode
		case OpMove:
			fop.Source = filepath.Clean(op.Source)
		case OpDelete:
			// Nothing to stage for a delete.
		}
		manifest.Ops = append(manifest.Ops, fop)
	}

	if err := j.writeManifest(manifest); err != nil {
		cleanup()
		return nil, fmt.Errorf("writing manifest %s: %w", id, err)
	}
	return manifest, nil
}

func validTransactionID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// Commit applies a prepared manifest's file operations online, advancing it
// through committing to committed and reclaiming its staged artifacts and
// manifest. It shares the exact apply/reclaim path used by crash recovery, so a
// commit interrupted by a crash leaves a durable committing (or still prepared)
// manifest that Recover re-drives to completion on the next startup — a commit
// is therefore never lost once Prepare has returned.
//
// Only a prepared or committing manifest may be committed; any other state is
// refused as an invalid manifest. Every target (and move source) is re-checked
// against the approved workspace roots as defense in depth before any bytes are
// moved into place.
func (j *Journal) Commit(m *Manifest) error {
	if m == nil {
		return fmt.Errorf("committing transaction: %w", ErrInvalidManifest)
	}
	for i, op := range m.Ops {
		if !j.withinRoots(op.Target) {
			return fmt.Errorf("op %d target %q: %w", i, op.Target, ErrTargetOutsideRoot)
		}
		if op.Kind == OpMove && !j.withinRoots(op.Source) {
			return fmt.Errorf("op %d move source %q: %w", i, op.Source, ErrTargetOutsideRoot)
		}
	}
	switch m.State {
	case StatePrepared, StateCommitting:
		return j.driveCommit(m, j.ManifestPath(m.ID))
	default:
		return fmt.Errorf("cannot commit manifest %q in state %q: %w", m.ID, m.State, ErrInvalidManifest)
	}
}

// validateOp checks an operation's kind and required fields and confirms its
// target (and, for a move, its source) resolves inside an approved root.
func (j *Journal) validateOp(op Op) error {
	switch op.Kind {
	case OpWrite, OpDelete:
	case OpMove:
		if op.Source == "" {
			return fmt.Errorf("move requires a source: %w", ErrInvalidOp)
		}
		if !j.withinRoots(op.Source) {
			return fmt.Errorf("move source %q: %w", op.Source, ErrTargetOutsideRoot)
		}
	default:
		return fmt.Errorf("%q: %w", op.Kind, ErrUnknownOpKind)
	}
	if op.Target == "" {
		return fmt.Errorf("operation requires a target: %w", ErrInvalidOp)
	}
	if !j.withinRoots(op.Target) {
		return fmt.Errorf("target %q: %w", op.Target, ErrTargetOutsideRoot)
	}
	return nil
}

// withinRoots reports whether path resolves inside any approved root.
func (j *Journal) withinRoots(path string) bool {
	for _, root := range j.roots {
		if util.PathWithinRoot(root, path) {
			return true
		}
	}
	return false
}

// stagePath returns the eventual staging file path beside target for a given
// transaction id and operation index. prepareStagePath may place the bytes below
// an earlier directory-move source until that move carries them here. Keeping the
// recorded stage beside the target makes the final write an atomic rename.
func (j *Journal) stagePath(target, id string, index int) string {
	dir := filepath.Dir(target)
	name := fmt.Sprintf("%s%s-%03d%s", stagePrefix, id, index, stageSuffix)
	return filepath.Join(dir, name)
}

// prepareStagePath returns where staged bytes must be written before the
// manifest is durable. Normally that is the final destination-side stage path.
// When an earlier operation moves a directory that contains the write target,
// creating that path would occupy the move destination and make the rename fail.
// Stage at the corresponding location below the existing source instead. The
// manifest still records finalPath: applying the earlier move carries the stage
// file to that exact path before applyWrite consumes it. Recovery can therefore
// restart before or after either rename without special-case replay state.
func (j *Journal) prepareStagePath(finalPath string, priorOps []Op) string {
	preparePath := filepath.Clean(finalPath)
	for i := len(priorOps) - 1; i >= 0; i-- {
		move := priorOps[i]
		if move.Kind != OpMove || !directoryMoveSourceExists(move.Source, priorOps[:i]) {
			continue
		}
		if rebased, ok := rebaseDescendant(preparePath, move.Target, move.Source); ok {
			preparePath = rebased
		}
	}
	return preparePath
}

// directoryMoveSourceExists reports whether source is a directory before its
// move runs. Earlier moves are reversed lexically so chained directory moves can
// still carry later staged writes without creating an as-yet absent source.
func directoryMoveSourceExists(source string, priorOps []Op) bool {
	path := filepath.Clean(source)
	for i := len(priorOps) - 1; i >= 0; i-- {
		move := priorOps[i]
		if move.Kind != OpMove {
			continue
		}
		if rebased, ok := rebaseDescendant(path, move.Target, move.Source); ok {
			path = rebased
		}
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// rebaseDescendant maps path from one directory tree to another. It is lexical
// because the destination commonly does not exist until the transaction applies.
func rebaseDescendant(path, from, to string) (string, bool) {
	rel, err := filepath.Rel(filepath.Clean(from), filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(filepath.Clean(to), rel), true
}

// stageBytes durably writes replacement bytes to a staging path, creating its
// parent directory if needed. The atomic write fsyncs the file and its directory,
// so the staged data survives a crash immediately after Prepare returns.
func (j *Journal) stageBytes(stagePath string, data []byte, mode os.FileMode) error {
	if err := j.mkdirAll(filepath.Dir(stagePath), util.DirPermissions); err != nil {
		return fmt.Errorf("creating staging directory: %w", err)
	}
	if err := j.writeFile(stagePath, data, mode); err != nil {
		return err
	}
	return nil
}

// writeManifest atomically persists a manifest to its durable path.
func (j *Journal) writeManifest(m *Manifest) error {
	if err := j.mkdirAll(j.dir, util.DirPermissions); err != nil {
		return fmt.Errorf("creating transactions directory: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}
	return j.writeFile(j.ManifestPath(m.ID), data, util.FilePermissions)
}

// LoadManifest reads and validates a manifest from disk.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %q: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decoding manifest %q: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest %q: %w", path, err)
	}
	return &m, nil
}
