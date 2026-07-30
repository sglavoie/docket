package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/transaction"
)

// UndoLedgerSchemaVersion is stamped into every persisted ledger file. A file
// carrying a newer version than the running binary understands is rejected
// rather than misinterpreted, matching the transaction manifest contract.
const UndoLedgerSchemaVersion = 2

const undoLedgerSchemaVersionV1 = 1

const (
	undoLedgerFileName = "ledger.json"
	undoLedgerBlobsDir = "blobs"
	WorkspaceUndoLimit = 50
)

// ArtifactImageKind distinguishes retained file contents from directory
// existence. Schema-v1 images have no kind and are interpreted as files.
type ArtifactImageKind string

const (
	ArtifactImageKindFile      ArtifactImageKind = "file"
	ArtifactImageKindDirectory ArtifactImageKind = "directory"
)

// ArtifactImage is the before/after image of one filesystem artifact touched by
// a committed transaction. File images retain byte hashes; directory images
// retain only type and existence. Undo restores the "before" state and redo
// restores the "after" state.
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
	// Kind is file or directory. It was absent from schema-v1 file images.
	Kind ArtifactImageKind `json:"kind"`
	// Path is the absolute filesystem path the image restores.
	Path string `json:"path"`
	// BeforeExists reports whether the artifact existed before the transaction.
	BeforeExists bool `json:"before_exists"`
	// BeforeHash references the retained blob holding the pre-transaction bytes.
	// Empty when BeforeExists is false.
	BeforeHash string `json:"before_hash,omitempty"`
	// AfterExists reports whether the artifact existed after the transaction.
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
	// Artifacts carries every affected file or directory's before/after image.
	Artifacts []ArtifactImage `json:"artifacts,omitempty"`
	// LegacySchemaVersion preserves the provenance of entries upgraded from v1.
	// Their file-only images need conservative attachment-directory cleanup on
	// every undo/redo, including after the containing ledger is rewritten as v2.
	LegacySchemaVersion int `json:"legacy_schema_version,omitempty"`
}

func (a ArtifactImage) imageKind() ArtifactImageKind {
	if a.Kind == "" {
		return ArtifactImageKindFile
	}
	return a.Kind
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

// LedgerArtifact is a caller-supplied before/after image. File images carry
// bytes; directory images carry only existence. The commit boundary captures
// these under the project locks and hands them to Record for persistence.
type LedgerArtifact struct {
	Kind         ArtifactImageKind
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
	roots           []string
	now             func() time.Time
}

// WorkspaceLedgerValidation summarizes a successful read-only validation.
type WorkspaceLedgerValidation struct {
	UndoEntries int
	RedoEntries int
}

// NewWorkspaceLedger returns a ledger persisted under dir (typically
// config.UndoLedgerDir(dataDir)). It retains exactly the newest
// WorkspaceUndoLimit committed transactions and restricts every restore write
// and delete to paths inside roots as defense in depth.
func NewWorkspaceLedger(dir string, roots []string) *WorkspaceLedger {
	cleanRoots := make([]string, 0, len(roots))
	for _, r := range roots {
		if r != "" {
			cleanRoots = append(cleanRoots, filepath.Clean(r))
		}
	}
	ledger := &WorkspaceLedger{
		dir:             filepath.Clean(dir),
		transactionsDir: filepath.Join(filepath.Dir(filepath.Clean(dir)), "transactions"),
		roots:           cleanRoots,
		now:             time.Now,
	}
	return ledger
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
		lf.SchemaVersion = undoLedgerSchemaVersionV1
	}
	if lf.SchemaVersion > UndoLedgerSchemaVersion {
		return nil, fmt.Errorf("undo ledger schema version %d newer than supported %d", lf.SchemaVersion, UndoLedgerSchemaVersion)
	}
	if lf.SchemaVersion < undoLedgerSchemaVersionV1 {
		return nil, fmt.Errorf("undo ledger schema version %d is invalid", lf.SchemaVersion)
	}
	if err := normalizeLedgerImages(&lf); err != nil {
		return nil, err
	}
	pruneLedgerCapacity(&lf)
	if lf.NextSeq == 0 {
		lf.NextSeq = 1
	}
	return &lf, nil
}

