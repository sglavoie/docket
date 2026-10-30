package storage

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// This file is the sync reconciliation of one project document: any number of
// concurrent versions measured against the version they all descend from. It
// extends MergeProjectDocuments, which is two-sided and all-or-nothing because
// its caller can abort and retry. Sync has no one to retry: every device must
// land on the same valid document from the same versions, whatever order they
// arrived in, so this merge always produces a document and reports what it
// could not merge as conflicts beside it.
//
// The rules are conservative and depend only on the inputs, never on which
// device runs them or on any timestamp:
//   - Independent changes merge field by field, as in MergeProjectDocuments.
//   - A task whose versions conflict is kept whole as the primary side wrote
//     it: the first side, in the caller's deterministic order, that changed it.
//     The competing versions stay recoverable from the sides themselves.
//   - Deleting a task never beats changing it: the changed task survives.
//   - A deleted parent survives while anything beneath it does, and a merged
//     hierarchy that would cycle or nest too deep reverts its reparenting.
//   - Two tasks sharing one short id keep their UUIDs; the one that held the
//     number before keeps it and the other is renumbered past every number used.
//   - Incompatible reorders keep the common base order.

// ReconcileProjectDocuments merges sides, the concurrent versions of one
// project document, against base, the version they share. base may be nil when
// the document did not exist before the sides created it. sides must be in a
// deterministic order that does not depend on delivery; sides[0] is the
// primary version conflicts fall back to. It returns a valid document and the
// conflicts it kept the primary version for. It errors only for no sides.
func ReconcileProjectDocuments(base *ProjectDocument, sides []*ProjectDocument) (*ProjectDocument, []MergeConflict, error) {
	if len(sides) == 0 {
		return nil, nil, fmt.Errorf("reconcile requires at least one side")
	}
	for i, side := range sides {
		if side == nil {
			return nil, nil, fmt.Errorf("reconcile side %d is nil", i)
		}
	}
	if len(sides) == 1 {
		return sides[0].clone(), nil, nil
	}
	if base == nil {
		base = &ProjectDocument{SchemaVersion: CurrentTaskSchemaVersion}
	}

	r := &projectReconciler{base: base, sides: sides, baseByID: indexTasks(base.Tasks)}
	for _, side := range sides {
		r.sideByID = append(r.sideByID, indexTasks(side.Tasks))
	}
	r.mergeTasks()
	r.repairHierarchy()
	r.renumberShortIDs()
	order := r.mergeOrder()

	name, nameConflict := reconcileScalar("name", base.ProjectName, sideNames(sides))
	if nameConflict != nil {
		r.conflicts = append(r.conflicts, *nameConflict)
	}
	comments, commentConflicts := reconcileProjectComments(base.Comments, sides)
	r.conflicts = append(r.conflicts, commentConflicts...)

	merged := &ProjectDocument{
		ProjectName:   name,
		Tasks:         orderTasks(r.merged, order),
		Comments:      comments,
		SchemaVersion: CurrentTaskSchemaVersion,
	}
	if audit := auditMergedDocument(merged); len(audit) > 0 {
		// Every rule above keeps the document valid; this is the backstop for a
		// rule that missed a case. The primary side is a document some device
		// wrote, so it is valid by construction.
		return sides[0].clone(), append(r.conflicts, audit...), nil
	}
	if failure := renderFailure(merged); failure != "" {
		return sides[0].clone(), append(r.conflicts, MergeConflict{Kind: MergeConflictInvariant, Field: "render", Detail: failure}), nil
	}
	merged.Revision = merged.ComputeRevision()
	return merged, r.conflicts, nil
}

// renderFailure is why doc does not render, or empty when it does.
func renderFailure(doc *ProjectDocument) string {
	if _, err := doc.WriteCurrent(); err != nil {
		return err.Error()
	}
	return ""
}

type projectReconciler struct {
	base      *ProjectDocument
	sides     []*ProjectDocument
	baseByID  map[string]*models.Task
	sideByID  []map[string]*models.Task
	merged    map[string]*models.Task
	conflicts []MergeConflict
}

