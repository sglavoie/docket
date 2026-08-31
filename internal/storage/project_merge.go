package storage

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// This file implements the deterministic three-way merge of a single project's
// documents: a common base (the snapshot the caller loaded), the caller's local
// edits, and the current remote/on-disk state. It merges by stable task ID and
// by field, resolving independent edits automatically and surfacing everything
// else as structured, machine-readable conflicts. It NEVER writes: a conflicting
// merge returns a nil merged document so the caller aborts the whole ChangeSet
// (the optimistic-concurrency boundary in commit.go). It mirrors the workflow
// engine's typed-conflict convention (internal/workflow/engine_types.go) rather
// than emitting preformatted strings, keeping conflict computation separate from
// any human presentation.

// MergeConflictKind classifies a structured three-way merge conflict so callers
// can branch on the failure class without parsing message text.
type MergeConflictKind string

const (
	// MergeConflictFieldEdit is two divergent edits to the same task field.
	MergeConflictFieldEdit MergeConflictKind = "field_edit"
	// MergeConflictDeleteEdit is one side deleting a task the other side edited.
	MergeConflictDeleteEdit MergeConflictKind = "delete_edit"
	// MergeConflictHierarchy is an incompatible parent (hierarchy) or sibling
	// order edit that cannot be reconciled automatically.
	MergeConflictHierarchy MergeConflictKind = "hierarchy"
	// MergeConflictProjectMetadata is a divergent edit to canonical project-level
	// metadata (currently the project name).
	MergeConflictProjectMetadata MergeConflictKind = "project_metadata"
	// MergeConflictInvariant is a merged result that is individually
	// conflict-free but violates a project audit or state invariant.
	MergeConflictInvariant MergeConflictKind = "invariant"
)

// MergeConflict is a single structured conflict. It is deliberately data, not a
// formatted message: Kind drives programmatic handling, the identity fields
// locate the conflict, and Base/Local/Current carry the diverging values for a
// later human-facing renderer to format. TaskID/TaskTitle are empty for
// project-level and ordering conflicts.
type MergeConflict struct {
	Kind      MergeConflictKind
	TaskTitle string
	Field     string
	Base      string
	Local     string
	Current   string
	Detail    string
}

// ProjectMergeResult is the outcome of a three-way merge. When Conflicts is
// empty, Merged is the conflict-free merged document (revision recomputed) and
// the caller may commit it. When Conflicts is non-empty, Merged is nil: the
// merge resolves all-or-nothing, so no partial document is ever produced.
type ProjectMergeResult struct {
	Merged    *ProjectDocument
	Conflicts []MergeConflict
}

// HasConflicts reports whether the merge produced any structured conflict.
func (r *ProjectMergeResult) HasConflicts() bool {
	return len(r.Conflicts) > 0
}

// mergeableTaskFields lists the struct-field indexes of models.Task that carry
// user-authored persisted state and therefore participate in the field-level
// merge. Computed fields (Children, Parent, Project, Level — tagged json:"-")
// and derived persistence metadata (UpdatedAt) are excluded: the former are
// rebuilt from merged content, while the latter is reconciled by
// mergeTaskFields and the commit boundary without producing user conflicts.
// Computed once from the type so the field set can never drift from the struct
// definition.
var mergeableTaskFields = buildMergeableTaskFields()

func buildMergeableTaskFields() []taskFieldDescriptor {
	t := reflect.TypeOf(models.Task{})
	descriptors := make([]taskFieldDescriptor, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		// json:"-" marks the computed, non-persisted fields; skip them.
		if name == "-" {
			continue
		}
		// UpdatedAt is stamped by every business mutation, so concurrent edits to
		// different fields naturally carry different values. It is not
		// user-authored conflict material; mergeTaskFields reconciles it below.
		if field.Name == "UpdatedAt" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		descriptors = append(descriptors, taskFieldDescriptor{index: i, jsonName: name})
	}
	return descriptors
}

type taskFieldDescriptor struct {
	index    int
	jsonName string
}

