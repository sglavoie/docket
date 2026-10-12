package synclog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// derivationOracle is conflict derivation as it stood before the predecessor
// scan: every maximal set is found by comparing each writer of a key against
// every other, once per writer. It is quadratic to cubic in a key's writers and
// is kept, verbatim, only as the reference the scan is held to.
type derivationOracle struct {
	mz   *materializer
	memo map[string]resolution
}

func newDerivationOracle(mz *materializer) *derivationOracle {
	return &derivationOracle{mz: mz, memo: map[string]resolution{}}
}

func oracleMaximal(g *graph, key string, set bitset) []int {
	var inside []int
	for _, w := range g.writers[key] {
		if set.has(w) {
			inside = append(inside, w)
		}
	}
	var out []int
	for _, w := range inside {
		dominated := false
		for _, other := range inside {
			if other != w && g.anc[other].has(w) {
				dominated = true
				break
			}
		}
		if !dominated {
			out = append(out, w)
		}
	}
	return out
}

func oracleMaximalRoots(g *graph, key string) []int {
	var out []int
	for _, w := range g.writers[key] {
		root := true
		for _, other := range g.writers[key] {
			if other != w && g.anc[w].has(other) {
				root = false
				break
			}
		}
		if root {
			out = append(out, w)
		}
	}
	return out
}

func oracleGenesis(g *graph, key string) (keyWrite, bool) {
	roots := oracleMaximalRoots(g, key)
	if len(roots) == 0 {
		return keyWrite{}, false
	}
	sort.Slice(roots, func(i, j int) bool { return g.txns[roots[i]].TxnID < g.txns[roots[j]].TxnID })
	return g.writes[roots[0]][key], true
}

func oracleWriterSignature(g *graph, key string, set bitset) string {
	var b strings.Builder
	for _, w := range g.writers[key] {
		if set.has(w) {
			fmt.Fprintf(&b, "%d,", w)
		}
	}
	return b.String()
}

func (o *derivationOracle) resolve(key string, set bitset) (resolution, error) {
	mz := o.mz
	maximal := oracleMaximal(mz.g, key, set)
	switch len(maximal) {
	case 0:
		genesis, ok := oracleGenesis(mz.g, key)
		if !ok {
			return resolution{}, nil
		}
		return resolution{value: genesis.base}, nil
	case 1:
		return resolution{value: mz.g.writes[maximal[0]][key].value}, nil
	}

	memoKey := key + "|" + oracleWriterSignature(mz.g, key, set)
	if cached, ok := o.memo[memoKey]; ok {
		return cached, nil
	}
	baseSet := set
	for _, w := range maximal {
		baseSet = baseSet.intersect(mz.g.anc[w])
	}
	baseResolution, err := o.resolve(key, baseSet)
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
	o.memo[memoKey] = out
	return out, nil
}

func (o *derivationOracle) openConflicts(key string, set bitset, current *string, top *Conflict) ([]Conflict, error) {
	mz := o.mz
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
	for _, w := range mz.g.writers[key] {
		if !set.has(w) {
			continue
		}
		before := append(bitset{}, mz.g.anc[w]...)
		before[w/64] &^= 1 << (uint(w) % 64)
		if len(oracleMaximal(mz.g, key, before)) < 2 {
			continue
		}
		r, err := o.resolve(key, before)
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

// materialize is materializer.materialize over the oracle's resolve,
// openConflicts and genesis. The catalog repair it shares with the materializer
// reads no maximal set.
func (o *derivationOracle) materialize(set bitset) (*materialized, error) {
	mz := o.mz
	out := &materialized{values: map[string]string{}, placement: map[string]string{}}
	for _, key := range mz.g.keys() {
		r, err := o.resolve(key, set)
		if err != nil {
			return nil, err
		}
		if r.value != nil {
			out.values[key] = *r.value
		}
		open, err := o.openConflicts(key, set, r.value, r.conflict)
		if err != nil {
			return nil, err
		}
		out.conflicts = append(out.conflicts, open...)
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
		out.placement[key] = o.place(key, set, slugs)
	}
	for i := range out.conflicts {
		conflict := &out.conflicts[i]
		if err := conflict.seal(); err != nil {
			return nil, err
		}
		conflict.Path = o.place(conflict.Key, set, slugs)
		if conflict.Kind == EffectCatalog {
			conflict.Path = catalogRel
		}
	}
	sort.Slice(out.conflicts, func(i, j int) bool { return out.conflicts[i].ID < out.conflicts[j].ID })
	return out, nil
}

func (o *derivationOracle) place(key string, set bitset, slugs map[string]string) string {
	mz := o.mz
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
			if genesis, found := oracleGenesis(mz.g, key); found {
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
