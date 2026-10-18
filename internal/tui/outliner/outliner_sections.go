package outliner

import (
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/query"
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

func (m *OutlinerModel) flattenSectionForest(
	forest *sectionTaskForest,
	projectID string,
	group OutlinerRowGroup,
	rows *[]OutlinerRow,
) {
	if forest == nil {
		return
	}
	var walk func([]*models.Task, []bool)
	walk = func(tasks []*models.Task, ancestorIsLast []bool) {
		for i, task := range tasks {
			isLast := i == len(tasks)-1
			prefix := BuildTreePrefix(ancestorIsLast, isLast)
			children := forest.children[task.ID]
			breadcrumb := ""
			if len(ancestorIsLast) == 0 && task.ParentID != nil {
				breadcrumb = taskAncestorBreadcrumb(task)
			}
			*rows = append(*rows, OutlinerRow{
				Task:               task,
				Group:              group,
				Prefix:             prefix,
				Breadcrumb:         breadcrumb,
				HasVisibleChildren: len(children) > 0,
				ProjectID:          projectID,
			})
			identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
			if identity.ProjectID == "" && m.Project != nil {
				identity.ProjectID = m.Project.ID
			}
			if len(children) == 0 {
				continue
			}
			if !m.IsExpanded(identity) {
				if rows == &m.Rows {
					m.hideSectionForestTasks(forest, children)
				}
				continue
			}
			next := append(append([]bool(nil), ancestorIsLast...), isLast)
			walk(children, next)
		}
	}
	walk(forest.roots, nil)
}

// reviewLaneKey names the Review lane on the synthetic rows that carry it. It
// is a lane identity, not a tag: review is a stored boolean on the task, and
// nothing here reads a tag list.
const reviewLaneKey = "review"

const (
	reviewSectionCollapseKey           = "_promoted:review"
	reviewProjectSectionCollapsePrefix = "_promoted_project:review:"
)

type reviewSectionBuild struct {
	groups    []projectRowGroup
	count     int
	projected map[taskref.TaskIdentity]bool
}

// buildReviewSection promotes tasks that currently need attention into the
// sparse All-mode Review lane. Pinning and explicit Today/Next membership are
// stronger claims for every candidate. Doing is stronger only for a scheduled
// check-in: a manual review flag keeps its established claim over lifecycle,
// while an automatic reminder stays visible in place on active work.
func (m *OutlinerModel) buildReviewSection(
	input OutlinerModeInput,
	pinned, today, next, doing map[taskref.TaskIdentity]bool,
) reviewSectionBuild {
	now := input.now()
	result := reviewSectionBuild{projected: make(map[taskref.TaskIdentity]bool)}
	var rows []OutlinerRow
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		identity := taskref.For(task, project)
		if task == nil || project == nil ||
			(task.IsDeferredAt(now) && !input.IncludeDeferred) ||
			!models.TaskNeedsAttention(task, now) ||
			pinned[identity] || today[identity] || next[identity] ||
			(!task.Review && doing[identity]) {
			continue
		}
		if input.TaskExclude != nil && input.TaskExclude(task, project) {
			continue
		}
		if input.TaskFilter != nil && !input.TaskFilter(task, project) {
			continue
		}
		row := m.outlinerRowFromModeRow(candidate.row)
		row.PromotedTag = reviewLaneKey
		row.Breadcrumb = taskAncestorBreadcrumb(task)
		row.HasVisibleChildren = false
		result.projected[identity] = true
		result.count++
		rows = append(rows, row)
	}
	m.sortSectionRows(rows)
	result.groups = m.groupSectionRowsByProject(rows, sectionForestOptions{rankRootsBySubtree: true}, m.sectionTaskLead(rows))
	return result
}

func (m *OutlinerModel) appendReviewSection(section reviewSectionBuild) {
	if section.count == 0 || !m.sectionZoomAdmits(reviewSectionCollapseKey) {
		return
	}
	m.Rows = append(m.Rows, OutlinerRow{
		Group:           OutlinerRowGroup{SectionKey: reviewSectionCollapseKey},
		IsSectionHeader: true,
		SectionName:     "Review",
		SectionCount:    section.count,
		CollapseKey:     reviewSectionCollapseKey,
		PromotedTag:     reviewLaneKey,
	})
	if m.sectionCollapsed(reviewSectionCollapseKey) {
		m.hideProjectRowGroups(section.groups)
		return
	}
	m.appendProjectGroupedForests(
		section.groups,
		OutlinerRow{PromotedTag: reviewLaneKey},
		reviewSectionCollapseKey,
		func(projectID string) string {
			return reviewProjectSectionCollapsePrefix + projectID
		},
	)
}

const projectNotesSectionKey = "_synthetic:notes"

// Project notes belong to the project's browsing view, outside its task lanes.
// Search keeps its own ranked note results; task filters, lenses and subtree
// focus must not acquire unrelated notes as apparent matches.
func (m *OutlinerModel) appendProjectNotesSection() {
	project := m.FocusedProjectHeaderOwner()
	if project == nil || project.Archived || m.HasSearchProjection() || m.HasActiveLens() ||
		len(m.Focus.Stack) != 0 || len(m.Focus.CrossStack) != 0 ||
		query.HasActiveCommonFilters(&m.Filter.CommonFilterState) ||
		!m.sectionZoomAdmits(projectNotesSectionKey) {
		return
	}
	rows := m.noteSearchRowsInScope()
	if len(rows) == 0 {
		return
	}
	// Scope filtering owns this slice. Match the Notes surface's newest-first
	// order with a stable filename tie-break, without reordering the shared index.
	slices.SortFunc(rows, func(a, b notes.LocatedNoteRow) int {
		if order := b.Note.ModifiedAt.Compare(a.Note.ModifiedAt); order != 0 {
			return order
		}
		return strings.Compare(a.Note.Filename, b.Note.Filename)
	})
	group := OutlinerRowGroup{SectionKey: projectNotesSectionKey}
	m.Rows = append(m.Rows, OutlinerRow{
		IsSectionHeader: true,
		SectionName:     "Notes",
		SectionCount:    len(rows),
		CollapseKey:     projectNotesSectionKey,
		Group:           group,
	})
	if m.sectionCollapsed(projectNotesSectionKey) {
		return
	}
	payloads := make(map[notes.NoteRowIdentity]outlinerSearchNoteMatch, len(rows))
	for _, row := range rows {
		identity := row.Identity()
		payloads[identity] = outlinerSearchNoteMatch{row: row}
		m.Rows = append(m.Rows, OutlinerRow{IsNoteRow: true, NoteRow: identity, Group: group})
	}
	m.Search.setNoteMatches(payloads)
}
