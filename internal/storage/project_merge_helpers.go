package storage

import (
	"fmt"
	"reflect"

	"github.com/sglavoie/tt/internal/models"
)

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
