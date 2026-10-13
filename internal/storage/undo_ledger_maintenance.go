package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

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

// selectNewestOwned returns the highest-Seq entry in stack matching scope, its
// index, and whether one was found. It is the undo stack's selector.
func selectNewestOwned(stack []LedgerEntry, scope LedgerScope, device string) (LedgerEntry, int, bool) {
	return selectOwned(stack, scope, device, func(candidate, best int) bool { return candidate > best })
}

// selectNextRedoOwned returns the lowest-Seq entry in stack matching scope, its
// index, and whether one was found. An entry keeps its Seq across both stacks
// and undo takes the highest first, so the redo stack's next entry — the last
// one undone — is its lowest: replaying the highest instead would reapply a
// later mutation over a file the earlier one has not been restored to yet.
func selectNextRedoOwned(stack []LedgerEntry, scope LedgerScope, device string) (LedgerEntry, int, bool) {
	return selectOwned(stack, scope, device, func(candidate, best int) bool { return candidate < best })
}

func selectOwned(stack []LedgerEntry, scope LedgerScope, device string, prefer func(candidate, best int) bool) (LedgerEntry, int, bool) {
	best := -1
	for i := range stack {
		if !entryMatchesScope(stack[i], scope) || !stack[i].ownedBy(device) {
			continue
		}
		if best == -1 || prefer(stack[i].Seq, stack[best].Seq) {
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

// Description names a recorded mutation for both history previews and restores.
func (entry LedgerEntry) Description() string {
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