// MergeProjectDocuments performs a field-level three-way merge of a single
// project's base, local, and current documents by stable task ID.
//
// Resolution rules (from the locked design in context.md):
//   - Independent task additions and edits to different fields merge automatically.
//   - Identical concurrent changes coalesce.
//   - Delete-versus-unchanged resolves as deletion.
//   - Delete-versus-edit, divergent edits to the same field, incompatible
//     hierarchy/order edits become conflicts.
//   - The merged result is re-audited (structure and state invariants);
//     any violation becomes a conflict.
//   - Any conflict aborts the whole merge: Merged is nil and nothing is written.
//
// It returns an error only for invalid input (a nil document); conflicts are
// returned as data, not errors.
func MergeProjectDocuments(base, local, current *ProjectDocument) (*ProjectMergeResult, error) {
	if base == nil || local == nil || current == nil {
		return nil, fmt.Errorf("three-way merge requires non-nil base, local, and current documents")
	}

	result := &ProjectMergeResult{}

	mergedTasks, taskConflicts := mergeTasks(base, local, current)
	result.Conflicts = append(result.Conflicts, taskConflicts...)

	orderedIDs, orderConflicts := mergeTaskOrder(base, local, current, mergedTasks)
	result.Conflicts = append(result.Conflicts, orderConflicts...)

	name, nameConflict := mergeScalarMetadata("name", base.ProjectName, local.ProjectName, current.ProjectName)
	if nameConflict != nil {
		result.Conflicts = append(result.Conflicts, *nameConflict)
	}

	// Any structural conflict aborts before we materialize a document: the merge
	// is all-or-nothing, so a partially resolved model must never escape.
	if result.HasConflicts() {
		return result, nil
	}

	merged := &ProjectDocument{
		ProjectName:   name,
		Tasks:         orderTasks(mergedTasks, orderedIDs),
		SchemaVersion: SchemaVersionV15,
	}

	// Re-run audits and state invariants on the merged model. A merge that is
	// field-wise clean can still compose an invalid whole such as a terminal state
	// without its completion timestamp.
	if conflicts := auditMergedDocument(merged); len(conflicts) > 0 {
		result.Conflicts = append(result.Conflicts, conflicts...)
		return result, nil
	}

	merged.Revision = merged.ComputeRevision()
	result.Merged = merged
	return result, nil
}

// mergeTasks resolves the presence and field state of every task ID across the
// three documents, returning the surviving merged tasks keyed by ID plus any
// per-task conflicts. Ordering is decided separately by mergeTaskOrder.
func mergeTasks(base, local, current *ProjectDocument) (map[string]*models.Task, []MergeConflict) {
	baseByID := indexTasks(base.Tasks)
	localByID := indexTasks(local.Tasks)
	currentByID := indexTasks(current.Tasks)

	merged := make(map[string]*models.Task)
	var conflicts []MergeConflict

	for _, id := range unionTaskIDs(base.Tasks, local.Tasks, current.Tasks) {
		b, inB := baseByID[id]
		l, inL := localByID[id]
		c, inC := currentByID[id]

		switch {
		case inB && inL && inC:
			task, fieldConflicts := mergeTaskFields(b, l, c)
			if len(fieldConflicts) > 0 {
				conflicts = append(conflicts, fieldConflicts...)
				continue
			}
			merged[id] = task
		case inB && inL && !inC:
			// current deleted the task; keep only if local left it untouched.
			if tasksEqual(b, l) {
				continue
			}
			conflicts = append(conflicts, deleteEditConflict(l, "local edited a task deleted on current"))
		case inB && !inL && inC:
			if tasksEqual(b, c) {
				continue
			}
			conflicts = append(conflicts, deleteEditConflict(c, "current edited a task deleted on local"))
		case inB && !inL && !inC:
			// Deleted on both sides: drop it.
			continue
		case !inB && inL && inC:
			// Added independently on both sides under the same ID: merge field by
			// field against a zero base so identical additions coalesce and
			// divergent ones surface as field conflicts.
			task, fieldConflicts := mergeTaskFields(&models.Task{ID: id}, l, c)
			if len(fieldConflicts) > 0 {
				conflicts = append(conflicts, fieldConflicts...)
				continue
			}
			merged[id] = task
		case !inB && inL && !inC:
			merged[id] = cloneTask(l)
		case !inB && !inL && inC:
			merged[id] = cloneTask(c)
		}
	}

	return merged, conflicts
}

