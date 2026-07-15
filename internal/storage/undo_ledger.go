package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/transaction"
)

// UndoLedgerSchemaVersion is stamped into every persisted ledger file. A file
// carrying a newer version than the running binary understands is rejected
// rather than misinterpreted, matching the transaction manifest contract.
const UndoLedgerSchemaVersion = 1

const (
	undoLedgerFileName = "ledger.json"
	undoLedgerBlobsDir = "blobs"
)

// ArtifactImage is the before/after byte-image of a single file touched by a
// committed transaction. Undo restores the "before" state and redo restores the
// "after" state, where a state is either the retained blob bytes (Exists true)
// or the file's absence (Exists false). Blob bytes live in content-addressed
// files under the ledger's blobs directory; the JSON keeps only the hash
// reference so identical images across entries share one blob.
//
// This uniform model reverses every kind of committed side effect: a project
// tasks.md rewrite is one artifact (before-bytes → after-bytes); an artifact
// delete is one artifact (before-bytes → absent); an attachment copy is one
// artifact (absent → after-bytes); an attachment delete is one artifact per
// file (before-bytes → absent); and an attachment move across projects is two
// artifacts per file (the source path loses its bytes, the destination path
// gains them), so undo and redo relocate the bytes by writing each side to its
// recorded state.
type ArtifactImage struct {
	// Path is the absolute file the image restores.
	Path string `json:"path"`
	// BeforeExists reports whether the file existed before the transaction.
	BeforeExists bool `json:"before_exists"`
	// BeforeHash references the retained blob holding the pre-transaction bytes.
	// Empty when BeforeExists is false.
	BeforeHash string `json:"before_hash,omitempty"`
	// AfterExists reports whether the file existed after the transaction.
	AfterExists bool `json:"after_exists"`
	// AfterHash references the retained blob holding the post-transaction bytes.
	// Empty when AfterExists is false.
	AfterHash string `json:"after_hash,omitempty"`
}

// LedgerEntry is one append-only record of a committed transaction: the manifest
// id it corresponds to, the projects it affected (for current-project
// selection), and the before/after artifact images needed to reverse (undo) or
// replay (redo) it.
type LedgerEntry struct {
	// Seq is the monotonically increasing sequence assigned at record time. It
	// orders the undo and redo stacks even after entries move between them, so
	// "newest" selection is a max-Seq scan rather than slice position.
	Seq int `json:"seq"`
	// TransactionID is the committed transaction manifest id this entry reverses.
	TransactionID string `json:"transaction_id"`
	// Action and Context describe the mutation for user-facing undo messages.
	Action  string `json:"action,omitempty"`
	Context string `json:"context,omitempty"`
	// Timestamp records when the entry was appended.
	Timestamp time.Time `json:"timestamp"`
	// ProjectIDs lists the logical ids of every project the transaction touched,
	// so current-project undo can select the newest entry affecting a project.
	ProjectIDs []string `json:"project_ids,omitempty"`
	// Artifacts carries every affected file's before/after image.
	Artifacts []ArtifactImage `json:"artifacts,omitempty"`
}

