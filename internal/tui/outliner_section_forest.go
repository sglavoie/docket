package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// sectionTaskForest is a presentation-only hierarchy containing tasks from one
// authoritative state group. Cross-group children become roots while
// same-group parent/child relationships retain normal tree rendering.
type sectionTaskForest struct {
	roots    []*models.Task
	children map[string][]*models.Task
	count    int
}

// buildStateGroupTaskForests builds the canonical All-mode forests without
// passing through legacy section buckets. Parent/child edges survive only when
// both tasks share the same state group; a cross-group child becomes a root in
// its own group and carries an ancestor breadcrumb when flattened.
func buildStateGroupTaskForests(tasks []*models.Task) map[models.WorkflowStateGroup]*sectionTaskForest {
	return buildTaskForestsByStateGroup(tasks, theme.TaskStateGroup)
}

func buildTaskForestsByStateGroup(
	tasks []*models.Task,
	classify func(*models.Task) models.WorkflowStateGroup,
) map[models.WorkflowStateGroup]*sectionTaskForest {
	ordered := flattenTaskPointers(tasks)
	included := make(map[string]*models.Task, len(ordered))
	for _, task := range ordered {
		included[task.ID] = task
	}

	forests := make(map[models.WorkflowStateGroup]*sectionTaskForest)
	for _, task := range ordered {
		group := classify(task)
		if !group.IsValid() {
			continue
		}
		forest := forests[group]
		if forest == nil {
			forest = &sectionTaskForest{children: make(map[string][]*models.Task)}
			forests[group] = forest
		}
		forest.count++
		if task.ParentID != nil {
			if parent := included[*task.ParentID]; parent != nil && classify(parent) == group {
				forest.children[parent.ID] = append(forest.children[parent.ID], task)
				continue
			}
		}
		forest.roots = append(forest.roots, task)
	}
	return forests
}

// tasks returns every task the forest holds. It walks the forest's own child
// index rather than Task.Children so descendants belonging to another state
// group stay out.
func (f *sectionTaskForest) tasks() []*models.Task {
	if f == nil {
		return nil
	}
	tasks := make([]*models.Task, 0, f.count)
	var walk func([]*models.Task)
	walk = func(nodes []*models.Task) {
		for _, task := range nodes {
			tasks = append(tasks, task)
			walk(f.children[task.ID])
		}
	}
	walk(f.roots)
	return tasks
}

func flattenTaskPointers(tasks []*models.Task) []*models.Task {
	var result []*models.Task
	var walk func([]*models.Task)
	walk = func(nodes []*models.Task) {
		for _, task := range nodes {
			if task == nil {
				continue
			}
			result = append(result, task)
			walk(task.Children)
		}
	}
	walk(tasks)
	return result
}

func taskAncestorBreadcrumb(task *models.Task) string {
	return taskAncestorBreadcrumbAfter(task, "")
}

// taskAncestorBreadcrumbAfter returns the task's ancestor path below an
// optional boundary. The boundary itself and ancestors above it are omitted.
func taskAncestorBreadcrumbAfter(task *models.Task, boundaryID string) string {
	if task == nil || task.Parent == nil {
		return ""
	}
	var reversed []string
	seen := make(map[string]bool)
	for parent := task.Parent; parent != nil && !seen[parent.ID]; parent = parent.Parent {
		if boundaryID != "" && parent.ID == boundaryID {
			break
		}
		seen[parent.ID] = true
		reversed = append(reversed, parent.Title)
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	if len(reversed) == 0 {
		return ""
	}
	return strings.Join(reversed, " › ") + " ›"
}

func (m *OutlinerModel) flattenSectionForest(forest *sectionTaskForest, projectID string, rows *[]outlinerRow) {
	if forest == nil {
		return
	}
	var walk func([]*models.Task, []bool)
	walk = func(tasks []*models.Task, ancestorIsLast []bool) {
		for i, task := range tasks {
			isLast := i == len(tasks)-1
			prefix := buildTreePrefix(ancestorIsLast, isLast)
			children := forest.children[task.ID]
			breadcrumb := ""
			if len(ancestorIsLast) == 0 && task.ParentID != nil {
				breadcrumb = taskAncestorBreadcrumb(task)
			}
			*rows = append(*rows, outlinerRow{
				task:               task,
				prefix:             prefix,
				Breadcrumb:         breadcrumb,
				hasVisibleChildren: len(children) > 0,
				ProjectID:          projectID,
			})
			identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
			if identity.ProjectID == "" && m.project != nil {
				identity.ProjectID = m.project.ID
			}
			if len(children) > 0 && m.isExpanded(identity) {
				next := append(append([]bool(nil), ancestorIsLast...), isLast)
				walk(children, next)
			}
		}
	}
	walk(forest.roots, nil)
}
