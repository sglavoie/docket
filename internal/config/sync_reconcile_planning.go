package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ReconcilePlanningArtifacts merges concurrent Today and Next collections.
// Membership merges as set changes against the base: an addition anywhere is
// kept and a removal anywhere is kept. One reorder, or several compatible ones,
// is adopted; incompatible reorders keep the base order. A task added to Today
// on one device and to Next on another stays in Today, since the two are
// mutually exclusive.
func ReconcilePlanningArtifacts(base []byte, sides [][]byte) ([]byte, []SyncMergeConflict, error) {
	if len(sides) == 0 {
		return nil, nil, fmt.Errorf("reconcile requires at least one side")
	}
	decoded := make([]*PlanningState, len(sides))
	workspaceID := ""
	for i, data := range sides {
		decoded[i] = &PlanningState{}
		id, err := decodeSyncedArtifact(domainPlanning, data, decoded[i])
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			workspaceID = id
		}
	}
	basePlanning := &PlanningState{}
	if base != nil {
		if _, err := decodeSyncedArtifact(domainPlanning, base, basePlanning); err != nil {
			return nil, nil, err
		}
	}

	var conflicts []SyncMergeConflict
	collection := func(name string, baseItems []ProjectTaskLocator, pick func(*PlanningState) []ProjectTaskLocator) []ProjectTaskLocator {
		baseKeys := locatorKeys(baseItems)
		var sequences [][]string
		for _, side := range decoded {
			sequences = append(sequences, locatorKeys(pick(side)))
		}
		members := mergeMembership[struct{}](baseKeys, sequences)
		order, conflicted := mergeSequences(baseKeys, sequences, members)
		if conflicted {
			conflicts = append(conflicts, SyncMergeConflict{Subject: name, Field: "order", Detail: "concurrent reorders are incompatible; the base order is kept"})
		}
		out := make([]ProjectTaskLocator, 0, len(order))
		for _, key := range order {
			out = append(out, locatorFromKey(key))
		}
		return out
	}
	result := &PlanningState{
		Today: collection("today", basePlanning.Today, func(p *PlanningState) []ProjectTaskLocator { return p.Today }),
		Next:  collection("next", basePlanning.Next, func(p *PlanningState) []ProjectTaskLocator { return p.Next }),
	}
	today := map[string]bool{}
	for _, item := range result.Today {
		today[locatorKey(item)] = true
	}
	next := result.Next[:0]
	for _, item := range result.Next {
		if today[locatorKey(item)] {
			conflicts = append(conflicts, SyncMergeConflict{Subject: locatorKey(item), Field: "collection", Detail: "planned for Today and Next concurrently; Today is kept"})
			continue
		}
		next = append(next, item)
	}
	result.Next = next
	if len(result.Today) == 0 {
		result.Today = nil
	}
	if len(result.Next) == 0 {
		result.Next = nil
	}

	encoded, err := encodeDomainEnvelope(domainPlanning, workspaceID, result)
	if err != nil {
		conflicts = append(conflicts, SyncMergeConflict{Subject: "planning", Field: "planning", Detail: fmt.Sprintf("the merged planning is invalid (%v); the primary version is kept", err)})
		return sides[0], conflicts, nil
	}
	return encoded, conflicts, nil
}

// mergeMembership is the set merge of keyed sequences against base: kept when
// in base and no side removed it, or when some side added it.
func mergeMembership[V any](base []string, sides [][]string) map[string]V {
	inBase := toStringSet(base)
	out := map[string]V{}
	for _, key := range base {
		removed := false
		for _, side := range sides {
			removed = removed || !toStringSet(side)[key]
		}
		if !removed {
			var zero V
			out[key] = zero
		}
	}
	for _, side := range sides {
		for _, key := range side {
			if !inBase[key] {
				var zero V
				out[key] = zero
			}
		}
	}
	return out
}

// mergeSequences orders the members of a merged keyed sequence. A side that
// changed the relative order of what it shares with base is a reorder; one
// reorder, or several that agree on what they share, is adopted, while
// incompatible reorders keep the base order and report a conflict. Members a
// side added follow in side order.
func mergeSequences[V any](base []string, sides [][]string, members map[string]V) ([]string, bool) {
	var reordered [][]string
	for _, side := range sides {
		if !sameStrings(filterStrings(base, toStringSet(side)), filterStrings(side, toStringSet(base))) {
			reordered = append(reordered, side)
		}
	}
	authoritative := base
	conflicted := false
	if len(reordered) > 0 {
		authoritative = reordered[0]
		for _, other := range reordered[1:] {
			shared := toStringSet(filterStrings(authoritative, toStringSet(other)))
			if !sameStrings(filterStrings(authoritative, shared), filterStrings(other, shared)) {
				authoritative = base
				conflicted = true
				break
			}
		}
	}
	var out []string
	placed := map[string]bool{}
	for _, sequence := range append(append([][]string{authoritative}, sides...), base) {
		for _, key := range sequence {
			if _, ok := members[key]; ok && !placed[key] {
				placed[key] = true
				out = append(out, key)
			}
		}
	}
	return out, conflicted
}

func reconcileString(base string, values []string) (string, bool) {
	var changed []string
	for _, value := range values {
		if value != base && !toStringSet(changed)[value] {
			changed = append(changed, value)
		}
	}
	switch len(changed) {
	case 0:
		return base, false
	case 1:
		return changed[0], false
	}
	return base, true
}

func indexCatalogProjects(projects []CatalogProject) map[string]CatalogProject {
	out := make(map[string]CatalogProject, len(projects))
	for _, p := range projects {
		out[p.ID] = p
	}
	return out
}

func catalogProjectIDs(projects []CatalogProject) []string {
	out := make([]string, len(projects))
	for i, p := range projects {
		out[i] = p.ID
	}
	return out
}

func appendDistinct(entries []CatalogProject, entry CatalogProject) []CatalogProject {
	for _, existing := range entries {
		if jsonEqual(existing, entry) {
			return entries
		}
	}
	return append(entries, entry)
}

// jsonEqual compares two values by their persisted encoding, which is what a
// device wrote. Decoded instants carry per-parse zones that reflect.DeepEqual
// would call different.
func jsonEqual(a, b any) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}

func jsonFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("json"), ",")[0]
	if name == "" {
		return field.Name
	}
	return name
}

func locatorKey(item ProjectTaskLocator) string { return item.ProjectID + "/" + item.TaskID }

func locatorFromKey(key string) ProjectTaskLocator {
	project, task, _ := strings.Cut(key, "/")
	return ProjectTaskLocator{ProjectID: project, TaskID: task}
}

func locatorKeys(items []ProjectTaskLocator) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = locatorKey(item)
	}
	return out
}

func unionStrings(sequences ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, sequence := range sequences {
		for _, value := range sequence {
			if !seen[value] {
				seen[value] = true
				out = append(out, value)
			}
		}
	}
	return out
}

func toStringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

func filterStrings(values []string, keep map[string]bool) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if keep[v] {
			out = append(out, v)
		}
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
