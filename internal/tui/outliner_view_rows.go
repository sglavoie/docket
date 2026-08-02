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
	task               *models.Task
	prefix             string
	Breadcrumb         string                    // Ancestor path for tasks promoted across a section boundary.
	hasVisibleChildren bool                      // Children rendered in this row's section-local tree.
	IsSectionHeader    bool                      // True if this row is a section header (not a task)
	IsComputedHeader   bool                      // True for a computed-mode grouping header such as a due-date bucket
	IsProjectHeader    bool                      // True if this row is a synthetic, render-only project sub-header (not a task)
	IsPinnedSection    bool                      // True for the synthetic Pinned header and every promoted task row beneath it
	IsTodaySection     bool                      // True for the synthetic Today header and every explicit Today task row beneath it
	IsNextSection      bool                      // True for the synthetic Next header and every explicit Next task row beneath it
	IsTodayMember      bool                      // True only for explicit Today members
	SectionName        string                    // Section name for header rows
	SectionCount       int                       // Number of tasks in this section
	StateGroup         models.WorkflowStateGroup // Canonical state group for workflow headers; empty for unrelated/computed headers
	CollapseKey        string                    // Stable identity for shared or project-owned grouping headers
	ProjectID          string                    // Project that owns task/project-header rows; empty for shared section headers
	IsSeparator        bool                      // True if this row is a visual separator between projects
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

// isSectionRowCollapsed reports whether a section-header row is collapsed.
func (m *OutlinerModel) isSectionRowCollapsed(row outlinerRow) bool {
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

// flattenTaskTreeWithFolds recursively walks the task tree and produces a flat
// list of outlinerRows, respecting fold state. Collapsed nodes (not in
// expandedNodes or explicitly false) stop recursion, showing only the parent.
// When expandedNodes is empty (default), all nodes are expanded.
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