// paths returns the sorted, de-duplicated set of every file path this entry
// touches. It is the overlap key for redo invalidation.
func (e LedgerEntry) paths() []string {
	seen := make(map[string]bool, len(e.Artifacts))
	out := make([]string, 0, len(e.Artifacts))
	for _, a := range e.Artifacts {
		clean := filepath.Clean(a.Path)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

func (e LedgerEntry) touchesProject(projectID string) bool {
	for _, id := range e.ProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

// blobHashes returns every non-empty blob hash referenced by this entry, used by
// blob garbage collection to compute the live set.
func (e LedgerEntry) blobHashes() []string {
	out := make([]string, 0, len(e.Artifacts)*2)
	for _, a := range e.Artifacts {
		if a.BeforeExists && a.BeforeHash != "" {
			out = append(out, a.BeforeHash)
		}
		if a.AfterExists && a.AfterHash != "" {
			out = append(out, a.AfterHash)
		}
	}
	return out
}

// ledgerFile is the persisted, schema-versioned workspace ledger: the undo stack
// (Entries, most recent has the highest Seq) and the redo stack (RedoEntries),
// plus the monotonic sequence counter.
type ledgerFile struct {
	SchemaVersion int           `json:"schema_version"`
	NextSeq       int           `json:"next_seq"`
	Entries       []LedgerEntry `json:"entries"`
	RedoEntries   []LedgerEntry `json:"redo_entries"`
}

// LedgerArtifact is a caller-supplied before/after byte-image for one file. The
// commit boundary captures these under the project locks (before images before
// applying the transaction, after images from the content it is about to write)
// and hands them to Record, which persists the bytes into content-addressed
// blobs and reduces them to an ArtifactImage.
type LedgerArtifact struct {
	Path         string
	BeforeExists bool
	Before       []byte
	AfterExists  bool
	After        []byte
}

// LedgerRecord is the request to append one committed transaction to the ledger.
type LedgerRecord struct {
	TransactionID string
	Action        string
	Context       string
	ProjectIDs    []string
	Artifacts     []LedgerArtifact
}

// preparedLedgerRecord is a side-effect-free ledger append plan. Ops write any
// missing content-addressed blobs first and the new ledger file last, so a
// CommitService can append them to the same transaction manifest as the user
// artifacts. file is the resulting in-memory ledger state used for post-commit
// best-effort blob garbage collection.
type preparedLedgerRecord struct {
	ops  []transaction.Op
	file *ledgerFile
}

// WorkspaceLedger is the single workspace-wide undo/redo history. It is an
// append-only ledger of committed transactions: each successful commit appends
// one entry referencing the transaction manifest and retaining the before/after
// image of every file the transaction touched. Undo and redo move entries
// between the undo and redo stacks while restoring the recorded images to disk.
//
// The commit boundary wires an optional ledger so callers cannot forget history.
// Project ChangeSets and ledger-aware artifact-only transactions use
// prepareRecord to put the blobs and ledger JSON in the same durable manifest as
// the user artifacts. A nil ledger disables history entirely, preserving behavior
// for callers and tests that do not opt in.
type WorkspaceLedger struct {
	dir             string
	transactionsDir string
	maxDepth        int
	roots           []string
	now             func() time.Time
}

// NewWorkspaceLedger returns a ledger persisted under dir (typically
// config.UndoLedgerDir(dataDir)). It retains at most maxDepth undo entries,
// pruning the oldest beyond that depth, and restricts every restore write and
// delete to paths inside roots as defense in depth. A maxDepth <= 0 disables
// retention limits (history grows unbounded); callers pass the configured undo
// depth.
func NewWorkspaceLedger(dir string, maxDepth int, roots []string) *WorkspaceLedger {
	cleanRoots := make([]string, 0, len(roots))
	for _, r := range roots {
		if r != "" {
			cleanRoots = append(cleanRoots, filepath.Clean(r))
		}
	}
	return &WorkspaceLedger{
		dir:             filepath.Clean(dir),
		transactionsDir: filepath.Join(filepath.Dir(filepath.Clean(dir)), "transactions"),
		maxDepth:        maxDepth,
		roots:           cleanRoots,
		now:             time.Now,
	}
}

func (l *WorkspaceLedger) ledgerPath() string {
	return filepath.Join(l.dir, undoLedgerFileName)
}

func (l *WorkspaceLedger) blobsDir() string {
	return filepath.Join(l.dir, undoLedgerBlobsDir)
}

func (l *WorkspaceLedger) blobPath(hash string) string {
	return filepath.Join(l.blobsDir(), hash+".blob")
}

// load reads the persisted ledger, returning an empty ledger when none exists.
func (l *WorkspaceLedger) load() (*ledgerFile, error) {
	data, err := os.ReadFile(l.ledgerPath())
	if os.IsNotExist(err) {
		return &ledgerFile{SchemaVersion: UndoLedgerSchemaVersion, NextSeq: 1}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read undo ledger: %w", err)
	}
	var lf ledgerFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("failed to parse undo ledger: %w", err)
	}
	if lf.SchemaVersion == 0 {
		lf.SchemaVersion = UndoLedgerSchemaVersion
	}
	if lf.SchemaVersion > UndoLedgerSchemaVersion {
		return nil, fmt.Errorf("undo ledger schema version %d newer than supported %d", lf.SchemaVersion, UndoLedgerSchemaVersion)
	}
	if lf.NextSeq == 0 {
		lf.NextSeq = 1
	}
	return &lf, nil
}

func encodeLedgerFile(lf *ledgerFile) ([]byte, error) {
	lf.SchemaVersion = UndoLedgerSchemaVersion
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal undo ledger: %w", err)
	}
	return data, nil
}

func (l *WorkspaceLedger) readBlob(hash string) ([]byte, error) {
	data, err := os.ReadFile(l.blobPath(hash))
	if err != nil {
		return nil, fmt.Errorf("failed to read undo blob %s: %w", shortRevision(hash), err)
	}
	return data, nil
}

// prepareRecord builds the complete filesystem change for one ledger append
// without writing anything. That lets every undoable commit stage the ledger
// blobs and JSON in its user-artifact manifest instead of risking a successful
// mutation followed by a failed, missing undo entry.
func (l *WorkspaceLedger) prepareRecord(rec LedgerRecord) (*preparedLedgerRecord, error) {
	if rec.TransactionID == "" {
		return nil, fmt.Errorf("storage: ledger record requires a transaction id")
	}
	lf, loadErr := l.load()
	if loadErr != nil {
		return nil, loadErr
	}

	var ops []transaction.Op
	plannedBlobs := make(map[string]bool)
	planBlob := func(data []byte) (string, error) {
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		if plannedBlobs[hash] {
			return hash, nil
		}
		plannedBlobs[hash] = true
		path := l.blobPath(hash)
		if _, statErr := os.Stat(path); statErr == nil {
			return hash, nil
		} else if !os.IsNotExist(statErr) {
			return "", fmt.Errorf("failed to stat undo blob: %w", statErr)
		}
		ops = append(ops, transaction.Op{
			Kind: transaction.OpWrite, Target: path, Data: data, Mode: FilePermissions,
		})
		return hash, nil
	}

	images := make([]ArtifactImage, 0, len(rec.Artifacts))
	for _, a := range rec.Artifacts {
		img := ArtifactImage{
			Path:         filepath.Clean(a.Path),
			BeforeExists: a.BeforeExists,
			AfterExists:  a.AfterExists,
		}
		if a.BeforeExists {
			hash, blobErr := planBlob(a.Before)
			if blobErr != nil {
				return nil, blobErr
			}
			img.BeforeHash = hash
		}
		if a.AfterExists {
			hash, blobErr := planBlob(a.After)
			if blobErr != nil {
				return nil, blobErr
			}
			img.AfterHash = hash
		}
		images = append(images, img)
	}

	entry := LedgerEntry{
		Seq:           lf.NextSeq,
		TransactionID: rec.TransactionID,
		Action:        rec.Action,
		Context:       rec.Context,
		Timestamp:     l.now(),
		ProjectIDs:    dedupeStrings(rec.ProjectIDs),
		Artifacts:     images,
	}
	lf.NextSeq++
	lf.Entries = append(lf.Entries, entry)

	// Redo invalidation: any pending redo entry that shares a file with the new
	// mutation can no longer be safely replayed, so drop it.
	lf.RedoEntries = removeOverlappingRedo(lf.RedoEntries, entry.paths())

	// Retention pruning: keep only the newest maxDepth undo entries.
	if l.maxDepth > 0 && len(lf.Entries) > l.maxDepth {
		sort.SliceStable(lf.Entries, func(i, j int) bool { return lf.Entries[i].Seq < lf.Entries[j].Seq })
		lf.Entries = lf.Entries[len(lf.Entries)-l.maxDepth:]
	}

	data, encodeErr := encodeLedgerFile(lf)
	if encodeErr != nil {
		return nil, encodeErr
	}
	ops = append(ops, transaction.Op{
		Kind: transaction.OpWrite, Target: l.ledgerPath(), Data: data, Mode: FilePermissions,
	})
	return &preparedLedgerRecord{ops: ops, file: lf}, nil
}

// Record appends a committed transaction to the ledger. It retains every
// artifact's before/after bytes as content-addressed blobs, clears overlapping
// redo entries, prunes the undo stack to its retention depth, and finally
// garbage-collects unreferenced blobs. CommitService uses prepareRecord directly
// so those writes share the user transaction; Record remains available to the
// standalone ledger API and focused ledger tests. The standalone path still uses
// the workspace commit lock and journal so it cannot race a commit or restore.
func (l *WorkspaceLedger) Record(rec LedgerRecord) error {
	service := NewCommitService(l.transactionsDir, l.roots)
	return service.withWorkspaceCommitLock(func() error {
		journal := transaction.NewJournal(l.transactionsDir, l.roots)
		if _, err := service.recoverLocked(journal); err != nil {
			return err
		}
		plan, err := l.prepareRecord(rec)
		if err != nil {
			return err
		}
		manifest, err := journal.Prepare(plan.ops)
		if err != nil {
			return fmt.Errorf("preparing undo ledger transaction: %w", err)
		}
		if err := journal.Commit(manifest); err != nil {
			return fmt.Errorf("committing undo ledger transaction: %w", err)
		}
		l.gcBlobs(plan.file)
		return nil
	})
}

// LedgerScope selects which history line an undo/redo targets: the whole
// workspace, or the newest entry affecting one project.
type LedgerScope struct {
	// ProjectID, when non-empty, restricts selection to the newest entry that
	// touched that project (current-project undo). Empty selects globally.
	ProjectID string
}

// GlobalScope targets the newest entry across the whole workspace.
func GlobalScope() LedgerScope { return LedgerScope{} }

// ProjectScope targets the newest entry affecting a single project.
func ProjectScope(projectID string) LedgerScope { return LedgerScope{ProjectID: projectID} }

// SelectNewestUndo returns the entry a subsequent Undo(scope) would reverse
// without applying it, and whether one exists. It is the read-only counterpart
// callers use to preview or gate the availability of undo.
func (l *WorkspaceLedger) SelectNewestUndo(scope LedgerScope) (LedgerEntry, bool, error) {
	lf, err := l.load()
	if err != nil {
		return LedgerEntry{}, false, err
	}
	entry, _, ok := selectNewest(lf.Entries, scope)
	return entry, ok, nil
}

// UndoCount reports how many undo entries the scope can still reach; the CLI
// uses it to tell the user how many steps remain after a restore.
func (l *WorkspaceLedger) UndoCount(scope LedgerScope) (int, error) {
	lf, err := l.load()
	if err != nil {
		return 0, err
	}
	return countInScope(lf.Entries, scope), nil
}

// RedoCount reports how many redo entries the scope can still reach.
func (l *WorkspaceLedger) RedoCount(scope LedgerScope) (int, error) {
	lf, err := l.load()
	if err != nil {
		return 0, err
	}
	return countInScope(lf.RedoEntries, scope), nil
}

func countInScope(stack []LedgerEntry, scope LedgerScope) int {
	if scope.ProjectID == "" {
		return len(stack)
	}
	n := 0
	for i := range stack {
		if stack[i].touchesProject(scope.ProjectID) {
			n++
		}
	}
	return n
}

// SelectNewestRedo returns the entry a subsequent Redo(scope) would replay.
func (l *WorkspaceLedger) SelectNewestRedo(scope LedgerScope) (LedgerEntry, bool, error) {
	lf, err := l.load()
	if err != nil {
		return LedgerEntry{}, false, err
	}
	entry, _, ok := selectNewest(lf.RedoEntries, scope)
	return entry, ok, nil
}

// Undo reverses the newest entry in scope, restoring every artifact to its
// before-image, then moves the entry onto the redo stack. It returns the undone
// action description. A global scope selects the newest entry overall; a project
// scope selects the newest entry that touched that project even when later
// entries for other projects sit above it (out-of-order project histories).
func (l *WorkspaceLedger) Undo(scope LedgerScope) (string, error) {
	_, action, err := l.UndoWithEntry(scope)
	return action, err
}

// UndoWithEntry is Undo plus the exact entry selected under the workspace lock.
// Callers that must refresh affected projects use this instead of previewing the
// ledger separately and then racing a concurrent commit before restore begins.
func (l *WorkspaceLedger) UndoWithEntry(scope LedgerScope) (LedgerEntry, string, error) {
	return l.restore(scope, false)
}

// Redo replays the newest entry in scope, restoring every artifact to its
// after-image, then moves the entry back onto the undo stack.
func (l *WorkspaceLedger) Redo(scope LedgerScope) (string, error) {
	_, action, err := l.RedoWithEntry(scope)
	return action, err
}

// RedoWithEntry is Redo plus the exact entry selected under the workspace lock.
func (l *WorkspaceLedger) RedoWithEntry(scope LedgerScope) (LedgerEntry, string, error) {
	return l.restore(scope, true)
}

// restore is the shared undo/redo engine. redo=false consumes the undo stack and
// applies before-images; redo=true consumes the redo stack and applies
// after-images. The consumed entry moves to the opposite stack so the operation
// is itself reversible, matching the linear Vim-style contract.
func (l *WorkspaceLedger) restore(scope LedgerScope, redo bool) (LedgerEntry, string, error) {
	service := NewCommitService(l.transactionsDir, l.roots)
	var restored LedgerEntry
	err := service.withWorkspaceCommitLock(func() error {
		journal := transaction.NewJournal(l.transactionsDir, l.roots)
		if _, recoverErr := service.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}

		lf, loadErr := l.load()
		if loadErr != nil {
			return loadErr
		}
		source := lf.Entries
		label := "undo"
		if redo {
			source = lf.RedoEntries
			label = "redo"
		}
		entry, idx, ok := selectNewest(source, scope)
		if !ok {
			if scope.ProjectID != "" {
				return fmt.Errorf("no %s history available for project %q", label, scope.ProjectID)
			}
			return fmt.Errorf("no %s history available", label)
		}

		artifacts := make([]ArtifactChange, 0, len(entry.Artifacts))
		for _, image := range entry.Artifacts {
			artifacts = append(artifacts, ArtifactChange{Path: image.Path})
		}
		lockPaths, lockErr := artifactLockPaths(artifacts, l.roots)
		if lockErr != nil {
			return lockErr
		}
		if err := createArtifactLockDirectories(lockPaths); err != nil {
			return err
		}
		return withOrderedFileLocks(lockPaths, func() error {
			manifest, prepareErr := l.prepareRestoreTransaction(journal, lf, entry, idx, redo)
			if prepareErr != nil {
				return prepareErr
			}
			if commitErr := journal.Commit(manifest); commitErr != nil {
				return fmt.Errorf("committing %s transaction: %w", label, commitErr)
			}
			l.gcBlobs(lf)
			restored = entry
			return nil
		})
	})
	if err != nil {
		return LedgerEntry{}, "", err
	}
	return restored, ledgerEntryDescription(restored), nil
}