// Validate checks the persisted ledger structure, restore-target containment,
// and every referenced blob without changing history or comparing against the
// workspace's current artifact bytes. A missing ledger is a valid empty
// history.
func (l *WorkspaceLedger) Validate() (WorkspaceLedgerValidation, error) {
	if l == nil {
		return WorkspaceLedgerValidation{}, fmt.Errorf("storage: workspace ledger is required")
	}
	lf, err := l.load()
	if err != nil {
		return WorkspaceLedgerValidation{}, err
	}

	seenSeq := make(map[int]bool, len(lf.Entries)+len(lf.RedoEntries))
	maxSeq := 0
	for _, stack := range []struct {
		name    string
		entries []LedgerEntry
	}{
		{name: "undo", entries: lf.Entries},
		{name: "redo", entries: lf.RedoEntries},
	} {
		for index, entry := range stack.entries {
			if entry.Seq <= 0 {
				return WorkspaceLedgerValidation{}, fmt.Errorf("storage: %s ledger entry %d has invalid sequence %d", stack.name, index, entry.Seq)
			}
			if seenSeq[entry.Seq] {
				return WorkspaceLedgerValidation{}, fmt.Errorf("storage: undo ledger repeats sequence %d", entry.Seq)
			}
			seenSeq[entry.Seq] = true
			if entry.Seq > maxSeq {
				maxSeq = entry.Seq
			}
			if strings.TrimSpace(entry.TransactionID) == "" {
				return WorkspaceLedgerValidation{}, fmt.Errorf("storage: ledger entry %d has no transaction id", entry.Seq)
			}
			for _, image := range entry.Artifacts {
				if !pathWithinApprovedRoots(image.Path, l.roots) {
					return WorkspaceLedgerValidation{}, fmt.Errorf("storage: ledger entry %d artifact path escapes approved roots: %s", entry.Seq, image.Path)
				}
			}
			if _, err := l.validateRestoreImages(entry); err != nil {
				return WorkspaceLedgerValidation{}, fmt.Errorf("storage: validate ledger entry %d: %w", entry.Seq, err)
			}
		}
	}
	if lf.NextSeq <= maxSeq {
		return WorkspaceLedgerValidation{}, fmt.Errorf("storage: undo ledger next sequence %d does not follow retained sequence %d", lf.NextSeq, maxSeq)
	}
	return WorkspaceLedgerValidation{
		UndoEntries: len(lf.Entries),
		RedoEntries: len(lf.RedoEntries),
	}, nil
}

func encodeLedgerFile(lf *ledgerFile) ([]byte, error) {
	if err := normalizeLedgerImages(lf); err != nil {
		return nil, err
	}
	pruneLedgerCapacity(lf)
	lf.SchemaVersion = UndoLedgerSchemaVersion
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal undo ledger: %w", err)
	}
	return data, nil
}

// pruneLedgerCapacity keeps the newest fixed-capacity transactions across both
// stacks. Undo and redo only move an entry between stacks, so counting both
// stacks enforces one workspace history even when loading a ledger written by
// an older release with a larger configured depth.
func pruneLedgerCapacity(lf *ledgerFile) {
	if lf == nil {
		return
	}
	total := len(lf.Entries) + len(lf.RedoEntries)
	if total <= WorkspaceUndoLimit {
		return
	}
	type entryRef struct {
		seq   int
		stack int
		index int
	}
	refs := make([]entryRef, 0, total)
	for i, entry := range lf.Entries {
		refs = append(refs, entryRef{seq: entry.Seq, index: i})
	}
	for i, entry := range lf.RedoEntries {
		refs = append(refs, entryRef{seq: entry.Seq, stack: 1, index: i})
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].seq != refs[j].seq {
			return refs[i].seq < refs[j].seq
		}
		if refs[i].stack != refs[j].stack {
			return refs[i].stack < refs[j].stack
		}
		return refs[i].index < refs[j].index
	})

	dropUndo := make(map[int]bool)
	dropRedo := make(map[int]bool)
	for _, ref := range refs[:total-WorkspaceUndoLimit] {
		if ref.stack == 0 {
			dropUndo[ref.index] = true
		} else {
			dropRedo[ref.index] = true
		}
	}
	lf.Entries = filterLedgerEntries(lf.Entries, dropUndo)
	lf.RedoEntries = filterLedgerEntries(lf.RedoEntries, dropRedo)
}

