package synclog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/docket/internal/transaction"
)

// Reconciliation makes a device's workspace a pure function of the set of
// transactions it holds, so devices holding the same transactions hold the
// same bytes and the same conflicts whatever order those transactions arrived
// in.
//
// Each logical key (see LogicalKey) is resolved on its own. The writers of a
// key that nothing else writing it descends from are its maximal writers. One
// maximal writer is a causal fast-forward: its value stands. Several are
// concurrent: their values are merged against the key's value at the
// intersection of their histories, which is itself resolved the same way, so
// criss-cross histories with several common ancestors need no arbitrary pick
// of one ancestor. Identical values coalesce and values equal to the base
// changed nothing. A deletion never beats a change: the change is kept and a
// conflict recorded. Two or more changes are merged by the kind's Merger,
// which keeps the content valid and reports what it could not merge.
//
// The workspace is then repaired as a whole: a project whose files survive
// because a change beat its deletion is named in the catalog again. Finally
// every project-scoped key is placed under the slug the resolved catalog gives
// its project, which is what lets a rename on one device and an edit on
// another land as one file.
//
// A conflict is never decided by a timestamp and never published. It is
// derived: every device holding the same transactions derives the same
// conflicts under the same content-addressed ids. A conflict is not only the
// merge of the key's current maximal writers: every merge a writer of the key
// was written over — the merge of the maximal writers just before it — left
// its conflict too, and that conflict stays open until a transaction of kind
// resolution that writes the key descends from every competing writer. An
// ordinary edit made on top of a merge, even of another task in the same
// document, therefore never closes a conflict and never drops the losing
// version: the conflict stays open with the workspace's current value as its
// result. A resolution is an ordinary transaction otherwise, and two devices
// resolving the same conflict at once either agree, and coalesce, or compete
// again as a new conflict over the two resolutions.

// ConflictFormat identifies a stored conflict record.
const ConflictFormat = "tt-sync-conflict"

// Version is one value of a logical key: absent, or its bytes.
type Version struct {
	Exists bool
	Data   []byte
}

// MergeRequest asks a Merger to combine two or more distinct changes to one
// key. Base is the key's value at the changes' common history and may be
// absent. Sides are present, distinct, differ from Base, and are ordered by
// digest, so Sides[0] is the same primary version on every device.
type MergeRequest struct {
	Kind  EffectKind
	Key   string
	Base  Version
	Sides []Version
}

// MergeResult is a Merger's combined value and what it could not merge.
type MergeResult struct {
	Data      []byte
	Conflicts []ConflictDetail
}

// ConflictDetail is one part of a key a merge could not reconcile on its own.
type ConflictDetail struct {
	Subject string `json:"subject,omitempty"`
	Field   string `json:"field,omitempty"`
	Detail  string `json:"detail"`
}

// Merger combines concurrent versions of synced artifacts. Both methods must
// be pure functions of their arguments: reconciliation converges only if every
// device's Merger returns the same bytes for the same inputs.
type Merger interface {
	// Merge returns a valid artifact combining req.Sides against req.Base.
	Merge(req MergeRequest) (MergeResult, error)
	// RestoreProject returns catalog naming projectID again, its entry taken
	// from the catalog from, and whether anything was restored.
	RestoreProject(catalog []byte, projectID string, from []byte) ([]byte, bool, error)
}

// Conflict is a derived, content-addressed record of competing changes to one
// key. Candidates name every competing value and the transactions that wrote
// it; their bytes stay retained in the history, so each competing edit remains
// recoverable. ResultBlob is the history's materialized result; WorkspaceConflicts
// replaces it with the observed working-file digest for inspection and resolution.
type Conflict struct {
	Format     string              `json:"format"`
	ID         string              `json:"id"`
	Key        string              `json:"key"`
	Kind       EffectKind          `json:"kind"`
	ProjectID  string              `json:"project_id,omitempty"`
	Path       string              `json:"path"`
	BaseBlob   *string             `json:"base_blob"`
	Candidates []ConflictCandidate `json:"candidates"`
	ResultBlob *string             `json:"result_blob"`
	Details    []ConflictDetail    `json:"details"`
}

// ConflictCandidate is one competing value and the maximal writers that left
// it. A nil Blob is a deletion.
type ConflictCandidate struct {
	Blob    *string  `json:"blob"`
	Writers []string `json:"writers"`
}

