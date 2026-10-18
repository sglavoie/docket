package synclog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

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
	memo   map[string]resolution
}

// resolution is one key's resolved value within one set of transactions and
// the conflict, unsealed, that merging its maximal writers left, if any.
type resolution struct {
	value    *string
	conflict *Conflict
}

func newMaterializer(h *History, g *graph, merger Merger, blobs ...BlobSource) *materializer {
	return &materializer{h: h, g: g, merger: merger, blobs: blobs, cache: map[string][]byte{}, memo: map[string]resolution{}}
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

// resolve resolves key within an ancestor-closed set, memoized by the key's
// maximal writers inside set, since the result depends only on the key's
// writers inside set and, the set being ancestor-closed, those are exactly the
// maximal writers and the writers they descend from. The conflict it reports
// is the one merging the current maximal writers leaves; openConflicts finds
// the ones earlier merges left.
func (mz *materializer) resolve(key string, set bitset) (resolution, error) {
	maximal := mz.g.maximal(key, set)
	switch len(maximal) {
	case 0:
		genesis, ok := mz.g.genesis(key)
		if !ok {
			return resolution{}, nil
		}
		return resolution{value: genesis.base}, nil
	case 1:
		return resolution{value: mz.g.writes[maximal[0]][key].value}, nil
	}

	memoKey := key + "|" + maximalSignature(maximal)
	if cached, ok := mz.memo[memoKey]; ok {
		return cached, nil
	}
	baseSet := set
	for _, w := range maximal {
		baseSet = baseSet.intersect(mz.g.anc[w])
	}
	baseResolution, err := mz.resolve(key, baseSet)
	if err != nil {
		return resolution{}, err
	}
	base := baseResolution.value

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
				return resolution{}, mergeErr
			}
			result = &merged
			details = append(details, mergeDetails...)
		}
	}
	out := resolution{value: result}
	if len(details) > 0 {
		conflict := &Conflict{
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
		out.conflict = conflict
	}
	mz.memo[memoKey] = out
	return out, nil
}

// openConflicts is every conflict on key that set leaves open: the conflict
// of its current maximal writers, plus the conflict of every merge a writer
// inside set was written over, unless a resolution inside set that writes the
// key descends from each of that conflict's competing writers. A carried
// conflict reports current — the key's value within set — as its result, since
// that is what the workspace holds meanwhile.
func (mz *materializer) openConflicts(key string, set bitset, current *string, top *Conflict) ([]Conflict, error) {
	var out []Conflict
	if top != nil {
		out = append(out, *top)
	}
	seen := map[string]bool{}
	var resolutions []int
	for _, w := range mz.g.writers[key] {
		if set.has(w) && mz.g.txns[w].Kind == KindResolution {
			resolutions = append(resolutions, w)
		}
	}
	for i, w := range mz.g.writers[key] {
		if !set.has(w) || !mz.g.mergesPredecessors(key, i) {
			continue
		}
		before := append(bitset{}, mz.g.anc[w]...)
		before[w/64] &^= 1 << (uint(w) % 64)
		r, err := mz.resolve(key, before)
		if err != nil {
			return nil, err
		}
		if r.conflict == nil || mz.resolved(r.conflict, resolutions) {
			continue
		}
		carried := *r.conflict
		carried.ResultBlob = current
		identity, err := json.Marshal(carried)
		if err != nil {
			return nil, err
		}
		if seen[string(identity)] {
			continue
		}
		seen[string(identity)] = true
		out = append(out, carried)
	}
	return out, nil
}

// resolved reports whether one of resolutions, which are in topological order,
// strictly descends from every writer competing in conflict. Only a resolution
// ordered after the newest of those writers can, and the newest resolutions
// are the likeliest to, so they are asked first.
func (mz *materializer) resolved(conflict *Conflict, resolutions []int) bool {
	var writers []int
	newest := -1
	for _, candidate := range conflict.Candidates {
		for _, writer := range candidate.Writers {
			w, ok := mz.g.index[writer]
			if !ok {
				return false
			}
			writers = append(writers, w)
			newest = max(newest, w)
		}
	}
	for i := len(resolutions) - 1; i >= 0 && resolutions[i] > newest; i-- {
		anc := mz.g.anc[resolutions[i]]
		all := true
		for _, w := range writers {
			if !anc.has(w) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
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

// maximalSignature names an ancestor-closed set by its maximal writers of one
// key, which decide every writer of that key the set holds.
func maximalSignature(maximal []int) string {
	var b strings.Builder
	for _, w := range maximal {
		b.WriteString(strconv.Itoa(w))
		b.WriteByte(',')
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
