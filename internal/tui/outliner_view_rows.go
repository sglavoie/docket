package tui

import (
	"strings"

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
	task            *models.Task
	prefix          string
	IsSectionHeader bool   // True if this row is a section header (not a task)
	SectionName     string // Section name for header rows
	SectionCount    int    // Number of tasks in this section
	CollapseKey     string // Key for collapsedSections map; "_xp:sectionName" sentinel in All-perspective cross-project mode (shared headers, no per-project prefix needed), empty in single-project mode
	ProjectID       string // Project that owns this row; set on task rows and cross-project section headers
	IsSeparator     bool   // True if this row is a visual separator between projects
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

// projectSlugByID looks up a project slug from m.loadedProjects by ID.
// Falls back to projectNameByID when the slug is empty.
func (m *OutlinerModel) projectSlugByID(id string) string {
	for _, p := range m.loadedProjects {
		if p.ID == id {
			if p.Slug != "" {
				return p.Slug
			}
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

// flattenTaskTree recursively walks the task tree and produces a flat list of
// outlinerRows with pre-computed connector prefixes. The ancestorIsLast stack
// tracks whether each ancestor is its parent's last child, which determines
// whether continuation pipes or spaces are drawn.
func flattenTaskTree(tasks []*models.Task, ancestorIsLast []bool, rows *[]outlinerRow) {
	for i, task := range tasks {
		isLast := i == len(tasks)-1
		prefix := ""
		if len(ancestorIsLast) > 0 || len(tasks) > 0 {
			prefix = buildTreePrefix(ancestorIsLast, isLast)
		}
		*rows = append(*rows, outlinerRow{task: task, prefix: prefix})

		if len(task.Children) > 0 {
			// Copy before appending to prevent slice aliasing: without the
			// copy, siblings that recurse into children could share the same
			// backing array, corrupting each other's ancestor trail when the
			// capacity isn't exhausted.
			child := make([]bool, len(ancestorIsLast)+1)
			copy(child, ancestorIsLast)
			child[len(ancestorIsLast)] = isLast
			flattenTaskTree(task.Children, child, rows)
		}
	}
}

// matchesSearch parses structured filters from the query and fuzzy-matches
// the remaining text against the task's title, tags, description, and comments.
func matchesSearch(task *models.Task, query string) bool {
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
		*rows = append(*rows, outlinerRow{task: task, prefix: prefix})

		if len(task.Children) > 0 && m.isExpanded(task.ID) {
			// Copy before appending to prevent slice aliasing (see flattenTaskTree).
			child := make([]bool, len(ancestorIsLast)+1)
			copy(child, ancestorIsLast)
			child[len(ancestorIsLast)] = isLast
			m.flattenTaskTreeWithFolds(task.Children, child, rows)
		}
	}
}

// isExpanded returns true if the node should show its children.
// When the expandedNodes map is empty (default), all nodes are expanded.
func (m *OutlinerModel) isExpanded(taskID string) bool {
	if len(m.expandedNodes) == 0 {
		return true
	}
	expanded, exists := m.expandedNodes[taskID]
	if !exists {
		return true // Default to expanded
	}
	return expanded
}
