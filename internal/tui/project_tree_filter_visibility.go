package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// isNonActiveGTDSection returns true for named GTD sections that are excluded
// from the Active view mode (Inbox, Next, Waiting, Someday, Review, Standby).
func isNonActiveGTDSection(section string) bool {
	return section != "" && models.IsGTDSection(section) && section != models.SectionCompleted
}

// applyFilterChange refreshes visibility, prunes stale selections, and clamps the cursor.
// Call this after any filter/view mutation to keep the UI consistent.
func (m *ProjectTreeModel) applyFilterChange() {
	m.resetViewScrollOffset()
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
}

func (m *ProjectTreeModel) updateVisibility() {
	// Reset context node flags
	for _, node := range m.nodes {
		node.IsContextNode = false
	}

	// Pass 1: Determine intrinsic visibility (ignoring parent state)
	// This determines which nodes would be visible based on their own properties
	intrinsicVisible := make(map[string]bool)
	for _, node := range m.nodes {
		intrinsicVisible[node.Task.ID] = m.passesFilters(node)
	}

	// Pass 2: Mark ancestors of intrinsically visible nodes as context nodes
	// This ensures parent hierarchy is shown even when parent is in different section
	for _, node := range m.nodes {
		if intrinsicVisible[node.Task.ID] {
			m.markAncestorsAsContext(node, intrinsicVisible)
		}
	}

	// Pass 2.5: Compute HasFilteredChildren for each node
	// A node has filtered children if any child is intrinsically visible or is a context node
	for _, node := range m.nodes {
		node.HasFilteredChildren = false
		for _, child := range node.Task.Children {
			if childNode := m.findNodeByTaskID(child.ID); childNode != nil {
				if intrinsicVisible[child.ID] || childNode.IsContextNode {
					node.HasFilteredChildren = true
					break
				}
			}
		}
	}

	// Pass 3: Apply final visibility respecting expand/collapse state
	for i, node := range m.nodes {
		parentLevel := node.Level - 1
		visible := intrinsicVisible[node.Task.ID] || node.IsContextNode

		// If we're visible, check if parent is expanded (unless parent is a context node)
		if visible && parentLevel >= 0 {
			for j := i - 1; j >= 0; j-- {
				if m.nodes[j].Level == parentLevel {
					// Parent must be visible and expanded for child to be visible
					if !m.nodes[j].Visible || !m.nodes[j].Expanded {
						visible = false
					}
					break
				}
			}
		}
		node.Visible = visible
	}
	m.clampCursor()
}