// mergeTasks decides the presence and fields of every task.
func (r *projectReconciler) mergeTasks() {
	r.merged = map[string]*models.Task{}
	for _, id := range r.unionIDs() {
		b, inBase := r.baseByID[id]
		var present []*models.Task
		deleted := false
		for _, side := range r.sideByID {
			if task, ok := side[id]; ok {
				present = append(present, task)
			} else if inBase {
				deleted = true
			}
		}
		if !inBase {
			r.merged[id] = r.mergeVersions(&models.Task{ID: id}, present)
			continue
		}
		var changed []*models.Task
		for _, task := range present {
			if !tasksEqual(b, task) {
				changed = append(changed, task)
			}
		}
		switch {
		case len(changed) == 0 && deleted:
			continue
		case len(changed) == 0:
			r.merged[id] = cloneTask(b)
		default:
			if deleted {
				r.conflicts = append(r.conflicts, deleteEditConflict(changed[0], "a task deleted concurrently was also changed; the change is kept"))
			}
			r.merged[id] = r.mergeVersions(b, changed)
		}
		// Reordering changes UpdatedAt without changing any mergeable task field.
		// Keep those touches even when their side was excluded from changed.
		task := r.merged[id]
		task.UpdatedAt = b.UpdatedAt
		if newest := latestUpdate(present); newest.After(task.UpdatedAt) {
			task.UpdatedAt = newest
		}
	}
}

func (r *projectReconciler) unionIDs() []string {
	seen := map[string]bool{}
	var ids []string
	for _, tasks := range append([][]*models.Task{r.base.Tasks}, sideTasks(r.sides)...) {
		for _, task := range tasks {
			if task != nil && !seen[task.ID] {
				seen[task.ID] = true
				ids = append(ids, task.ID)
			}
		}
	}
	return ids
}

// mergeVersions merges the distinct versions of one task against base. A
// field two versions changed differently, or a field-wise merge that composes
// an invalid task, keeps the first version whole.
func (r *projectReconciler) mergeVersions(base *models.Task, versions []*models.Task) *models.Task {
	distinct := make([]*models.Task, 0, len(versions))
	for _, version := range versions {
		duplicate := false
		for _, kept := range distinct {
			if tasksEqual(kept, version) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			distinct = append(distinct, version)
		}
	}
	if len(distinct) == 1 {
		task := cloneTask(distinct[0])
		task.UpdatedAt = latestUpdate(versions)
		return task
	}

	copyOfBase := *base
	merged := &copyOfBase
	mv := reflect.ValueOf(merged).Elem()
	bv := reflect.ValueOf(base).Elem()
	var conflicts []MergeConflict
	for _, f := range mergeableTaskFields {
		if f.checkInUnit {
			continue
		}
		baseField := bv.Field(f.index).Interface()
		var adopted []reflect.Value
		for _, version := range distinct {
			field := reflect.ValueOf(version).Elem().Field(f.index)
			if f.equal(field.Interface(), baseField) {
				continue
			}
			repeat := false
			for _, prior := range adopted {
				if f.equal(prior.Interface(), field.Interface()) {
					repeat = true
					break
				}
			}
			if !repeat {
				adopted = append(adopted, field)
			}
		}
		switch len(adopted) {
		case 0:
		case 1:
			mv.Field(f.index).Set(adopted[0])
		default:
			conflicts = append(conflicts, versionConflict(distinct, f.jsonName, baseField, adopted))
		}
	}
	baseUnit := checkInUnitOf(base)
	var units []taskCheckInUnit
	for _, version := range distinct {
		unit := checkInUnitOf(version)
		if unit.equals(baseUnit) {
			continue
		}
		repeat := false
		for _, prior := range units {
			if prior.equals(unit) {
				repeat = true
				break
			}
		}
		if !repeat {
			units = append(units, unit)
		}
	}
	adopt := baseUnit
	switch len(units) {
	case 0:
	case 1:
		adopt = units[0]
	default:
		values := make([]reflect.Value, len(units))
		for i, unit := range units {
			values[i] = reflect.ValueOf(unit.String())
		}
		conflicts = append(conflicts, versionConflict(distinct, "check_in", baseUnit.String(), values))
	}
	merged.NextCheckIn = models.CloneInstant(adopt.next)
	merged.CheckInRepeatDays = adopt.repeatDays
	merged.LastReviewedAt = models.CloneInstant(adopt.lastReviewedAt)
	merged.UpdatedAt = latestUpdate(append([]*models.Task{base}, versions...))

	if len(conflicts) == 0 {
		probe := &ProjectDocument{Tasks: []*models.Task{merged}}
		for _, c := range auditMergedDocument(probe) {
			if c.Field != "audit" {
				conflicts = append(conflicts, c)
			}
		}
	}
	if len(conflicts) > 0 {
		r.conflicts = append(r.conflicts, conflicts...)
		task := cloneTask(distinct[0])
		task.UpdatedAt = latestUpdate(versions)
		return task
	}
	return cloneTask(merged)
}

