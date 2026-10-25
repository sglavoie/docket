package storage

import (
	"fmt"
	"sort"

	"github.com/sglavoie/docket/internal/models"
)

// repairHierarchy keeps every surviving task's parent, then reverts merged
// reparenting that cycles or nests too deep, then, as a last resort, lifts a
// task still in an invalid chain to the root.
func (r *projectReconciler) repairHierarchy() {
	r.restoreMissingParents()
	if invalid := r.invalidHierarchy(); len(invalid) > 0 {
		reverted := false
		for _, id := range sortedTaskIDs(r.merged) {
			task := r.merged[id]
			b, inBase := r.baseByID[id]
			if !inBase || sameParent(task.ParentID, b.ParentID) {
				continue
			}
			task.ParentID = cloneParent(b.ParentID)
			reverted = true
		}
		if reverted {
			r.conflicts = append(r.conflicts, MergeConflict{
				Kind:   MergeConflictHierarchy,
				Field:  "parent_id",
				Detail: fmt.Sprintf("concurrent reparenting composed an invalid hierarchy at %v; reparenting reverted to the common base", invalid),
			})
		}
		r.restoreMissingParents()
	}
	for _, id := range r.invalidHierarchy() {
		task := r.merged[id]
		task.ParentID = nil
		r.conflicts = append(r.conflicts, MergeConflict{
			Kind:      MergeConflictHierarchy,
			TaskTitle: task.Title,
			Field:     "parent_id",
			Detail:    "the task's merged parent chain stayed invalid; it was moved to the root",
		})
	}
}

// restoreMissingParents brings back, from the base or a side, any parent a
// surviving task still names.
func (r *projectReconciler) restoreMissingParents() {
	for changed := true; changed; {
		changed = false
		for _, id := range sortedTaskIDs(r.merged) {
			task := r.merged[id]
			if task.ParentID == nil {
				continue
			}
			parentID := *task.ParentID
			if _, ok := r.merged[parentID]; ok {
				continue
			}
			restored := r.baseByID[parentID]
			for i := 0; restored == nil && i < len(r.sideByID); i++ {
				restored = r.sideByID[i][parentID]
			}
			if restored == nil {
				task.ParentID = nil
				r.conflicts = append(r.conflicts, MergeConflict{
					Kind: MergeConflictHierarchy, TaskTitle: task.Title, Field: "parent_id",
					Detail: fmt.Sprintf("parent %q exists in no version; the task was moved to the root", parentID),
				})
			} else {
				r.merged[parentID] = cloneTask(restored)
				r.conflicts = append(r.conflicts, MergeConflict{
					Kind: MergeConflictDeleteEdit, TaskTitle: restored.Title, Field: "task",
					Detail: fmt.Sprintf("a task deleted concurrently still has %q beneath it; it is kept", task.Title),
				})
			}
			changed = true
		}
	}
}

// invalidHierarchy lists the tasks whose parent chain cycles or nests deeper
// than models.MaxNestingDepth, in id order.
func (r *projectReconciler) invalidHierarchy() []string {
	var invalid []string
	for _, id := range sortedTaskIDs(r.merged) {
		depth := 0
		seen := map[string]bool{id: true}
		for current := r.merged[id]; current != nil && current.ParentID != nil; depth++ {
			parentID := *current.ParentID
			if seen[parentID] || depth >= models.MaxNestingDepth {
				invalid = append(invalid, id)
				break
			}
			seen[parentID] = true
			current = r.merged[parentID]
		}
	}
	return invalid
}

// renumberShortIDs gives every task a distinct short id. A number two
// concurrent creations both minted stays with the task that held it in the
// base, else with the smaller UUID; every other holder is renumbered, in UUID
// order, past the largest number in the document.
func (r *projectReconciler) renumberShortIDs() {
	holders := map[int][]string{}
	maxID := 0
	for _, id := range sortedTaskIDs(r.merged) {
		short := r.merged[id].ShortID
		holders[short] = append(holders[short], id)
		if short > maxID {
			maxID = short
		}
	}
	var renumber []string
	for short, ids := range holders {
		if len(ids) < 2 {
			continue
		}
		keeper := ids[0]
		for _, id := range ids {
			if b, ok := r.baseByID[id]; ok && b.ShortID == short {
				keeper = id
				break
			}
		}
		for _, id := range ids {
			if id != keeper {
				renumber = append(renumber, id)
			}
		}
	}
	sort.Strings(renumber)
	for _, id := range renumber {
		maxID++
		r.merged[id].ShortID = maxID
	}
}

// mergeOrder merges each sibling group's order across all sides: one side's
// reorder, or several compatible ones, is adopted; incompatible reorders keep
// the base order. Additions follow in side order.
func (r *projectReconciler) mergeOrder() []string {
	all := append([][]*models.Task{r.base.Tasks}, sideTasks(r.sides)...)
	backbone := survivingTaskOrder(r.merged, all...)
	groupMembers := map[string]map[string]bool{}
	for _, id := range backbone {
		group := taskSiblingGroup(r.merged[id])
		if groupMembers[group] == nil {
			groupMembers[group] = map[string]bool{}
		}
		groupMembers[group][id] = true
	}
	groups := make([]string, 0, len(groupMembers))
	for group := range groupMembers {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	mergedGroups := map[string][]string{}
	for _, group := range groups {
		members := groupMembers[group]
		baseIDs := documentSiblingOrder(r.base.Tasks, group, members)
		sequences := make([][]string, 0, len(r.sides))
		var reordered [][]string
		for _, side := range r.sides {
			ids := documentSiblingOrder(side.Tasks, group, members)
			sequences = append(sequences, ids)
			if relativeOrderChanged(baseIDs, ids) {
				reordered = append(reordered, ids)
			}
		}
		authoritative := baseIDs
		if len(reordered) > 0 {
			authoritative = reordered[0]
			for _, other := range reordered[1:] {
				shared := intersectionSet(authoritative, other)
				if !stringSlicesEqual(filterOrder(authoritative, shared), filterOrder(other, shared)) {
					authoritative = baseIDs
					label := "root tasks"
					if group != "" {
						label = fmt.Sprintf("children of task %q", group)
					}
					r.conflicts = append(r.conflicts, MergeConflict{
						Kind: MergeConflictHierarchy, Field: "order",
						Detail: fmt.Sprintf("concurrent reorders of %s are incompatible; the base order is kept", label),
					})
					break
				}
			}
		}
		parts := append([][]string{authoritative}, sequences...)
		parts = append(parts, baseIDs, filterOrder(backbone, members))
		mergedGroups[group] = completeSiblingOrder(parts...)
	}
	next := map[string]int{}
	ordered := make([]string, 0, len(backbone))
	for _, id := range backbone {
		group := taskSiblingGroup(r.merged[id])
		ordered = append(ordered, mergedGroups[group][next[group]])
		next[group]++
	}
	return ordered
}

// reconcileScalar merges one project-level string: one change is adopted,
// competing changes keep the base value.

func sortedTaskIDs(tasks map[string]*models.Task) []string {
	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sameParent(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func cloneParent(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
