package synclog

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// materialize resolves every known key within set, repairs the workspace, and
// places each key.
func (mz *materializer) materialize(set bitset) (*materialized, error) {
	out := &materialized{values: map[string]string{}, placement: map[string]string{}}
	for _, key := range mz.g.keys() {
		r, err := mz.resolve(key, set)
		if err != nil {
			return nil, err
		}
		if r.value != nil {
			out.values[key] = *r.value
		}
		open, err := mz.openConflicts(key, set, r.value, r.conflict)
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