// mergeTaskFields three-way merges the persisted fields of one task. It starts
// from a copy of base and, for each field, adopts the side that changed it;
// fields both sides changed identically coalesce, and fields both sides changed
// differently become conflicts. A change to parent_id is classified as a
// hierarchy conflict rather than a plain field conflict.
func mergeTaskFields(base, local, current *models.Task) (*models.Task, []MergeConflict) {
	merged := cloneTask(base)
	mv := reflect.ValueOf(merged).Elem()
	bv := reflect.ValueOf(base).Elem()
	lv := reflect.ValueOf(local).Elem()
	cv := reflect.ValueOf(current).Elem()

	var conflicts []MergeConflict
	for _, f := range mergeableTaskFields {
		bField := bv.Field(f.index).Interface()
		lField := lv.Field(f.index).Interface()
		cField := cv.Field(f.index).Interface()

		localChanged := !reflect.DeepEqual(lField, bField)
		currentChanged := !reflect.DeepEqual(cField, bField)

		switch {
		case localChanged && currentChanged:
			if reflect.DeepEqual(lField, cField) {
				mv.Field(f.index).Set(lv.Field(f.index)) // coalesce identical concurrent edits
				continue
			}
			conflicts = append(conflicts, fieldEditConflict(mergedTitle(local, current), f.jsonName, bField, lField, cField))
		case localChanged:
			mv.Field(f.index).Set(lv.Field(f.index))
		case currentChanged:
			mv.Field(f.index).Set(cv.Field(f.index))
		}
	}

	// Adopting a field copies its slice header, so the dependency list the merge
	// just took from one side still points at that side's backing array. Detach
	// it for the same reason cloneTask does: the merged document becomes live
	// state, and a later dependency edit must not reach back into an input.
	merged.Dependencies = models.CloneTaskReferences(merged.Dependencies)

	// UpdatedAt describes the newest accepted mutation rather than an
	// independently editable value. Taking the latest value is deterministic,
	// commutative, and prevents two independent business edits from conflicting
	// merely because their writers stamped different times.
	merged.UpdatedAt = base.UpdatedAt
	if local.UpdatedAt.After(merged.UpdatedAt) {
		merged.UpdatedAt = local.UpdatedAt
	}
	if current.UpdatedAt.After(merged.UpdatedAt) {
		merged.UpdatedAt = current.UpdatedAt
	}

	return merged, conflicts
}

// mergeTaskOrder three-way merges each sibling group independently. The flat
// document slice is only a carrier for manual order: tasks with different
// parents do not participate in the same ordering decision. Independent
// reorders under different parents therefore coalesce, while incompatible
// reorders within one parent group remain hierarchy conflicts.
func mergeTaskOrder(base, local, current *ProjectDocument, survivors map[string]*models.Task) ([]string, []MergeConflict) {
	backbone := survivingTaskOrder(survivors, base.Tasks, local.Tasks, current.Tasks)
	groupMembers := make(map[string]map[string]bool)
	groupOrder := make([]string, 0)
	for _, id := range backbone {
		group := taskSiblingGroup(survivors[id])
		if groupMembers[group] == nil {
			groupMembers[group] = make(map[string]bool)
			groupOrder = append(groupOrder, group)
		}
		groupMembers[group][id] = true
	}

	mergedGroups := make(map[string][]string, len(groupMembers))
	var conflicts []MergeConflict
	for _, group := range groupOrder {
		members := groupMembers[group]
		baseIDs := documentSiblingOrder(base.Tasks, group, members)
		localIDs := documentSiblingOrder(local.Tasks, group, members)
		currentIDs := documentSiblingOrder(current.Tasks, group, members)

		authoritative, conflict := mergeSiblingOrder(group, baseIDs, localIDs, currentIDs)
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
			continue
		}
		mergedGroups[group] = completeSiblingOrder(authoritative, localIDs, currentIDs, baseIDs, filterOrder(backbone, members))
	}
	if len(conflicts) > 0 {
		return nil, conflicts
	}

	// Keep the cross-group slot pattern from the deterministic backbone and fill
	// each group's slots from its independently merged order. This changes only
	// sibling order; unrelated groups retain their own relative order.
	next := make(map[string]int, len(mergedGroups))
	ordered := make([]string, 0, len(backbone))
	for _, id := range backbone {
		group := taskSiblingGroup(survivors[id])
		index := next[group]
		ordered = append(ordered, mergedGroups[group][index])
		next[group] = index + 1
	}
	return ordered, nil
}

