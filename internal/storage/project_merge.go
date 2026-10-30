package storage

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/sglavoie/docket/internal/models"
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
// later human-facing renderer to format. TaskTitle is empty for project-level
// and ordering conflicts.
//
// CommentID locates a project-comment conflict by the identity the collection
// merges on, and is empty for every other conflict. A project comment is not a
// task, so it is named in a field of its own rather than borrowed into
// TaskTitle: a bundle that spelled a comment id where a reader expects a task
// title would send the operator looking for a task that does not exist.
type MergeConflict struct {
	Kind      MergeConflictKind
	TaskTitle string
	CommentID string
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
		descriptors = append(descriptors, taskFieldDescriptor{
			index: i, jsonName: name, checkInUnit: checkInUnitFields[name],
		})
	}
	return descriptors
}

// checkInUnitFields names the three persisted fields that answer one question
// together: when this task is next meant to be looked at, how far acknowledging
// moves that date, and when it was last answered. They stay in
// mergeableTaskFields so tasksEqual and every other reader still sees a complete
// field set; mergeTaskFields is the one place that skips them, because merging
// them independently is what would compose a hybrid schedule.
//
// The names are the JSON tags rather than the Go field names, since the
// descriptor's jsonName is what a conflict is reported under and a mismatch here
// would silently fall back to the per-field path.
var checkInUnitFields = map[string]bool{
	"next_check_in":        true,
	"check_in_repeat_days": true,
	"last_reviewed_at":     true,
}

type taskFieldDescriptor struct {
	index    int
	jsonName string
	// checkInUnit marks a field mergeTaskFields hands to mergeCheckInUnit rather
	// than merging on its own.
	checkInUnit bool
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

	comments, commentConflicts := mergeProjectComments(base.Comments, local.Comments, current.Comments)
	result.Conflicts = append(result.Conflicts, commentConflicts...)

	// Any structural conflict aborts before we materialize a document: the merge
	// is all-or-nothing, so a partially resolved model must never escape.
	if result.HasConflicts() {
		return result, nil
	}

	merged := &ProjectDocument{
		ProjectName:   name,
		Tasks:         orderTasks(mergedTasks, orderedIDs),
		Comments:      comments,
		SchemaVersion: CurrentTaskSchemaVersion,
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
	copyOfBase := *base
	merged := &copyOfBase
	mv := reflect.ValueOf(merged).Elem()
	bv := reflect.ValueOf(base).Elem()
	lv := reflect.ValueOf(local).Elem()
	cv := reflect.ValueOf(current).Elem()

	var conflicts []MergeConflict
	for _, f := range mergeableTaskFields {
		if f.checkInUnit {
			continue
		}
		bField := bv.Field(f.index).Interface()
		lField := lv.Field(f.index).Interface()
		cField := cv.Field(f.index).Interface()

		localChanged := !f.equal(lField, bField)
		currentChanged := !f.equal(cField, bField)

		switch {
		case localChanged && currentChanged:
			if f.equal(lField, cField) {
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

	if unitConflict := mergeCheckInUnit(merged, base, local, current); unitConflict != nil {
		conflicts = append(conflicts, *unitConflict)
	}

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

	// Reflection adopts pointers and slice headers. Detach after every field has
	// been selected so the result shares no mutable task data with any input.
	return cloneTask(merged), conflicts
}
