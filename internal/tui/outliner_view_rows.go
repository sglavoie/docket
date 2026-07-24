package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// Tree connector characters for outliner rendering.
const (
	connectorBranch   = "├── "
	connectorLastItem = "└── "
	connectorPipe     = "│   "
	connectorSpace    = "    "
)

// outlinerRow represents a single flattened row for rendering.
type outlinerRow struct {
	task               *models.Task
	priorityTask       *PriorityTask // Cached priority metadata for flat Priority-mode task rows.
	prefix             string
	Breadcrumb         string // Ancestor path for tasks promoted across a section boundary.
	hasVisibleChildren bool   // Children rendered in this row's section-local tree.
	IsSectionHeader    bool   // True if this row is a section header (not a task)
	IsLensHeader       bool   // True for a non-GTD grouping header such as a due-date bucket
	IsProjectHeader    bool   // True if this row is a synthetic, render-only project sub-header (not a task)
	SectionName        string // Section name for header rows
	SectionCount       int    // Number of tasks in this section
	CollapseKey        string // Key for collapsedSections map; "_xp:sectionName" sentinel in All-perspective cross-project mode (shared headers, no per-project prefix needed), empty in single-project mode
	ProjectID          string // Project that owns task/project-header rows; empty for shared section headers
	IsSeparator        bool   // True if this row is a visual separator between projects
}

// isTaskRow reports whether the row carries a task payload (i.e. it is not a
// section header, project sub-header, or separator). Use this in place of the
// historical `!IsSectionHeader && !IsSeparator` pattern so that newly
// introduced row kinds (e.g. project sub-headers) are consistently excluded
// from task-only logic.
func (r outlinerRow) isTaskRow() bool {
	return !r.IsSectionHeader && !r.IsProjectHeader && !r.IsSeparator
}

// isNavigable reports whether the cursor is allowed to rest on this row.
// Section headers remain navigable (Enter toggles section fold); task rows
// are navigable; project sub-headers are navigable so Enter can scope to the
// project and Shift+Enter can open its administration menu. Only separators
// (purely decorative) are skipped by cursor movement and rebuild/clamp paths.
func (r outlinerRow) isNavigable() bool {
	return !r.IsSeparator
}

// collapseKey returns the effective key for collapsedSections lookups.
// Cross-project headers set CollapseKey to "projectID:sectionName"; single-project
// headers leave it empty and fall back to SectionName.
func (r outlinerRow) effectiveCollapseKey() string {
	if r.CollapseKey != "" {
		return r.CollapseKey
	}
	return r.SectionName
}

// isSectionRowCollapsed reports whether a section-header row is currently
// collapsed. Cross-project Priority mode headers use the session-only
// crossPriorityCollapsedGroups map; every other header reads from the persisted
// collapsedSections map.
func (m *OutlinerModel) isSectionRowCollapsed(row outlinerRow) bool {
	if g, ok := priorityGroupFromCollapseKey(row.CollapseKey); ok {
		return m.crossPriorityCollapsedGroups[g]
	}
	if isReviewCollapseKey(row.CollapseKey) {
		return m.reviewCollapsedSections[row.CollapseKey]
	}
	return m.collapsedSections[row.effectiveCollapseKey()]
}

// projectNameByID looks up a project name from m.loadedProjects by ID.
// Returns the ID itself as a fallback if the project is not found.
func (m *OutlinerModel) projectNameByID(id string) string {
	for _, p := range m.loadedProjects {
		if p.ID == id {
			return p.Name
		}
	}
	return id
}

// buildTreePrefix computes the connector prefix string from the ancestor
// is-last-child boolean stack. Each true entry means that ancestor was the
// last child of its parent (so we render spaces), while false means we need
// a continuation pipe.
func buildTreePrefix(ancestorIsLast []bool, isLast bool) string {
	var b strings.Builder
	for _, last := range ancestorIsLast {
		if last {
			b.WriteString(connectorSpace)
		} else {
			b.WriteString(connectorPipe)
		}
	}
	if isLast {
		b.WriteString(connectorLastItem)
	} else {
		b.WriteString(connectorBranch)
	}
	return b.String()
}

// matchesSearch parses structured filters from the query and fuzzy-matches
// the remaining text against the task's title, tags, description, and comments.
func matchesSearch(task *models.Task, query string, _ *config.PlanningState) bool {
	return TaskMatchesStructuredSearch(task, query)
}

// flattenTaskTreeWithFolds recursively walks the task tree and produces a flat
// list of outlinerRows, respecting fold state. Collapsed nodes (not in
// expandedNodes or explicitly false) stop recursion, showing only the parent.
// When expandedNodes is empty (default), all nodes are expanded.
func (m *OutlinerModel) flattenTaskTreeWithFolds(tasks []*models.Task, ancestorIsLast []bool, rows *[]outlinerRow) {
	for i, task := range tasks {
		isLast := i == len(tasks)-1
		prefix := ""
		if len(ancestorIsLast) > 0 || len(tasks) > 0 {
			prefix = buildTreePrefix(ancestorIsLast, isLast)
		}
		*rows = append(*rows, outlinerRow{task: task, prefix: prefix, hasVisibleChildren: len(task.Children) > 0})

		identity := TaskIdentity{TaskID: task.ID}
		if m.project != nil {
			identity.ProjectID = m.project.ID
		}
		if len(task.Children) > 0 && m.isExpanded(identity) {
			// Copy before appending to prevent slice aliasing.
			child := make([]bool, len(ancestorIsLast)+1)
			copy(child, ancestorIsLast)
			child[len(ancestorIsLast)] = isLast
			m.flattenTaskTreeWithFolds(task.Children, child, rows)
		}
	}
}

// insertDepth0Separators inserts a blank separator row between depth-0 task
// groups that have visible children. Only separator rows are added — never at
// the start or end of the list.
func (m *OutlinerModel) insertDepth0Separators() {
	if len(m.rows) <= 1 {
		return
	}
	isRoot := func(prefix string) bool {
		return strings.HasPrefix(prefix, connectorBranch) ||
			strings.HasPrefix(prefix, connectorLastItem)
	}
	prevWasChild := false
	result := make([]outlinerRow, 0, len(m.rows)+4)
	for _, row := range m.rows {
		if row.task != nil && !row.IsSectionHeader {
			rootRow := isRoot(row.prefix)
			if rootRow && prevWasChild {
				result = append(result, outlinerRow{IsSeparator: true})
			}
			prevWasChild = !rootRow
		}
		result = append(result, row)
	}
	m.rows = result
}

// isExpanded returns true if the node should show its children.
// When the expandedNodes map is empty (default), all nodes are expanded.
func (m *OutlinerModel) isExpanded(identity TaskIdentity) bool {
	if len(m.expandedNodes) == 0 {
		return true
	}
	expanded, exists := m.expandedNodes[identity]
	if !exists {
		return true // Default to expanded
	}
	return expanded
}
