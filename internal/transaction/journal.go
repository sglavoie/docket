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
// the replacement bytes for each write beside its target and fsyncs them, then
// writes the manifest in StatePrepared to the durable transactions directory.
// Writing the manifest is the preparation barrier: once it returns successfully
// every staged artifact it names is durably on disk. If validation, staging, or
// the manifest write fails, any staging files created by this call are removed
// and no prepared manifest is left behind.
func (j *Journal) Prepare(ops []Op) (*Manifest, error) {
	if len(ops) == 0 {
		return nil, ErrEmptyTransaction
	}

	// Validate every operation up front so a bad target is caught before any
	// bytes touch the filesystem.
	for i, op := range ops {
		if err := j.validateOp(op); err != nil {
			return nil, fmt.Errorf("op %d: %w", i, err)
		}
	}

	id := j.newID()
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
			if err := j.stageBytes(stagePath, op.Data, mode); err != nil {
				cleanup()
				return nil, fmt.Errorf("op %d: staging write target %q: %w", i, op.Target, err)
			}
			staged = append(staged, stagePath)
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

// stagePath returns the staging file path beside target for a given transaction
// id and operation index. Staging beside the target keeps the eventual commit an
// atomic same-directory rename.
func (j *Journal) stagePath(target, id string, index int) string {
	dir := filepath.Dir(target)
	name := fmt.Sprintf("%s%s-%03d%s", stagePrefix, id, index, stageSuffix)
	return filepath.Join(dir, name)
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

// List returns every valid manifest currently stored in the transactions
// directory, sorted by id. A missing directory yields no manifests. It is the
// entry point crash recovery uses to enumerate in-flight transactions.
func (j *Journal) List() ([]*Manifest, error) {
	entries, err := os.ReadDir(j.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading transactions directory: %w", err)
	}
	var out []*Manifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), manifestSuffix) {
			continue
		}
		m, err := LoadManifest(filepath.Join(j.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
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