// prepareRestoreTransaction validates the complete selected entry before it
// stages anything, then journals every target image with the moved stack state.
// Artifact operations stay first and ledger.json is last, so recovery can always
// roll a crash forward to matching user bytes and visible history.
func (l *WorkspaceLedger) prepareRestoreTransaction(
	journal *transaction.Journal,
	lf *ledgerFile,
	entry LedgerEntry,
	idx int,
	redo bool,
) (*transaction.Manifest, error) {
	ops, err := l.preflightRestoreOps(entry, redo)
	if err != nil {
		return nil, err
	}
	if redo {
		lf.RedoEntries = removeEntryAt(lf.RedoEntries, idx)
		lf.Entries = append(lf.Entries, entry)
	} else {
		lf.Entries = removeEntryAt(lf.Entries, idx)
		lf.RedoEntries = append(lf.RedoEntries, entry)
	}
	ledgerData, err := encodeLedgerFile(lf)
	if err != nil {
		return nil, err
	}
	ops = append(ops, transaction.Op{
		Kind: transaction.OpWrite, Target: l.ledgerPath(), Data: ledgerData, Mode: FilePermissions,
	})
	manifest, err := journal.Prepare(ops)
	if err != nil {
		return nil, fmt.Errorf("preparing ledger restore transaction: %w", err)
	}
	return manifest, nil
}

