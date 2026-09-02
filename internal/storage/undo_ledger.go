package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
