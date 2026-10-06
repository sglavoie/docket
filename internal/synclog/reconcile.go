package synclog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/transaction"
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
// conflicts under the same content-addressed ids. A transaction that writes
// the key and descends from every competing writer resolves it, so a
// resolution is an ordinary transaction, and two devices resolving the same
// conflict at once either agree, and coalesce, or compete again as a new
// conflict over the two resolutions.

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
// recoverable. Result is what the workspace holds meanwhile.
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

// materialized is the resolved workspace for one set of transactions.
type materialized struct {
	// values holds the digest of every present logical key.
	values map[string]string
	// placement maps every present logical key to its physical path.
	placement map[string]string
	conflicts []Conflict
}

// physical is the materialized workspace by physical path.
func (s *materialized) physical() map[string]string {
	out := make(map[string]string, len(s.values))
	for key, digest := range s.values {
		out[s.placement[key]] = digest
	}
	return out
}

type materializer struct {
	h      *History
	g      *graph
	merger Merger
	blobs  []BlobSource
	cache  map[string][]byte
	memo   map[string]*string
}

func newMaterializer(h *History, g *graph, merger Merger, blobs ...BlobSource) *materializer {
	return &materializer{h: h, g: g, merger: merger, blobs: blobs, cache: map[string][]byte{}, memo: map[string]*string{}}
}

// data returns the bytes behind digest from merged results, the history's
// retained blobs, or the incoming blob sources.
func (mz *materializer) data(digest string) ([]byte, error) {
	if data, ok := mz.cache[digest]; ok {
		return data, nil
	}
	if data, err := mz.h.Blob(digest); err == nil {
		mz.cache[digest] = data
		return data, nil
	}
	for _, source := range mz.blobs {
		if source == nil {
			continue
		}
		if data, err := fetchBlob(source, digest); err == nil {
			mz.cache[digest] = data
			return data, nil
		}
	}
	return nil, fmt.Errorf("%w: blob %s is not available to reconcile", ErrIncomplete, digest)
}

func (mz *materializer) version(digest *string) (Version, error) {
	if digest == nil {
		return Version{}, nil
	}
	data, err := mz.data(*digest)
	if err != nil {
		return Version{}, err
	}
	return Version{Exists: true, Data: data}, nil
}

// value resolves key within set. A non-nil record receives the conflict a
// merge at this level reports; merges of historical bases report none, since
// whatever conflicted there was superseded by the writers above it.
func (mz *materializer) value(key string, set bitset, record *[]Conflict) (*string, error) {
	maximal := mz.g.maximal(key, set)
	switch len(maximal) {
	case 0:
		genesis, ok := mz.g.genesis(key)
		if !ok {
			return nil, nil
		}
		return genesis.base, nil
	case 1:
		return mz.g.writes[maximal[0]][key].value, nil
	}

	memoKey := key + "|" + writerSignature(mz.g, key, set)
	if record == nil {
		if cached, ok := mz.memo[memoKey]; ok {
			return cached, nil
		}
	}
	baseSet := set
	for _, w := range maximal {
		baseSet = baseSet.intersect(mz.g.anc[w])
	}
	base, err := mz.value(key, baseSet, nil)
	if err != nil {
		return nil, err
	}

	writersOf := map[string][]string{}
	for _, w := range maximal {
		v := mz.g.writes[w][key].value
		writersOf[digestOrAbsent(v)] = append(writersOf[digestOrAbsent(v)], mz.g.txns[w].TxnID)
	}
	values := make([]string, 0, len(writersOf))
	for v := range writersOf {
		if v != digestOrAbsent(base) {
			values = append(values, v)
		}
	}
	sort.Strings(values)

	var result *string
	var details []ConflictDetail
	switch {
	case len(values) == 0:
		result = base
	case len(values) == 1:
		result = absentOrDigest(values[0])
	default:
		var present []string
		for _, v := range values {
			if v != "" {
				present = append(present, v)
			}
		}
		if len(present) < len(values) {
			details = append(details, ConflictDetail{Detail: "deleted concurrently with a change; the change is kept"})
		}
		if len(present) == 1 {
			result = absentOrDigest(present[0])
		} else {
			merged, mergeDetails, mergeErr := mz.merge(key, base, present)
			if mergeErr != nil {
				return nil, mergeErr
			}
			result = &merged
			details = append(details, mergeDetails...)
		}
	}
	if record == nil {
		mz.memo[memoKey] = result
	} else if len(details) > 0 {
		conflict := Conflict{
			Format:     ConflictFormat,
			Key:        key,
			Kind:       mz.g.writes[maximal[0]][key].kind,
			ProjectID:  mz.g.writes[maximal[0]][key].projectID,
			BaseBlob:   base,
			ResultBlob: result,
			Details:    details,
		}
		candidates := make([]string, 0, len(writersOf))
		for v := range writersOf {
			candidates = append(candidates, v)
		}
		sort.Strings(candidates)
		for _, v := range candidates {
			writers := append([]string{}, writersOf[v]...)
			sort.Strings(writers)
			conflict.Candidates = append(conflict.Candidates, ConflictCandidate{Blob: absentOrDigest(v), Writers: writers})
		}
		*record = append(*record, conflict)
	}
	return result, nil
}