func mergeSiblingOrder(group string, baseIDs, localIDs, currentIDs []string) ([]string, *MergeConflict) {
	localReordered := relativeOrderChanged(baseIDs, localIDs)
	currentReordered := relativeOrderChanged(baseIDs, currentIDs)

	switch {
	case !localReordered && !currentReordered:
		return baseIDs, nil
	case localReordered && !currentReordered:
		return localIDs, nil
	case !localReordered && currentReordered:
		return currentIDs, nil
	default:
		shared := intersectionSet(localIDs, currentIDs)
		if stringSlicesEqual(filterOrder(localIDs, shared), filterOrder(currentIDs, shared)) {
			return localIDs, nil
		}
		label := "root tasks"
		if group != "" {
			label = fmt.Sprintf("children of task %q", group)
		}
		return nil, &MergeConflict{
			Kind:   MergeConflictHierarchy,
			Field:  "order",
			Detail: fmt.Sprintf("local and current reordered %s incompatibly", label),
		}
	}
}

func completeSiblingOrder(sequences ...[]string) []string {
	var ordered []string
	placed := make(map[string]bool)
	for _, sequence := range sequences {
		for _, id := range sequence {
			if placed[id] {
				continue
			}
			ordered = append(ordered, id)
			placed[id] = true
		}
	}
	return ordered
}

func survivingTaskOrder(survivors map[string]*models.Task, taskSets ...[]*models.Task) []string {
	ordered := make([]string, 0, len(survivors))
	placed := make(map[string]bool, len(survivors))
	for _, tasks := range taskSets {
		for _, task := range tasks {
			if task == nil || placed[task.ID] {
				continue
			}
			if _, ok := survivors[task.ID]; !ok {
				continue
			}
			ordered = append(ordered, task.ID)
			placed[task.ID] = true
		}
	}
	return ordered
}

func documentSiblingOrder(tasks []*models.Task, group string, members map[string]bool) []string {
	ordered := make([]string, 0, len(members))
	for _, task := range tasks {
		if task == nil || !members[task.ID] || taskSiblingGroup(task) != group {
			continue
		}
		ordered = append(ordered, task.ID)
	}
	return ordered
}

func taskSiblingGroup(task *models.Task) string {
	if task == nil || task.ParentID == nil {
		return ""
	}
	return *task.ParentID
}

// auditMergedDocument re-runs the structural audit and state
// invariants on a merged document, returning conflicts for any violation. It
// never mutates the merged tasks (unlike BuildTaskTree, which silently repairs),
// so a genuine invariant break surfaces instead of being papered over.
func auditMergedDocument(doc *ProjectDocument) []MergeConflict {
	project := &models.Project{
		Name:  doc.ProjectName,
		Tasks: doc.Tasks,
	}

	var conflicts []MergeConflict
	if err := models.AuditProject(project); err != nil {
		conflicts = append(conflicts, MergeConflict{
			Kind:   MergeConflictInvariant,
			Field:  "audit",
			Detail: err.Error(),
		})
	}

	for _, task := range doc.Tasks {
		// State invariant: a completion timestamp exists exactly for terminal
		// states. A field-wise clean merge can still combine a terminal state from
		// one side with a nil (or lingering) timestamp from the other.
		terminal := task.IsTerminalState()
		if terminal && task.CompletedAt == nil {
			conflicts = append(conflicts, MergeConflict{
				Kind:      MergeConflictInvariant,
				TaskTitle: task.Title,
				Field:     "completed_at",
				Detail:    fmt.Sprintf("terminal state %q has no completion timestamp", task.State()),
			})
		}
		if !terminal && task.CompletedAt != nil {
			conflicts = append(conflicts, MergeConflict{
				Kind:      MergeConflictInvariant,
				TaskTitle: task.Title,
				Field:     "completed_at",
				Detail:    fmt.Sprintf("non-terminal state %q carries a completion timestamp", task.State()),
			})
		}
	}

	return conflicts
}

// mergeScalarMetadata three-way merges a single string project-metadata field,
// returning the merged value and a conflict when both sides diverged.
func mergeScalarMetadata(field, base, local, current string) (string, *MergeConflict) {
	value, conflicted := scalarThreeWay(base, local, current)
	if !conflicted {
		return value, nil
	}
	return "", &MergeConflict{
		Kind:    MergeConflictProjectMetadata,
		Field:   field,
		Base:    base,
		Local:   local,
		Current: current,
	}
}

