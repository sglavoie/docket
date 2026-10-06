package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/tt/internal/transaction"
)

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
		Device:        l.device(),
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
	service := NewCommitService(l.transactionsDir, l.roots).OnMachine(l.machineID)
	return service.withWorkspaceCommitLock(func() error {
		journal := service.journal()
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
