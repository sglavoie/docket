package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// This file reconciles the synced workspace catalog and planning artifacts
// for sync: any number of concurrent versions of one file merged against the
// version they share. Every device must reach the same bytes from the same
// versions whatever order they arrived in, so each merge depends only on its
// inputs and always produces an artifact the live reader accepts, reporting
// what it could not merge beside the result instead of refusing.
//
// sides are in a deterministic order chosen by the caller; sides[0] is the
// primary version a conflict falls back to. Timestamps never pick a winner.

// SyncMergeConflict is one thing a reconciliation could not merge on its own.
// Subject names the project or planning entry, Field the part of it, and
// Detail what was kept.
type SyncMergeConflict struct {
	Subject string `json:"subject"`
	Field   string `json:"field"`
	Detail  string `json:"detail"`
}

func decodeSyncedArtifact(kind domainKind, data []byte, payload any) (string, error) {
	env, err := decodeEnvelope(data)
	if err != nil {
		return "", fmt.Errorf("decoding %s: %w", kind.name(), err)
	}
	if env.Kind != kind.name() {
		return "", fmt.Errorf("decoding %s: envelope kind %q", kind.name(), env.Kind)
	}
	if err := json.Unmarshal(env.Data, payload); err != nil {
		return "", fmt.Errorf("decoding %s payload: %w", kind.name(), err)
	}
	return env.WorkspaceID, nil
}

// ReconcileCatalogArtifacts merges concurrent workspace catalogs project by
// project. An entry one side changed takes that change; an entry several sides
// changed differently keeps the primary changed version whole; deleting a
// project never beats changing it. A merged slug two projects would share
// reverts to the common base, and two new projects sharing one slug keep it on
// the smaller id while the other takes a suffix of its own id.
func ReconcileCatalogArtifacts(base []byte, sides [][]byte) ([]byte, []SyncMergeConflict, error) {
	if len(sides) == 0 {
		return nil, nil, fmt.Errorf("reconcile requires at least one side")
	}
	decoded := make([]*WorkspaceCatalog, len(sides))
	workspaceID := ""
	for i, data := range sides {
		decoded[i] = &WorkspaceCatalog{}
		id, err := decodeSyncedArtifact(domainCatalog, data, decoded[i])
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			workspaceID = id
		}
	}
	baseCatalog := &WorkspaceCatalog{SchemaVersion: decoded[0].SchemaVersion, WorkspaceID: decoded[0].WorkspaceID}
	if base != nil {
		if _, err := decodeSyncedArtifact(domainCatalog, base, baseCatalog); err != nil {
			return nil, nil, err
		}
	}

	var conflicts []SyncMergeConflict
	baseByID := indexCatalogProjects(baseCatalog.Projects)
	sideByID := make([]map[string]CatalogProject, len(decoded))
	var sequences [][]string
	for i, catalog := range decoded {
		sideByID[i] = indexCatalogProjects(catalog.Projects)
		sequences = append(sequences, catalogProjectIDs(catalog.Projects))
	}
	merged := map[string]CatalogProject{}
	for _, id := range unionStrings(append([][]string{catalogProjectIDs(baseCatalog.Projects)}, sequences...)...) {
		b, inBase := baseByID[id]
		var changed []CatalogProject
		deleted := false
		for _, side := range sideByID {
			entry, ok := side[id]
			switch {
			case !ok:
				deleted = deleted || inBase
			case inBase && jsonEqual(entry, b):
			default:
				changed = appendDistinct(changed, entry)
			}
		}
		if len(changed) == 0 {
			if inBase && !deleted {
				merged[id] = b
			}
			continue
		}
		if deleted {
			conflicts = append(conflicts, SyncMergeConflict{Subject: id, Field: "project", Detail: "a project deleted concurrently was also changed; the project is kept"})
		}
		entry, fieldConflicts := mergeCatalogEntry(id, b, changed)
		conflicts = append(conflicts, fieldConflicts...)
		merged[id] = entry
	}

	order, orderConflict := mergeSequences(catalogProjectIDs(baseCatalog.Projects), sequences, merged)
	if orderConflict {
		conflicts = append(conflicts, SyncMergeConflict{Subject: "projects", Field: "order", Detail: "concurrent reorders are incompatible; the base order is kept"})
	}
	conflicts = append(conflicts, repairCatalogSlugs(order, merged, baseByID)...)

	result := &WorkspaceCatalog{
		SchemaVersion: decoded[0].SchemaVersion,
		WorkspaceID:   decoded[0].WorkspaceID,
		Projects:      make([]CatalogProject, 0, len(order)),
	}
	for _, id := range order {
		result.Projects = append(result.Projects, merged[id])
	}
	unfiledValues := make([]string, len(decoded))
	for i, catalog := range decoded {
		unfiledValues[i] = catalog.SystemUnfiledProjectID
	}
	unfiled, unfiledConflict := reconcileString(baseCatalog.SystemUnfiledProjectID, unfiledValues)
	result.SystemUnfiledProjectID = unfiled
	if unfiledConflict {
		conflicts = append(conflicts, SyncMergeConflict{Subject: "workspace", Field: "system_unfiled_project_id", Detail: "competing values; the base value is kept"})
	}

	encoded, err := encodeDomainEnvelope(domainCatalog, workspaceID, result)
	if err != nil {
		conflicts = append(conflicts, SyncMergeConflict{Subject: "workspace", Field: "catalog", Detail: fmt.Sprintf("the merged catalog is invalid (%v); the primary version is kept", err)})
		return sides[0], conflicts, nil
	}
	return encoded, conflicts, nil
}