// seal stamps the conflict's id: the digest of everything but the id and the
// path, since a later rename moves where a conflict sits without changing it.
func (c *Conflict) seal() error {
	copied := *c
	copied.ID = ""
	copied.Path = ""
	data, err := json.Marshal(copied)
	if err != nil {
		return err
	}
	c.ID = Digest(data)
	return nil
}

// ReconcilePlan is an incoming transaction reconciled with this device's
// history, ready to stage.
type ReconcilePlan struct {
	Plan
	// Expect is the digest each touched workspace path must still hold (nil:
	// absent) when the plan commits; anything else is a change made since.
	Expect map[string]*string
}

// ErrUnrecordedChange marks a workspace file whose bytes are not what the
// history says this device last wrote there. Reconciling over it would lose
// the change, so the incoming transaction waits until the change is recorded.
var ErrUnrecordedChange = fmt.Errorf("%w: a workspace file has changes the sync history has not recorded", ErrNeedsMerge)

// PlanReconcile plans applying another device's transaction by reconciling
// it with everything this device holds, together with its receipt. It admits
// exactly what PlanIncoming admits, but a transaction concurrent with local
// changes is merged rather than refused: the workspace becomes the resolved
// workspace of every transaction held, and only the files that resolution
// changes are written. The receipt names the conflicts left open, and each
// conflict's record is stored beside the history.
func (h *History) PlanReconcile(m *Manifest, blobs BlobSource, merger Merger, localTxnID string) (*ReconcilePlan, error) {
	state, err := h.admit(m)
	if err != nil {
		return nil, err
	}
	for _, effect := range m.Effects {
		if effect.Blob != nil {
			if _, fetchErr := fetchBlob(blobs, *effect.Blob); fetchErr != nil {
				return nil, fetchErr
			}
		}
	}
	g, err := h.loadGraph(m)
	if err != nil {
		return nil, err
	}
	mz := newMaterializer(h, g, merger, blobs)
	before, err := mz.materialize(g.without(m.TxnID))
	if err != nil {
		return nil, err
	}
	after, err := mz.materialize(g.all())
	if err != nil {
		return nil, err
	}

	o := newOverlay(h)
	if err = h.baseline(state, o); err != nil {
		return nil, err
	}
	plan := &ReconcilePlan{Expect: map[string]*string{}}
	effectOps, err := h.planWrites(o, mz, before, after, state, plan)
	if err != nil {
		return nil, err
	}
	for _, effect := range m.Effects {
		for _, digest := range []*string{effect.BaseBlob, effect.Blob} {
			if digest == nil || o.planned[*digest] {
				continue
			}
			if data, dataErr := mz.data(*digest); dataErr == nil {
				o.retainBytes(*digest, data)
			}
		}
	}

	conflictOps, conflictIDs, err := h.conflictRecords(after.conflicts)
	if err != nil {
		return nil, err
	}
	receiptPath := h.receiptPath(m.DeviceID, m.DeviceSeq, m.TxnID)
	receipt, err := encodeJSON(Receipt{
		Format:     ReceiptFormat,
		TxnID:      m.TxnID,
		LocalTxnID: localTxnID,
		ReceivedAt: h.now().UTC().Format(time.RFC3339Nano),
		Manifest:   *m,
		Conflicts:  conflictIDs,
	})
	if err != nil {
		return nil, err
	}
	receiptOp, err := immutableOp(receiptPath, receipt)
	if err != nil {
		return nil, err
	}
	state.Received[m.DeviceID] = m.DeviceSeq
	state.Heads = mergeHeads(state.Heads, m.Parents, m.TxnID)

	ops := append([]transaction.Op{}, effectOps...)
	ops = append(ops, o.blobOps...)
	ops = append(ops, conflictOps...)
	ops = append(ops, receiptOp)
	ops, err = h.appendRecords(ops, state, &IndexEntry{
		Format:    IndexFormat,
		TxnID:     m.TxnID,
		DeviceID:  m.DeviceID,
		DeviceSeq: m.DeviceSeq,
		Origin:    "incoming",
		Record:    h.rel(receiptPath),
	})
	if err != nil {
		return nil, err
	}
	plan.Ops = ops
	return plan, nil
}