func filterLedgerEntries(entries []LedgerEntry, drop map[int]bool) []LedgerEntry {
	if len(drop) == 0 {
		return entries
	}
	filtered := make([]LedgerEntry, 0, len(entries)-len(drop))
	for i, entry := range entries {
		if !drop[i] {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func normalizeLedgerImages(lf *ledgerFile) error {
	stacks := [][]LedgerEntry{lf.Entries, lf.RedoEntries}
	for _, stack := range stacks {
		for entryIdx := range stack {
			entry := &stack[entryIdx]
			if lf.SchemaVersion == undoLedgerSchemaVersionV1 && entry.LegacySchemaVersion == 0 {
				entry.LegacySchemaVersion = undoLedgerSchemaVersionV1
			}
			for imageIdx := range entry.Artifacts {
				image := &entry.Artifacts[imageIdx]
				if image.Kind == "" {
					if lf.SchemaVersion != undoLedgerSchemaVersionV1 {
						return fmt.Errorf("storage: ledger v%d entry %d artifact %q is missing its image kind", lf.SchemaVersion, entry.Seq, image.Path)
					}
					image.Kind = ArtifactImageKindFile
				}
				switch image.Kind {
				case ArtifactImageKindFile:
				case ArtifactImageKindDirectory:
					if image.BeforeHash != "" || image.AfterHash != "" {
						return fmt.Errorf("storage: ledger directory artifact %q cannot reference file blobs", image.Path)
					}
				default:
					return fmt.Errorf("storage: ledger artifact %q has unknown image kind %q", image.Path, image.Kind)
				}
			}
		}
	}
	return nil
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
	// load marks every entry originating in a v1 ledger before a new v2 entry is
	// appended. Advance the in-memory schema now so normalization does not mistake
	// that new entry for legacy history when the mixed ledger is encoded.
	lf.SchemaVersion = UndoLedgerSchemaVersion

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
		kind := a.Kind
		if kind == "" {
			kind = ArtifactImageKindFile
		}
		if kind != ArtifactImageKindFile && kind != ArtifactImageKindDirectory {
			return nil, fmt.Errorf("storage: ledger artifact %q has unknown image kind %q", a.Path, kind)
		}
		img := ArtifactImage{
			Kind:         kind,
			Path:         filepath.Clean(a.Path),
			BeforeExists: a.BeforeExists,
			AfterExists:  a.AfterExists,
		}
		if kind == ArtifactImageKindFile && a.BeforeExists {
			hash, blobErr := planBlob(a.Before)
			if blobErr != nil {
				return nil, blobErr
			}
			img.BeforeHash = hash
		}
		if kind == ArtifactImageKindFile && a.AfterExists {
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

	// A forward mutation starts a new linear history branch. Every pending redo
	// transaction is therefore unreachable, including entries whose artifacts do
	// not overlap this mutation.
	lf.RedoEntries = nil

	// Retention pruning: keep only the newest fixed-capacity undo entries.
	if len(lf.Entries) > WorkspaceUndoLimit {
		sort.SliceStable(lf.Entries, func(i, j int) bool { return lf.Entries[i].Seq < lf.Entries[j].Seq })
		lf.Entries = lf.Entries[len(lf.Entries)-WorkspaceUndoLimit:]
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
// artifact's before/after bytes as content-addressed blobs, clears the entire
// redo stack, prunes the undo stack to its fixed capacity, and finally
// garbage-collects unreferenced blobs. CommitService uses prepareRecord directly
// so those writes share the user transaction; Record remains available to the
// standalone ledger API and focused ledger tests. When recording is enabled,
// the standalone path uses the workspace commit lock and journal so it cannot
// race a commit or restore.
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
// workspace, the newest entry affecting one project, the newest entry touching
// an artifact root, or the intersection of project and artifact restrictions.
type LedgerScope struct {
	// ProjectID, when non-empty, restricts selection to the newest entry that
	// touched that project (current-project undo).
	ProjectID string
	// ArtifactRoot, when non-empty, restricts selection to entries with at least
	// one artifact lexically contained by this path. Matching deliberately does
	// not resolve symlinks or require the artifact to exist: undo entries commonly
	// describe files that a create, delete, rename, or move made absent.
	ArtifactRoot string
}

// GlobalScope targets the newest entry across the whole workspace.
func GlobalScope() LedgerScope { return LedgerScope{} }

// ProjectScope targets the newest entry affecting a single project.
func ProjectScope(projectID string) LedgerScope { return LedgerScope{ProjectID: projectID} }

// ArtifactRootScope targets the newest entry touching root or one of its
// descendants, using cleaned lexical paths and path-component containment.
func ArtifactRootScope(root string) LedgerScope {
	return LedgerScope{ArtifactRoot: filepath.Clean(root)}
}

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
	n := 0
	for i := range stack {
		if entryMatchesScope(stack[i], scope) {
			n++
		}
	}
	return n
}

// entryMatchesScope is the single matching rule shared by selection and stack
// counts. Non-empty restrictions combine with AND semantics; the zero-value
// scope therefore matches every workspace entry.
func entryMatchesScope(entry LedgerEntry, scope LedgerScope) bool {
	if scope.ProjectID != "" && !entry.touchesProject(scope.ProjectID) {
		return false
	}
	if scope.ArtifactRoot == "" {
		return true
	}
	for _, artifact := range entry.Artifacts {
		if pathWithinArtifactRoot(scope.ArtifactRoot, artifact.Path) {
			return true
		}
		// A directory image also touches scopes nested below that directory. This
		// matters when callers scope history to one attachment file or subtree.
		if artifact.imageKind() == ArtifactImageKindDirectory && pathWithinArtifactRoot(artifact.Path, scope.ArtifactRoot) {
			return true
		}
	}
	return false
}

// pathWithinArtifactRoot reports lexical path-component containment only. It
// intentionally avoids filesystem inspection and symlink resolution because a
// restore target may currently be absent.
func pathWithinArtifactRoot(root, target string) bool {
	if root == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
			switch {
			case scope.ProjectID != "" && scope.ArtifactRoot != "":
				return fmt.Errorf("no %s history available for project %q under artifact root %q", label, scope.ProjectID, scope.ArtifactRoot)
			case scope.ProjectID != "":
				return fmt.Errorf("no %s history available for project %q", label, scope.ProjectID)
			case scope.ArtifactRoot != "":
				return fmt.Errorf("no %s history available for artifact root %q", label, scope.ArtifactRoot)
			}
			return fmt.Errorf("no %s history available", label)
		}

		var directoryLockPaths []string
		for _, image := range entry.Artifacts {
			if image.imageKind() == ArtifactImageKindDirectory {
				directoryLockPaths = append(directoryLockPaths, filepath.Clean(image.Path))
			}
		}
		// Attachment files use the adjacent task-directory lock even when the
		// directory itself did not transition. Creating a per-file lock inside an
		// attachment directory would add a non-user sibling and alter cleanup.
		directoryLockPaths = append(directoryLockPaths, attachmentDirectoryPaths(entry)...)
		artifacts := make([]ArtifactChange, 0, len(entry.Artifacts)+len(directoryLockPaths))
		for _, image := range entry.Artifacts {
			if image.imageKind() == ArtifactImageKindFile && pathWithinAnyDirectory(image.Path, directoryLockPaths) {
				continue
			}
			artifacts = append(artifacts, ArtifactChange{Path: image.Path})
		}
		for _, dir := range directoryLockPaths {
			artifacts = append(artifacts, ArtifactChange{Path: dir})
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

func pathWithinAnyDirectory(path string, dirs []string) bool {
	for _, dir := range dirs {
		if pathWithinArtifactRoot(dir, path) {
			return true
		}
	}
	return false
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

// preflightRestoreOps validates both retained images and every current file or
// directory state before returning phased operations. File writes/deletes are
// emitted first. Directories that should be absent are removed deepest-first;
// directories that should exist are recreated by descendant file writes. Any
// drift aborts the grouped restore before Journal.Prepare stages bytes.
func (l *WorkspaceLedger) preflightRestoreOps(entry LedgerEntry, redo bool) ([]transaction.Op, error) {
	blobs, err := l.validateRestoreImages(entry)
	if err != nil {
		return nil, err
	}

	fileOps := make([]transaction.Op, 0, len(entry.Artifacts))
	directoryDeletes := make(map[string]bool)
	fileDeletes := make(map[string]bool)
	for _, image := range entry.Artifacts {
		op, planned, planErr := preflightRestoreImage(image, redo, blobs, directoryDeletes, fileDeletes)
		if planErr != nil {
			return nil, planErr
		}
		if planned {
			fileOps = append(fileOps, op)
		}
	}

	if entry.LegacySchemaVersion == undoLedgerSchemaVersionV1 {
		legacyDeletes, legacyErr := preflightLegacyAttachmentDirectoryDeletes(legacyAttachmentCleanupCandidates(entry, redo), fileDeletes)
		if legacyErr != nil {
			return nil, legacyErr
		}
		for _, dir := range legacyDeletes {
			directoryDeletes[dir] = true
		}
	}

	return appendDirectoryDeleteOps(fileOps, directoryDeletes), nil
}

func (l *WorkspaceLedger) validateRestoreImages(entry LedgerEntry) (map[string][]byte, error) {
	blobs := make(map[string][]byte)
	seen := make(map[string]bool, len(entry.Artifacts))
	for _, image := range entry.Artifacts {
		path := filepath.Clean(image.Path)
		if seen[path] {
			return nil, fmt.Errorf("storage: ledger entry contains duplicate artifact target %q", path)
		}
		seen[path] = true
		switch image.imageKind() {
		case ArtifactImageKindDirectory:
			if image.BeforeHash != "" || image.AfterHash != "" {
				return nil, fmt.Errorf("storage: ledger directory artifact %q cannot reference file blobs", path)
			}
		case ArtifactImageKindFile:
			if err := l.loadRestoreBlob(blobs, path, "before", image.BeforeExists, image.BeforeHash); err != nil {
				return nil, err
			}
			if err := l.loadRestoreBlob(blobs, path, "after", image.AfterExists, image.AfterHash); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("storage: ledger artifact %q has unknown image kind %q", path, image.Kind)
		}
	}
	return blobs, nil
}

func (l *WorkspaceLedger) loadRestoreBlob(blobs map[string][]byte, path, label string, exists bool, hash string) error {
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

func preflightRestoreImage(
	image ArtifactImage,
	redo bool,
	blobs map[string][]byte,
	directoryDeletes map[string]bool,
	fileDeletes map[string]bool,
) (transaction.Op, bool, error) {
	path := filepath.Clean(image.Path)
	expectedExists, expectedHash := image.AfterExists, image.AfterHash
	desiredExists, desiredHash := image.BeforeExists, image.BeforeHash
	if redo {
		expectedExists, expectedHash = image.BeforeExists, image.BeforeHash
		desiredExists, desiredHash = image.AfterExists, image.AfterHash
	}

	if image.imageKind() == ArtifactImageKindDirectory {
		actualExists, actualKind, err := readDirectoryState(path)
		if err != nil {
			return transaction.Op{}, false, err
		}
		expectedState := "<absent>"
		if expectedExists {
			expectedState = "directory"
		}
		if actualExists != expectedExists || (actualExists && actualKind != "directory") {
			return transaction.Op{}, false, &ArtifactConflictError{Path: path, Expected: expectedState, Actual: actualKind}
		}
		if !desiredExists {
			directoryDeletes[path] = true
		}
		return transaction.Op{}, false, nil
	}

	current, currentExists, actualKind, err := readRegularFileMaybe(path)
	if err != nil {
		return transaction.Op{}, false, fmt.Errorf("reading ledger restore target %q: %w", path, err)
	}
	actualHash := ""
	if currentExists && actualKind == "file" {
		actualHash = RawArtifactRevision(current)
	}
	if currentExists != expectedExists || (currentExists && (actualKind != "file" || actualHash != expectedHash)) {
		if actualKind != "file" {
			actualHash = actualKind
		}
		return transaction.Op{}, false, &ArtifactConflictError{Path: path, Expected: expectedHash, Actual: actualHash}
	}

	if desiredExists {
		return transaction.Op{Kind: transaction.OpWrite, Target: path, Data: blobs[desiredHash], Mode: FilePermissions}, true, nil
	}
	fileDeletes[path] = true
	return transaction.Op{Kind: transaction.OpDelete, Target: path}, true, nil
}

func appendDirectoryDeleteOps(fileOps []transaction.Op, directoryDeletes map[string]bool) []transaction.Op {
	dirs := make([]string, 0, len(directoryDeletes))
	for dir := range directoryDeletes {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool {
		leftDepth := strings.Count(filepath.Clean(dirs[i]), string(filepath.Separator))
		rightDepth := strings.Count(filepath.Clean(dirs[j]), string(filepath.Separator))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return dirs[i] < dirs[j]
	})
	for _, dir := range dirs {
		fileOps = append(fileOps, transaction.Op{Kind: transaction.OpDelete, Target: dir})
	}
	return fileOps
}

func readRegularFileMaybe(path string) ([]byte, bool, string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, "<absent>", nil
	}
	if err != nil {
		return nil, false, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, true, filesystemKind(info), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, true, "file", err
	}
	return data, true, "file", nil
}

func readDirectoryState(path string) (bool, string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, "<absent>", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("stating ledger directory restore target %q: %w", path, err)
	}
	return true, filesystemKind(info), nil
}

func filesystemKind(info os.FileInfo) string {
	switch {
	case info.IsDir():
		return "directory"
	case info.Mode().IsRegular():
		return "file"
	case info.Mode()&os.ModeSymlink != 0:
		return "symlink"
	default:
		return "non-regular"
	}
}

// legacyAttachmentCleanupCandidates identifies task attachment directories that
// a v1 file-only restore may need to remove. Safety is decided later from the
// complete current directory contents while the restore locks are held.
func legacyAttachmentCleanupCandidates(entry LedgerEntry, redo bool) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, image := range entry.Artifacts {
		if image.imageKind() != ArtifactImageKindFile {
			continue
		}
		desiredExists := image.BeforeExists
		if redo {
			desiredExists = image.AfterExists
		}
		if desiredExists {
			continue
		}
		dir := filepath.Dir(filepath.Clean(image.Path))
		if filepath.Base(filepath.Dir(dir)) != attachmentsDirName || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func attachmentDirectoryPaths(entry LedgerEntry) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, image := range entry.Artifacts {
		if image.imageKind() != ArtifactImageKindFile {
			continue
		}
		dir := filepath.Dir(filepath.Clean(image.Path))
		if filepath.Base(filepath.Dir(dir)) != attachmentsDirName || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

// preflightLegacyAttachmentDirectoryDeletes applies the v1 conservative rule:
// remove a task directory only when it currently contains at least one regular
// file and every entry is one of the files this restore already plans to delete.
// Unrelated siblings, symlinks, and nested directories always preserve it.
func preflightLegacyAttachmentDirectoryDeletes(candidates []string, fileDeletes map[string]bool) ([]string, error) {
	var deletes []string
	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading legacy attachment directory %q during restore preflight: %w", dir, err)
		}
		safe := len(entries) > 0
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("stating legacy attachment entry %q during restore preflight: %w", filepath.Join(dir, entry.Name()), err)
			}
			path := filepath.Join(dir, entry.Name())
			if !info.Mode().IsRegular() || !fileDeletes[path] {
				safe = false
				break
			}
		}
		if safe {
			deletes = append(deletes, dir)
		}
	}
	return deletes, nil
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
		if !entryMatchesScope(stack[i], scope) {
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