// passesFilters checks if a node passes all current filters (view mode, status, etc.)
// without considering parent visibility. This determines "intrinsic" visibility.
func (m *ProjectTreeModel) passesFilters(node *TreeNode) bool { //nolint:gocyclo
	// Handle snoozed view mode
	if m.viewMode == ViewModeSnoozed {
		// In Snoozed view, ONLY show snoozed tasks
		if !node.Task.IsSnoozed() {
			return false
		}
		// Apply search filter in snoozed view
		if m.SearchQuery() != "" {
			if !m.matchesSearch(node.Task) && !m.hasMatchingDescendant(node.Task) {
				return false
			}
		}
		return true
	}

	// Hide snoozed tasks from all other views
	if node.Task.IsSnoozed() {
		return false
	}

	// Handle completion status based on view mode
	if m.viewMode == ViewModeCompleted {
		// In Completed view, ONLY show completed tasks
		if !node.Task.IsComplete() {
			return false
		}
		// Apply section filter for sub-filtering by original section
		if len(m.filter.Sections) > 0 {
			if !m.filter.Sections[node.Task.Section] {
				return false
			}
		}
	} else {
		// In all other views, HIDE completed tasks
		if node.Task.IsComplete() {
			return false
		}
		// Apply view mode section logic
		switch m.viewMode {
		case ViewModeActive:
			if isNonActiveGTDSection(node.Task.Section) {
				return false
			}
		case ViewModeAll:
			// Show all non-completed, non-snoozed tasks (no section filtering)
		default:
			// Single-section view modes (Next, Inbox, Waiting, Someday, Review, Standby)
			if required := viewModeToSection(m.viewMode); required != "" {
				if node.Task.Section != required {
					return false
				}
			}
		}
	}

	// Apply common filters (type, status, priority, tags, due date, attrs, score)
	projectID := ""
	if m.project != nil {
		projectID = m.project.ID
	}
	if !passesCommonFilters(node.Task, &m.filter.CommonFilterState, projectID, m.config) {
		return false
	}

	// Apply criteria filter (from a loaded criteria-mode SavedFilter)
	if m.activeCriteriaFilter != nil {
		if !passesSavedFilterCriteria(node.Task, m.project, m.activeCriteriaFilter, m.config) {
			return false
		}
	}

	// Apply pinned tag tab filter.
	if tagFilter := m.activePinnedTagFilter(); tagFilter != "" {
		if !node.Task.HasTag(tagFilter) {
			return false
		}
	}

	// Apply section filter (OR logic: task must be in ANY selected section)
	// In Completed view, section filter is already applied above for sub-filtering
	// In other views, view mode already controls section visibility
	if m.viewMode != ViewModeCompleted && len(m.filter.Sections) > 0 {
		if !m.filter.Sections[node.Task.Section] {
			return false
		}
	}

	// Apply actionable filter (hide tasks not ready to work on)
	if m.filter.Actionable {
		// Hide tasks with incomplete dependencies
		if m.project != nil && m.project.HasIncompleteDependencies(node.Task) {
			return false
		}
		// Hide manually-blocked tasks
		if node.Task.Status == models.StatusBlocked {
			return false
		}
		// Hide Inbox/Next/Waiting/Someday/Review/Standby tasks (GTD-aligned - not immediately actionable)
		if isNonActiveGTDSection(node.Task.Section) {
			return false
		}
	}

	// Apply search filter
	if m.SearchQuery() != "" {
		if !m.matchesSearch(node.Task) && !m.hasMatchingDescendant(node.Task) {
			return false
		}
	}

	return true
}

// markAncestorsAsContext walks up the ancestor chain and marks invisible ancestors
// as context nodes so they appear (dimmed) to preserve hierarchy visibility.
func (m *ProjectTreeModel) markAncestorsAsContext(node *TreeNode, intrinsicVisible map[string]bool) {
	task := node.Task
	if task.Parent == nil {
		return
	}

	// Find parent node
	for _, n := range m.nodes {
		if n.Task.ID == *task.ParentID {
			// Only mark as context if parent isn't intrinsically visible
			if !intrinsicVisible[n.Task.ID] && !n.IsContextNode {
				n.IsContextNode = true
			}
			// Recursively mark grandparents
			m.markAncestorsAsContext(n, intrinsicVisible)
			break
		}
	}
}

func (m *ProjectTreeModel) matchesSearch(task *models.Task) bool {
	return TaskMatchesStructuredSearch(task, m.SearchQuery())
}

// getSearchMatchPositions returns match positions for the title if there's a search query.
// This is called during rendering to get positions for highlighting.
func (m *ProjectTreeModel) getSearchMatchPositions(task *models.Task) []int {
	if m.SearchQuery() == "" {
		return nil
	}
	query := strings.TrimSpace(ExtractSearchTextQuery(m.SearchQuery()))
	if query == "" {
		return nil
	}
	_, positions := fuzzy.ScoreTermsWithPositions(task.Title, query)
	return positions
}

func (m *ProjectTreeModel) hasMatchingDescendant(task *models.Task) bool {
	for _, child := range task.Children {
		if m.matchesSearch(child) || m.hasMatchingDescendant(child) {
			return true
		}
	}
	return false
}