// planWrites brings the paths named by the resolved history to their after
// values, checking writes against this device's tracked workspace. Retained
// branches can make before differ from what this device actually committed;
// treating that reconstructed value as the disk precondition would strand
// recorded work. Paths unchanged between before and after still need applying
// when their tracked value differs. Paths outside both histories are left alone.
// Every replaced and written blob is retained, and every write is guarded by
// the tracked digest so an unrecorded edit is still refused.
func (h *History) planWrites(o *overlay, mz *materializer, before, after *materialized, state *State, plan *ReconcilePlan) ([]transaction.Op, error) {
	oldFiles, newFiles := before.physical(), after.physical()
	paths := make([]string, 0, len(oldFiles)+len(newFiles))
	for rel := range oldFiles {
		paths = append(paths, rel)
	}
	for rel := range newFiles {
		if _, ok := oldFiles[rel]; !ok {
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)
	var effectOps []transaction.Op
	dirs := map[string]bool{}
	for _, rel := range paths {
		oldDigest, hadOld := state.Tracked[rel]
		newDigest, hasNew := newFiles[rel]
		if hadOld == hasNew && oldDigest == newDigest {
			continue
		}
		if _, _, local, classifyErr := Classify(rel); classifyErr != nil || local {
			return nil, fmt.Errorf("%w: reconciliation would write %s", ErrInvalidManifest, rel)
		}
		abs := h.abs(rel)
		current, isDir, lookupErr := o.lookup(abs)
		if lookupErr != nil {
			return nil, lookupErr
		}
		var expect *string
		if hadOld {
			expect = digestPtr(oldDigest)
		}
		if isDir || !baseMatches(current, expect) {
			return nil, fmt.Errorf("%w: %s", ErrUnrecordedChange, rel)
		}
		if current.exists {
			o.retain(current)
		}
		plan.Expect[abs] = expect
		if !hasNew {
			effectOps = append(effectOps, transaction.Op{Kind: transaction.OpDelete, Target: abs})
			delete(state.Tracked, rel)
			continue
		}
		data, dataErr := mz.data(newDigest)
		if dataErr != nil {
			return nil, dataErr
		}
		o.retainBytes(newDigest, data)
		effectOps = append(effectOps, writeOp(abs, data))
		dirs[filepath.Dir(abs)] = true
		state.Tracked[rel] = newDigest
	}
	for dir := range dirs {
		plan.MoveTargets = append(plan.MoveTargets, dir)
	}
	sort.Strings(plan.MoveTargets)
	return effectOps, nil
}

// conflictPath is where a conflict's record is kept.
func (h *History) conflictPath(id string) string {
	hexID := digestHex(id)
	return filepath.Join(h.Dir(), "conflicts", hexID[:2], hexID+".json")
}

// conflictRecords stages a record for every conflict not stored yet. Records
// are content addressed, so an existing one is already this record.
func (h *History) conflictRecords(conflicts []Conflict) ([]transaction.Op, []string, error) {
	var ops []transaction.Op
	ids := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		ids = append(ids, conflict.ID)
		path := h.conflictPath(conflict.ID)
		if _, err := os.Lstat(path); err == nil {
			continue
		}
		data, err := encodeJSON(conflict)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, writeOp(path, data))
	}
	return ops, ids, nil
}

// OpenConflicts derives the conflicts this device's history leaves open: the
// same set, under the same ids, on every device holding the same transactions.
func (h *History) OpenConflicts(merger Merger) ([]Conflict, error) {
	g, err := h.loadGraph(nil)
	if err != nil {
		return nil, err
	}
	state, err := newMaterializer(h, g, merger).materialize(g.all())
	if err != nil {
		return nil, err
	}
	return state.conflicts, nil
}

// WorkspaceConflicts binds the derived conflicts to the files a user can
// actually inspect and resolve. A local edit can leave the working file at a
// candidate while materialization computes a different, unretained merge. That
// synthetic result is neither a readable current version nor a valid write
// precondition. Include the observed file digest in the conflict ID so changes
// after inspection are still refused, including edits made outside Docket.
// OpenConflicts remains the deterministic history-only projection.
func (h *History) WorkspaceConflicts(merger Merger) ([]Conflict, error) {
	conflicts, err := h.OpenConflicts(merger)
	if err != nil {
		return nil, err
	}
	current := make(map[string]*string)
	for i := range conflicts {
		conflict := &conflicts[i]
		digest, read := current[conflict.Path]
		if !read {
			data, readErr := os.ReadFile(h.WorkspacePath(conflict.Path))
			switch {
			case os.IsNotExist(readErr):
			case readErr != nil:
				return nil, fmt.Errorf("reading current conflict content: %w", readErr)
			default:
				digest = digestPtr(Digest(data))
			}
			current[conflict.Path] = digest
		}
		conflict.ResultBlob = digest
		if err := conflict.seal(); err != nil {
			return nil, err
		}
	}
	return conflicts, nil
}
