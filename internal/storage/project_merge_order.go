package storage

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

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