// preflightRestoreOps validates both retained images for every artifact and the
// current target revision before returning the desired write/delete operations.
// Undo expects each target to still match its after-image; redo expects the
// before-image. Any drift aborts the whole grouped restore before Journal.Prepare
// creates a staging file or manifest.
func (l *WorkspaceLedger) preflightRestoreOps(entry LedgerEntry, redo bool) ([]transaction.Op, error) {
	blobs := make(map[string][]byte)
	loadImage := func(path, label string, exists bool, hash string) error {
		if !exists {
			if hash != "" {
				return fmt.Errorf("storage: ledger artifact %q has a %s hash for an absent image", path, label)
			}
			return nil
		}
		if hash == "" {
			return fmt.Errorf("storage: ledger artifact %q is missing its %s blob hash", path, label)
		}
		if _, ok := blobs[hash]; ok {
			return nil
		}
		data, err := l.readBlob(hash)
		if err != nil {
			return err
		}
		if actual := RawArtifactRevision(data); actual != hash {
			return fmt.Errorf("storage: undo blob %s failed its content hash check", shortRevision(hash))
		}
		blobs[hash] = data
		return nil
	}

	seen := make(map[string]bool, len(entry.Artifacts))
	for _, image := range entry.Artifacts {
		path := filepath.Clean(image.Path)
		if seen[path] {
			return nil, fmt.Errorf("storage: ledger entry contains duplicate artifact target %q", path)
		}
		seen[path] = true
		if err := loadImage(path, "before", image.BeforeExists, image.BeforeHash); err != nil {
			return nil, err
		}
		if err := loadImage(path, "after", image.AfterExists, image.AfterHash); err != nil {
			return nil, err
		}
	}

	ops := make([]transaction.Op, 0, len(entry.Artifacts))
	for _, image := range entry.Artifacts {
		path := filepath.Clean(image.Path)
		expectedExists, expectedHash := image.AfterExists, image.AfterHash
		desiredExists, desiredHash := image.BeforeExists, image.BeforeHash
		if redo {
			expectedExists, expectedHash = image.BeforeExists, image.BeforeHash
			desiredExists, desiredHash = image.AfterExists, image.AfterHash
		}

		current, currentExists, err := readFileMaybe(path)
		if err != nil {
			return nil, fmt.Errorf("reading ledger restore target %q: %w", path, err)
		}
		actualHash := ""
		if currentExists {
			actualHash = RawArtifactRevision(current)
		}
		if currentExists != expectedExists || (currentExists && actualHash != expectedHash) {
			return nil, &ArtifactConflictError{Path: path, Expected: expectedHash, Actual: actualHash}
		}

		if desiredExists {
			ops = append(ops, transaction.Op{
				Kind: transaction.OpWrite, Target: path, Data: blobs[desiredHash], Mode: FilePermissions,
			})
		} else {
			ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
		}
	}
	return ops, nil
}

