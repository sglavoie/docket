package synclog

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// PlanBootstrap rebuilds an empty working copy from a complete, verified
// history. It uses the same materializer as PlanReconcile, but derives the
// final workspace once instead of reconstructing every historical prefix.
// All retained content, receipts, indexes, conflicts and final files belong
// to one journal transaction. Nil means this is not an empty working copy;
// an incomplete or incompatible history must use ordinary per-item import.
func (h *History) PlanBootstrap(manifests []*Manifest, blobs BlobSource, merger Merger, localTxnID string) (*ReconcilePlan, error) {
	state, err := h.LoadState()
	if err != nil || state != nil {
		return nil, err
	}
	// A missing device state does not prove there is no history: another
	// author or a retained branch may already have indexed transactions.
	// Ordinary reconciliation must include those records, even if all their
	// working files are absent. Bootstrap is only for a new history.
	if _, err := os.Lstat(filepath.Join(h.Dir(), "index")); err == nil {
		return nil, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	occupied := false
	if err := h.walkSyncedFiles(func(_, _ string, _ fs.FileInfo) error {
		occupied = true
		return nil
	}); err != nil {
		return nil, err
	}
	if occupied || len(manifests) == 0 {
		return nil, nil
	}

	state, err = h.bootstrapState(manifests)
	if err != nil {
		return nil, err
	}
	g, err := buildGraph(manifests)
	if err != nil {
		return nil, err
	}
	if !bootstrapAdmissible(g.txns) {
		return nil, fmt.Errorf("%w: transaction parents and author sequences cannot be applied in order", ErrIncomplete)
	}
	mz := newMaterializer(h, g, merger, blobs)
	o := newOverlay(h)
	// Retain every historical base and value, including superseded content.
	// Verifying only the final files would admit an incomplete history and
	// lose the versions needed for subsequent merges and conflict inspection.
	for _, m := range g.txns {
		for _, effect := range m.Effects {
			for _, digest := range []*string{effect.BaseBlob, effect.Blob} {
				if digest == nil || o.planned[*digest] {
					continue
				}
				data, err := mz.data(*digest)
				if err != nil {
					return nil, err
				}
				o.retainBytes(*digest, data)
			}
		}
	}
	after, err := mz.materialize(g.all())
	if err != nil {
		return nil, err
	}
	plan := &ReconcilePlan{Expect: map[string]*string{}}
	ops, err := h.planWrites(o, mz, &materialized{}, after, state, plan)
	if err != nil {
		return nil, err
	}
	conflicts, conflictIDs, err := h.conflictRecords(after.conflicts)
	if err != nil {
		return nil, err
	}
	ops = append(ops, o.blobOps...)
	ops = append(ops, conflicts...)
	for _, m := range g.txns {
		receiptPath := h.receiptPath(m.DeviceID, m.DeviceSeq, m.TxnID)
		receipt, err := encodeJSON(Receipt{
			Format: ReceiptFormat, TxnID: m.TxnID, LocalTxnID: localTxnID,
			ReceivedAt: h.now().UTC().Format(time.RFC3339Nano), Manifest: *m,
			Conflicts: conflictIDs,
		})
		if err != nil {
			return nil, err
		}
		record, err := immutableOp(receiptPath, receipt)
		if err != nil {
			return nil, err
		}
		index, err := encodeJSON(IndexEntry{
			Format: IndexFormat, TxnID: m.TxnID, DeviceID: m.DeviceID,
			DeviceSeq: m.DeviceSeq, Origin: "incoming", Record: h.rel(receiptPath),
		})
		if err != nil {
			return nil, err
		}
		indexOp, err := immutableOp(h.indexPath(m.TxnID), index)
		if err != nil {
			return nil, err
		}
		ops = append(ops, record, indexOp)
		state.Heads = mergeHeads(state.Heads, m.Parents, m.TxnID)
	}
	plan.Ops, err = h.appendRecords(ops, state, nil)
	return plan, err
}

// bootstrapState admits a complete history without intermediate disk writes.
// Sequence continuity is checked independently of graph order: unrelated
// authors need not have sorted before the enrollment genesis in the inbox.
func (h *History) bootstrapState(manifests []*Manifest) (*State, error) {
	var state *State
	seen := map[string]bool{}
	sequences := map[string][]uint64{}
	for _, m := range manifests {
		if m == nil {
			return nil, fmt.Errorf("%w: no manifest", ErrInvalidManifest)
		}
		if err := m.Validate(); err != nil {
			return nil, err
		}
		if err := h.readable(m); err != nil {
			return nil, err
		}
		if m.DeviceID == h.deviceID {
			return nil, fmt.Errorf("%w: %s", ErrOwnTransaction, m.TxnID)
		}
		if seen[m.TxnID] {
			return nil, fmt.Errorf("%w: duplicate transaction %s", ErrInvalidManifest, m.TxnID)
		}
		seen[m.TxnID] = true
		sequences[m.DeviceID] = append(sequences[m.DeviceID], m.DeviceSeq)
		if m.Kind == KindEnroll && m.DeviceSeq == 1 && len(m.Parents) == 0 {
			state = h.newState(m.WorkspaceID)
			state.Indexed = true
		}
	}
	if state == nil {
		return nil, fmt.Errorf("%w: the workspace genesis has not arrived", ErrIncomplete)
	}
	for _, m := range manifests {
		if m.WorkspaceID != state.WorkspaceID {
			return nil, ErrForeignWorkspace
		}
	}
	for device, seqs := range sequences {
		sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
		for i, seq := range seqs {
			if seq != uint64(i+1) {
				return nil, fmt.Errorf("%w: device %s has a gap or reused sequence", ErrIncomplete, device)
			}
		}
		state.Received[device] = uint64(len(seqs))
	}
	return state, nil
}

// Parent order alone is insufficient: an author sequence can depend on a
// later sequence, even indirectly through another author. Match admission's
// two prerequisites before acknowledging any of the history.
func bootstrapAdmissible(manifests []*Manifest) bool {
	known := map[string]bool{}
	received := map[string]uint64{}
	for len(known) < len(manifests) {
		before := len(known)
		for _, m := range manifests {
			if known[m.TxnID] || m.DeviceSeq != received[m.DeviceID]+1 {
				continue
			}
			ready := true
			for _, parent := range m.Parents {
				if !known[parent] {
					ready = false
					break
				}
			}
			if ready {
				known[m.TxnID] = true
				received[m.DeviceID] = m.DeviceSeq
			}
		}
		if len(known) == before {
			return false
		}
	}
	return true
}