// --- small helpers -------------------------------------------------------

func indexTasks(tasks []*models.Task) map[string]*models.Task {
	m := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		m[task.ID] = task
	}
	return m
}

// unionTaskIDs returns every distinct task ID across the three documents in a
// deterministic order: base order first, then local-only additions, then
// current-only additions, so conflict reporting is stable across runs.
func unionTaskIDs(base, local, current []*models.Task) []string {
	var order []string
	seen := make(map[string]bool)
	appendIDs := func(tasks []*models.Task) {
		for _, task := range tasks {
			if task == nil || seen[task.ID] {
				continue
			}
			seen[task.ID] = true
			order = append(order, task.ID)
		}
	}
	appendIDs(base)
	appendIDs(local)
	appendIDs(current)
	return order
}

func orderTasks(byID map[string]*models.Task, order []string) []*models.Task {
	tasks := make([]*models.Task, 0, len(order))
	for _, id := range order {
		if task, ok := byID[id]; ok {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// tasksEqual reports whether two tasks are equal across all persisted fields,
// ignoring the computed (json:"-") fields.
func tasksEqual(a, b *models.Task) bool {
	av := reflect.ValueOf(a).Elem()
	bv := reflect.ValueOf(b).Elem()
	for _, f := range mergeableTaskFields {
		if !reflect.DeepEqual(av.Field(f.index).Interface(), bv.Field(f.index).Interface()) {
			return false
		}
	}
	return true
}

// cloneTask returns a shallow copy of a task's persisted fields. Slice and map
// fields share backing storage with the source; the merged document is a
// read-only result (session 20 does not write), so this aliasing is safe and
// avoids an expensive deep copy of every task.
//
// Dependencies is the one exception. The merge adopts a whole field from one
// side and the resolved document later flows into a live project, so an aliased
// locator slice would let a subsequent dependency edit reach back into the base
// or current document the merge read from.
func cloneTask(src *models.Task) *models.Task {
	copied := *src
	copied.Dependencies = models.CloneTaskReferences(src.Dependencies)
	return &copied
}

func mergedTitle(local, current *models.Task) string {
	if local != nil && local.Title != "" {
		return local.Title
	}
	if current != nil {
		return current.Title
	}
	return ""
}

func deleteEditConflict(edited *models.Task, detail string) MergeConflict {
	title := ""
	if edited != nil {
		title = edited.Title
	}
	return MergeConflict{
		Kind:      MergeConflictDeleteEdit,
		TaskTitle: title,
		Field:     "task",
		Detail:    detail,
	}
}

func fieldEditConflict(title, field string, base, local, current any) MergeConflict {
	kind := MergeConflictFieldEdit
	if field == "parent_id" {
		kind = MergeConflictHierarchy
	}
	return MergeConflict{
		Kind:      kind,
		TaskTitle: title,
		Field:     field,
		Base:      fmt.Sprintf("%v", base),
		Local:     fmt.Sprintf("%v", local),
		Current:   fmt.Sprintf("%v", current),
	}
}

func scalarThreeWay(base, local, current string) (string, bool) {
	localChanged := local != base
	currentChanged := current != base
	switch {
	case localChanged && currentChanged:
		if local == current {
			return local, false
		}
		return "", true
	case localChanged:
		return local, false
	case currentChanged:
		return current, false
	default:
		return base, false
	}
}

// relativeOrderChanged reports whether side reordered the tasks it shares with
// base relative to base's order. Additions and deletions alone do not count as a
// reorder; only a changed relative sequence of the shared IDs does.
func relativeOrderChanged(base, side []string) bool {
	shared := intersectionSet(base, side)
	return !stringSlicesEqual(filterOrder(base, shared), filterOrder(side, shared))
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func intersectionSet(a, b []string) map[string]bool {
	aSet := toSet(a)
	shared := make(map[string]bool)
	for _, v := range b {
		if aSet[v] {
			shared[v] = true
		}
	}
	return shared
}

func filterOrder(ids []string, keep map[string]bool) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if keep[id] {
			out = append(out, id)
		}
	}
	return out
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}
