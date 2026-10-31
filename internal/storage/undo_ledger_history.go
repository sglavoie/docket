package storage

import (
	"cmp"
	"encoding/hex"
	"fmt"
	"slices"
)

// LedgerHistory is a read-only snapshot of this device's workspace history.
// Undo is newest first; Redo is in the order the redo key will replay it.
type LedgerHistory struct {
	Undo []LedgerEntry
	Redo []LedgerEntry
}

func (l *WorkspaceLedger) History(scope LedgerScope) (LedgerHistory, error) {
	lf, err := l.load()
	if err != nil {
		return LedgerHistory{}, err
	}
	copyStack := func(stack []LedgerEntry, ascending bool) []LedgerEntry {
		var entries []LedgerEntry
		for _, entry := range stack {
			if entryMatchesScope(entry, scope) && entry.ownedBy(l.device()) {
				entry.ProjectIDs = slices.Clone(entry.ProjectIDs)
				entry.Artifacts = slices.Clone(entry.Artifacts)
				entries = append(entries, entry)
			}
		}
		slices.SortFunc(entries, func(a, b LedgerEntry) int {
			if ascending {
				return cmp.Compare(a.Seq, b.Seq)
			}
			return cmp.Compare(b.Seq, a.Seq)
		})
		return entries
	}
	return LedgerHistory{Undo: copyStack(lf.Entries, false), Redo: copyStack(lf.RedoEntries, true)}, nil
}

// HistoryArtifacts reads checked, retained images for an entry still in this
// device's history. It never opens the live artifacts or attempts a restore.
// Missing/corrupt images fail visibly without retiring the history entry.
func (l *WorkspaceLedger) HistoryArtifacts(transactionID string) ([]LedgerArtifact, error) {
	history, err := l.History(GlobalScope())
	if err != nil {
		return nil, err
	}
	for _, entry := range append(history.Undo, history.Redo...) {
		if entry.TransactionID != transactionID {
			continue
		}
		for _, image := range entry.Artifacts {
			if !pathWithinApprovedRoots(image.Path, l.roots) {
				return nil, fmt.Errorf("history artifact is outside the workspace: %s", image.Path)
			}
			for _, hash := range []string{image.BeforeHash, image.AfterHash} {
				if hash == "" {
					continue
				}
				decoded, decodeErr := hex.DecodeString(hash)
				if decodeErr != nil || len(decoded) != 32 {
					return nil, fmt.Errorf("history artifact has an invalid retained image hash")
				}
			}
		}
		blobs, err := l.validateRestoreImages(entry)
		if err != nil {
			return nil, err
		}
		images := make([]LedgerArtifact, 0, len(entry.Artifacts))
		for _, image := range entry.Artifacts {
			images = append(images, LedgerArtifact{
				Kind: image.imageKind(), Path: image.Path,
				BeforeExists: image.BeforeExists, Before: slices.Clone(blobs[image.BeforeHash]),
				AfterExists: image.AfterExists, After: slices.Clone(blobs[image.AfterHash]),
			})
		}
		return images, nil
	}
	return nil, fmt.Errorf("this change is no longer in retained history")
}

// RedoTransactionWithEntry binds redo to the action the browser advertised.
// The existing restore lock checks the identity before making any changes.
func (l *WorkspaceLedger) RedoTransactionWithEntry(scope LedgerScope, transactionID string) (LedgerEntry, string, error) {
	return l.restore(scope, true, transactionID)
}