func (mz *materializer) merge(key string, base *string, present []string) (string, []ConflictDetail, error) {
	if mz.merger == nil {
		return "", nil, fmt.Errorf("%w: %s has concurrent changes and no merger is attached", ErrNeedsMerge, key)
	}
	baseVersion, err := mz.version(base)
	if err != nil {
		return "", nil, err
	}
	req := MergeRequest{Key: key, Base: baseVersion}
	for _, digest := range present {
		v, versionErr := mz.version(&digest)
		if versionErr != nil {
			return "", nil, versionErr
		}
		req.Sides = append(req.Sides, v)
	}
	for _, w := range mz.g.writers[key] {
		req.Kind = mz.g.writes[w][key].kind
		break
	}
	result, err := mz.merger.Merge(req)
	if err != nil {
		return "", nil, fmt.Errorf("reconciling %s: %w", key, err)
	}
	digest := Digest(result.Data)
	mz.cache[digest] = result.Data
	return digest, result.Conflicts, nil
}

// materialize resolves every known key within set, repairs the workspace, and
// places each key.
func (mz *materializer) materialize(set bitset) (*materialized, error) {
	out := &materialized{values: map[string]string{}, placement: map[string]string{}}
	for _, key := range mz.g.keys() {
		v, err := mz.value(key, set, &out.conflicts)
		if err != nil {
			return nil, err
		}
		if v != nil {
			out.values[key] = *v
		}
	}
	catalog, err := mz.catalog(out)
	if err != nil {
		return nil, err
	}
	if catalog, err = mz.restoreProjects(out, set, catalog); err != nil {
		return nil, err
	}
	slugs := map[string]string{}
	if catalog != nil {
		identity, parseErr := parseCatalog(catalog)
		if parseErr != nil {
			return nil, parseErr
		}
		for slug, id := range identity.slugs {
			slugs[id] = slug
		}
	}
	for key := range out.values {
		out.placement[key] = mz.place(key, set, slugs)
	}
	for i := range out.conflicts {
		conflict := &out.conflicts[i]
		if err := conflict.seal(); err != nil {
			return nil, err
		}
		conflict.Path = mz.place(conflict.Key, set, slugs)
		if conflict.Kind == EffectCatalog {
			// A restored project is the catalog's conflict: the catalog is what
			// resolving it writes.
			conflict.Path = catalogRel
		}
	}
	sort.Slice(out.conflicts, func(i, j int) bool { return out.conflicts[i].ID < out.conflicts[j].ID })
	return out, nil
}

// catalog is the resolved catalog's bytes, read from disk when no known
// transaction has written it.
func (mz *materializer) catalog(state *materialized) ([]byte, error) {
	if digest, ok := state.values[catalogRel]; ok {
		return mz.data(digest)
	}
	if _, written := mz.g.writers[catalogRel]; written {
		return nil, nil
	}
	data, err := os.ReadFile(mz.h.abs(catalogRel))
	if isNotExist(err) {
		return nil, nil
	}
	return data, err
}