// mergeCatalogEntry merges the distinct changed versions of one entry field by
// field; competing fields keep the first version whole.
func mergeCatalogEntry(id string, base CatalogProject, versions []CatalogProject) (CatalogProject, []SyncMergeConflict) {
	if len(versions) == 1 {
		return versions[0], nil
	}
	merged := base
	mv := reflect.ValueOf(&merged).Elem()
	bv := reflect.ValueOf(base)
	var competing []string
	for i := 0; i < bv.NumField(); i++ {
		var adopted []reflect.Value
		for _, version := range versions {
			field := reflect.ValueOf(version).Field(i)
			if jsonEqual(field.Interface(), bv.Field(i).Interface()) {
				continue
			}
			repeat := false
			for _, prior := range adopted {
				repeat = repeat || jsonEqual(prior.Interface(), field.Interface())
			}
			if !repeat {
				adopted = append(adopted, field)
			}
		}
		switch len(adopted) {
		case 0:
		case 1:
			mv.Field(i).Set(adopted[0])
		default:
			competing = append(competing, jsonFieldName(bv.Type().Field(i)))
		}
	}
	if len(competing) == 0 {
		return merged, nil
	}
	return versions[0], []SyncMergeConflict{{
		Subject: id, Field: strings.Join(competing, ","),
		Detail: fmt.Sprintf("%d competing versions; the primary version is kept", len(versions)),
	}}
}

// repairCatalogSlugs keeps merged slugs unique: a changed slug that collides
// reverts to its base slug, and a remaining collision keeps the slug on the
// smallest id and suffixes the others with their own id.
func repairCatalogSlugs(order []string, merged map[string]CatalogProject, base map[string]CatalogProject) []SyncMergeConflict {
	var conflicts []SyncMergeConflict
	holders := func() map[string][]string {
		out := map[string][]string{}
		for _, id := range order {
			out[merged[id].Slug] = append(out[merged[id].Slug], id)
		}
		return out
	}
	for slug, ids := range holders() {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			if b, ok := base[id]; ok && b.Slug != slug {
				entry := merged[id]
				entry.Slug = b.Slug
				merged[id] = entry
				conflicts = append(conflicts, SyncMergeConflict{Subject: id, Field: "slug", Detail: fmt.Sprintf("slug %q is taken concurrently; %q is kept", slug, b.Slug)})
			}
		}
	}
	for slug, ids := range holders() {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		for _, id := range ids[1:] {
			entry := merged[id]
			suffix := strings.ToLower(strings.NewReplacer("_", "-", ".", "-").Replace(id))
			if len(suffix) > 8 {
				suffix = suffix[:8]
			}
			entry.Slug = strings.Trim(slug+"-"+suffix, "-")
			merged[id] = entry
			conflicts = append(conflicts, SyncMergeConflict{Subject: id, Field: "slug", Detail: fmt.Sprintf("slug %q was created twice; this project is renamed to %q", slug, entry.Slug)})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Subject != conflicts[j].Subject {
			return conflicts[i].Subject < conflicts[j].Subject
		}
		return conflicts[i].Detail < conflicts[j].Detail
	})
	return conflicts
}

// RestoreCatalogProject returns catalog with projectID's entry restored from
// the catalog from, appended after the existing projects. It is how a project
// deleted on one device while another changed its files stays named: deletion
// never beats a change. restored is false when catalog already names the
// project or from does not.
func RestoreCatalogProject(catalog []byte, projectID string, from []byte) ([]byte, bool, error) {
	current := &WorkspaceCatalog{}
	workspaceID, err := decodeSyncedArtifact(domainCatalog, catalog, current)
	if err != nil {
		return nil, false, err
	}
	if _, ok := indexCatalogProjects(current.Projects)[projectID]; ok {
		return catalog, false, nil
	}
	source := &WorkspaceCatalog{}
	if _, sourceErr := decodeSyncedArtifact(domainCatalog, from, source); sourceErr != nil {
		return nil, false, sourceErr
	}
	entry, ok := indexCatalogProjects(source.Projects)[projectID]
	if !ok {
		return catalog, false, nil
	}
	current.Projects = append(current.Projects, entry)
	order := catalogProjectIDs(current.Projects)
	merged := indexCatalogProjects(current.Projects)
	repairCatalogSlugs(order, merged, map[string]CatalogProject{})
	for i, id := range order {
		current.Projects[i] = merged[id]
	}
	encoded, err := encodeDomainEnvelope(domainCatalog, workspaceID, current)
	if err != nil {
		return nil, false, err
	}
	return encoded, true, nil
}

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