// versionConflict reports competing versions of one field. Values are
// rendered through their pointees, so the record reads the same on every
// device rather than naming one process's addresses.
func versionConflict(versions []*models.Task, field string, base any, values []reflect.Value) MergeConflict {
	conflict := fieldEditConflict(versions[0].Title, field, nil, nil, nil)
	conflict.Base = taskFieldConflictValue(field, base)
	conflict.Local = taskFieldConflictValue(field, values[0].Interface())
	conflict.Current = taskFieldConflictValue(field, values[1].Interface())
	if len(values) > 2 {
		rest := make([]string, 0, len(values)-2)
		for _, value := range values[2:] {
			rest = append(rest, taskFieldConflictValue(field, value.Interface()))
		}
		conflict.Detail = fmt.Sprintf("further versions: %v", rest)
	}
	return conflict
}

// latestUpdate is the newest touch time among versions. UpdatedAt is not
// user-authored, so the merge takes the latest as MergeProjectDocuments does.
func latestUpdate(versions []*models.Task) (latest time.Time) {
	for _, version := range versions {
		if version.UpdatedAt.After(latest) {
			latest = version.UpdatedAt
		}
	}
	return latest
}

func reconcileScalar(field, base string, values []string) (string, *MergeConflict) {
	var changed []string
	for _, value := range values {
		if value != base && !containsString(changed, value) {
			changed = append(changed, value)
		}
	}
	switch len(changed) {
	case 0:
		return base, nil
	case 1:
		return changed[0], nil
	}
	keep := base
	if keep == "" {
		keep = values[0]
	}
	return keep, &MergeConflict{
		Kind: MergeConflictProjectMetadata, Field: field, Base: base, Local: changed[0], Current: changed[1],
		Detail: fmt.Sprintf("%d competing values; %q is kept", len(changed), keep),
	}
}

// reconcileProjectComments merges comments by id: deletion never beats an
// edit, and competing edits keep the primary side's version.
func reconcileProjectComments(base []*models.ProjectComment, sides []*ProjectDocument) ([]*models.ProjectComment, []MergeConflict) {
	baseByID := indexProjectComments(base)
	sideByID := make([]map[string]*models.ProjectComment, len(sides))
	ids := map[string]bool{}
	for id := range baseByID {
		ids[id] = true
	}
	for i, side := range sides {
		sideByID[i] = indexProjectComments(side.Comments)
		for id := range sideByID[i] {
			ids[id] = true
		}
	}
	var out []*models.ProjectComment
	var conflicts []MergeConflict
	for id := range ids {
		b, inBase := baseByID[id]
		var changed []*models.ProjectComment
		deleted := false
		for _, side := range sideByID {
			comment, ok := side[id]
			switch {
			case !ok:
				deleted = deleted || inBase
			case inBase && sameComment(b, comment):
			default:
				duplicate := false
				for _, prior := range changed {
					duplicate = duplicate || sameComment(prior, comment)
				}
				if !duplicate {
					changed = append(changed, comment)
				}
			}
		}
		switch {
		case len(changed) == 0 && (deleted || !inBase):
			continue
		case len(changed) == 0:
			out = append(out, cloneComment(b))
			continue
		}
		if len(changed) > 1 || (deleted && inBase) {
			conflicts = append(conflicts, MergeConflict{
				Kind: MergeConflictProjectMetadata, CommentID: id, Field: "comment",
				Detail: fmt.Sprintf("%d competing versions (deleted elsewhere: %t); the primary version is kept", len(changed), deleted),
			})
		}
		out = append(out, cloneComment(changed[0]))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	sort.SliceStable(conflicts, func(i, j int) bool { return conflicts[i].CommentID < conflicts[j].CommentID })
	return out, conflicts
}

func sameComment(a, b *models.ProjectComment) bool {
	return a.Text == b.Text && a.CreatedAt.Equal(b.CreatedAt)
}

func cloneComment(c *models.ProjectComment) *models.ProjectComment {
	copied := *c
	return &copied
}

func sideTasks(sides []*ProjectDocument) [][]*models.Task {
	out := make([][]*models.Task, len(sides))
	for i, side := range sides {
		out[i] = side.Tasks
	}
	return out
}

func sideNames(sides []*ProjectDocument) []string {
	out := make([]string, len(sides))
	for i, side := range sides {
		out[i] = side.ProjectName
	}
	return out
}