// restoreProjects names again every project whose files survived while the
// resolved catalog dropped it: a deletion never beats a change. The entry is
// taken from the catalog the deleting transaction replaced.
func (mz *materializer) restoreProjects(state *materialized, set bitset, catalog []byte) ([]byte, error) {
	if catalog == nil {
		return nil, nil
	}
	identity, err := parseCatalog(catalog)
	if err != nil {
		return nil, err
	}
	named := map[string]bool{}
	for _, id := range identity.slugs {
		named[id] = true
	}
	orphaned := map[string]bool{}
	for key := range state.values {
		if projectID, _, ok := projectKeyParts(key); ok && !named[projectID] {
			orphaned[projectID] = true
		}
	}
	projectIDs := make([]string, 0, len(orphaned))
	for id := range orphaned {
		projectIDs = append(projectIDs, id)
	}
	sort.Strings(projectIDs)
	for _, projectID := range projectIDs {
		deleter, from, findErr := mz.projectDeletion(set, projectID)
		if findErr != nil {
			return nil, findErr
		}
		if from == nil || mz.merger == nil {
			continue
		}
		before := Digest(catalog)
		restored, ok, restoreErr := mz.merger.RestoreProject(catalog, projectID, from)
		if restoreErr != nil {
			return nil, fmt.Errorf("restoring project %s to the catalog: %w", projectID, restoreErr)
		}
		if !ok {
			continue
		}
		catalog = restored
		digest := Digest(restored)
		mz.cache[digest] = restored
		state.values[catalogRel] = digest
		fromDigest := Digest(from)
		state.conflicts = append(state.conflicts, Conflict{
			Format:     ConflictFormat,
			Key:        "project/" + projectID,
			Kind:       EffectCatalog,
			BaseBlob:   &fromDigest,
			Candidates: []ConflictCandidate{{Blob: &before, Writers: []string{deleter}}},
			ResultBlob: &digest,
			Details: []ConflictDetail{{
				Subject: projectID, Field: "project",
				Detail: "the project was deleted concurrently with changes to its files; the project is kept",
			}},
		})
	}
	return catalog, nil
}

// projectDeletion finds the catalog write inside set that removed projectID:
// the smallest txn id among them, and the catalog bytes it replaced.
func (mz *materializer) projectDeletion(set bitset, projectID string) (string, []byte, error) {
	var candidates []int
	for _, w := range mz.g.writers[catalogRel] {
		if set.has(w) {
			candidates = append(candidates, w)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return mz.g.txns[candidates[i]].TxnID < mz.g.txns[candidates[j]].TxnID })
	for _, w := range candidates {
		write := mz.g.writes[w][catalogRel]
		if write.base == nil {
			continue
		}
		before, err := mz.data(*write.base)
		if err != nil {
			return "", nil, err
		}
		if !catalogNames(before, projectID) {
			continue
		}
		if write.value != nil {
			after, err := mz.data(*write.value)
			if err != nil {
				return "", nil, err
			}
			if catalogNames(after, projectID) {
				continue
			}
		}
		return mz.g.txns[w].TxnID, before, nil
	}
	return "", nil, nil
}

func catalogNames(data []byte, projectID string) bool {
	identity, err := parseCatalog(data)
	if err != nil {
		return false
	}
	for _, id := range identity.slugs {
		if id == projectID {
			return true
		}
	}
	return false
}

// place is a key's physical path: a project-scoped key sits under the slug the
// resolved catalog gives its project, or, for a project no catalog names,
// under the slug its latest writer used.
func (mz *materializer) place(key string, set bitset, slugs map[string]string) string {
	projectID, rest, ok := projectKeyParts(key)
	if !ok {
		return key
	}
	slug := slugs[projectID]
	if slug == "" {
		path := ""
		for _, w := range mz.g.writers[key] {
			if set.has(w) {
				path = mz.g.writes[w][key].path
			}
		}
		if path == "" {
			if genesis, found := mz.g.genesis(key); found {
				path = genesis.path
			}
		}
		parts := strings.SplitN(path, "/", 3)
		if len(parts) >= 2 {
			slug = parts[1]
		}
	}
	return "projects/" + slug + "/" + rest
}

func writerSignature(g *graph, key string, set bitset) string {
	var b strings.Builder
	for _, w := range g.writers[key] {
		if set.has(w) {
			fmt.Fprintf(&b, "%d,", w)
		}
	}
	return b.String()
}

func digestOrAbsent(d *string) string {
	if d == nil {
		return ""
	}
	return *d
}

func absentOrDigest(v string) *string {
	if v == "" {
		return nil
	}
	return digestPtr(v)
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

// planWrites stages the workspace writes and deletions that turn the resolved
// workspace before into after, refusing a touched file whose bytes are not
// what before says this device holds. It retains every replaced and written
// blob, tracks the written digests, and records each touched path's expected
// bytes and every directory a write needs.
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
		oldDigest, hadOld := oldFiles[rel]
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