// gcBlobs removes every retained blob no longer referenced by any surviving
// entry in either stack. It is best-effort: a failed removal leaks a blob but
// never corrupts the ledger, so errors are swallowed.
func (l *WorkspaceLedger) gcBlobs(lf *ledgerFile) {
	live := make(map[string]bool)
	for _, e := range lf.Entries {
		for _, h := range e.blobHashes() {
			live[h] = true
		}
	}
	for _, e := range lf.RedoEntries {
		for _, h := range e.blobHashes() {
			live[h] = true
		}
	}
	entries, err := os.ReadDir(l.blobsDir())
	if err != nil {
		return
	}
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || filepath.Ext(name) != ".blob" {
			continue
		}
		hash := name[:len(name)-len(".blob")]
		if !live[hash] {
			_ = os.Remove(filepath.Join(l.blobsDir(), name))
		}
	}
}

// selectNewest returns the highest-Seq entry in stack matching scope, its index,
// and whether one was found.
func selectNewest(stack []LedgerEntry, scope LedgerScope) (LedgerEntry, int, bool) {
	best := -1
	for i := range stack {
		if scope.ProjectID != "" && !stack[i].touchesProject(scope.ProjectID) {
			continue
		}
		if best == -1 || stack[i].Seq > stack[best].Seq {
			best = i
		}
	}
	if best == -1 {
		return LedgerEntry{}, -1, false
	}
	return stack[best], best, true
}

func removeEntryAt(stack []LedgerEntry, idx int) []LedgerEntry {
	out := make([]LedgerEntry, 0, len(stack)-1)
	out = append(out, stack[:idx]...)
	out = append(out, stack[idx+1:]...)
	return out
}

// removeOverlappingRedo drops every redo entry that touches any path in
// newPaths, so a redo whose files were changed by a later mutation is discarded
// rather than replayed onto diverged content.
func removeOverlappingRedo(redo []LedgerEntry, newPaths []string) []LedgerEntry {
	if len(redo) == 0 || len(newPaths) == 0 {
		return redo
	}
	newSet := make(map[string]bool, len(newPaths))
	for _, p := range newPaths {
		newSet[p] = true
	}
	kept := make([]LedgerEntry, 0, len(redo))
	for _, e := range redo {
		overlap := false
		for _, p := range e.paths() {
			if newSet[p] {
				overlap = true
				break
			}
		}
		if !overlap {
			kept = append(kept, e)
		}
	}
	return kept
}

func ledgerEntryDescription(entry LedgerEntry) string {
	if entry.Action == "" {
		return entry.TransactionID
	}
	if entry.Context != "" {
		return fmt.Sprintf("%s: %s", entry.Action, entry.Context)
	}
	return entry.Action
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
