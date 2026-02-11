package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// toggleAllView toggles the all view mode.
// When active, the normal 5-column view is replaced with a single full-width
// list of all non-completed, non-snoozed tasks from all sections.
func (m *KanbanViewModel) toggleAllView() {
	m.allViewMode = !m.allViewMode
	if m.allViewMode {
		m.buildAllNodes()
		m.allCursor = 0
		m.allScroll = 0
		m.statusMessage = "All view (g*/F1/q to exit)"
	} else {
		m.allNodes = nil
		m.statusMessage = ""
	}
}

// buildAllNodes builds the list of all tasks for the all view.
// Includes all non-completed, non-snoozed tasks from all sections, preserving hierarchy.
func (m *KanbanViewModel) buildAllNodes() {
	m.allNodes = nil
	if m.Project == nil {
		return
	}

	// Build a set of eligible task IDs for hierarchy lookup
	eligibleIDs := make(map[string]bool)
	for _, task := range m.Project.Tasks {
		if !task.Status.IsComplete() && !task.IsSnoozed() {
			// Apply filters
			if m.passesFilters(task) {
				eligibleIDs[task.ID] = true
			}
		}
	}

	// Find root tasks (no parent or parent not in eligible set)
	var rootTasks []*KanbanNode
	for _, task := range m.Project.Tasks {
		if !eligibleIDs[task.ID] {
			continue
		}
		isRoot := task.ParentID == nil || !eligibleIDs[*task.ParentID]
		if isRoot {
			rootTasks = append(rootTasks, &KanbanNode{
				Task:    task,
				Level:   0,
				Visible: true,
			})
		}
	}

	// Add nodes recursively with hierarchy
	for _, root := range rootTasks {
		m.addAllNode(root, 0, eligibleIDs)
	}
}

// addAllNode recursively adds a task and its children to the all view.
func (m *KanbanViewModel) addAllNode(node *KanbanNode, level int, eligibleIDs map[string]bool) {
	// Cap level at MaxKanbanDisplayLevel
	displayLevel := level
	if displayLevel > MaxKanbanDisplayLevel {
		displayLevel = MaxKanbanDisplayLevel
	}

	// Check if this task has eligible children
	hasEligibleChildren := false
	for _, child := range node.Task.Children {
		if eligibleIDs[child.ID] {
			hasEligibleChildren = true
			break
		}
	}

	node.Level = displayLevel
	node.HasChildrenHere = hasEligibleChildren

	// Get or default expand state (default = expanded)
	expanded := true
	if state, ok := m.ExpandState[node.Task.ID]; ok {
		expanded = state
	} else {
		m.ExpandState[node.Task.ID] = true
	}
	node.Expanded = expanded
	node.Visible = true

	m.allNodes = append(m.allNodes, node)

	// Add children if expanded
	if expanded {
		for _, child := range node.Task.Children {
			if eligibleIDs[child.ID] {
				childNode := &KanbanNode{
					Task:    child,
					Level:   displayLevel + 1,
					Visible: true,
				}
				m.addAllNode(childNode, level+1, eligibleIDs)
			}
		}
	}
}

// handleAllViewKey handles key input in all view mode.
func (m *KanbanViewModel) handleAllViewKey(key string) (bool, interface{}) {
	switch key {
	case "j", "down":
		// Move cursor down
		if len(m.allNodes) > 0 && m.allCursor < len(m.allNodes)-1 {
			m.allCursor++
		}
		return true, nil

	case "k", "up":
		// Move cursor up
		if m.allCursor > 0 {
			m.allCursor--
		}
		return true, nil

	case "g":
		// gg - go to first (handled by prefix)
		m.pendingPrefix = "g"
		return true, nil

	case "m":
		// Move prefix (send-to-section, move to top/bottom)
		m.pendingPrefix = "m"
		return true, nil

	case "G":
		// Go to last
		if len(m.allNodes) > 0 {
			m.allCursor = len(m.allNodes) - 1
		}
		return true, nil

	case "q", "esc":
		// Exit all view
		m.toggleAllView()
		return true, nil

	case " ":
		// Toggle expand/collapse
		if m.allCursor < len(m.allNodes) {
			node := m.allNodes[m.allCursor]
			if node.HasChildrenHere {
				m.ExpandState[node.Task.ID] = !node.Expanded
				m.buildAllNodes()
				// Try to keep cursor on same task
				for i, n := range m.allNodes {
					if n.Task.ID == node.Task.ID {
						m.allCursor = i
						break
					}
				}
			}
		}
		return true, nil

	// Tab row focus switching
	case "(", ")":
		if tabs := m.customSectionTabs(); tabs != nil {
			m.customSectionRowFocused = !m.customSectionRowFocused
		}
		return true, nil

	// Tab switching within focused row
	case "[":
		if m.customSectionRowFocused {
			if tabs := m.customSectionTabs(); tabs != nil {
				if m.customSectionTab > 0 {
					m.customSectionTab--
				} else {
					m.customSectionTab = len(tabs) - 1
				}
				m.buildAllNodes()
				m.allCursor = 0
			}
		} else {
			if m.activeTab > 0 {
				m.applyTabPresetForAllView(m.activeTab - 1)
			} else {
				m.applyTabPresetForAllView(len(kanbanTabNames) - 1)
			}
		}
		return true, nil

	case "]":
		if m.customSectionRowFocused {
			if tabs := m.customSectionTabs(); tabs != nil {
				if m.customSectionTab < len(tabs)-1 {
					m.customSectionTab++
				} else {
					m.customSectionTab = 0
				}
				m.buildAllNodes()
				m.allCursor = 0
			}
		} else {
			if m.activeTab < len(kanbanTabNames)-1 {
				m.applyTabPresetForAllView(m.activeTab + 1)
			} else {
				m.applyTabPresetForAllView(0)
			}
		}
		return true, nil
	}

	return false, nil
}

// applyTabPresetForAllView applies the preset filter and rebuilds all nodes.
func (m *KanbanViewModel) applyTabPresetForAllView(tab int) {
	m.activeTab = tab
	// Reset manual filters
	m.ClearAllFilters()
	m.actionableFilter = false

	switch tab {
	case kanbanTabActionable:
		m.actionableFilter = true
	case kanbanTabHighPri:
		m.filter.Priority = 1
	case kanbanTabEpics:
		m.filter.Type = models.TaskTypeEpic
	case kanbanTabStories:
		m.filter.Type = models.TaskTypeStory
	case kanbanTabTasks:
		m.filter.Type = models.TaskTypeTask
	case kanbanTabBugs:
		m.filter.Type = models.TaskTypeBug
	}

	m.buildAllNodes()
	m.allCursor = 0
}

// renderAllView renders the full-width all tasks view.
func (m *KanbanViewModel) renderAllView() string {
	var b strings.Builder

	// Render custom section tab bar above preset tabs (when custom sections exist)
	hasCustomSections := m.customSectionTabs() != nil
	if hasCustomSections {
		csBar := RenderColoredTabBar(m.customSectionTabs(), m.customSectionTab, m.Width()-1, 1, m.Project.SectionColors)
		if m.customSectionRowFocused {
			focusIndicator := SelectionCursorStyle.Render("▐")
			csBar = focusIndicator + csBar
		} else {
			csBar = " " + csBar
		}
		b.WriteString(csBar)
		b.WriteString("\n")
	}

	// Render tab bar at the top
	presetBar := m.renderKanbanTabBar()
	if hasCustomSections {
		if !m.customSectionRowFocused {
			focusIndicator := SelectionCursorStyle.Render("▐")
			presetBar = focusIndicator + presetBar
		} else {
			presetBar = " " + presetBar
		}
	}
	b.WriteString(presetBar)
	b.WriteString("\n")

	// Header showing "All Sections" indicator
	headerStyle := ViewModeAllStyle
	header := headerStyle.Render(fmt.Sprintf("All Sections (%d)", len(m.allNodes)))
	b.WriteString(header)
	b.WriteString("\n")

	// Calculate viewport height
	tabBarHeight := TabBarHeight
	if hasCustomSections {
		tabBarHeight += TabBarHeight
	}
	viewportHeight := m.Height() - tabBarHeight - 3 // header + status + help hint

	if len(m.allNodes) == 0 {
		b.WriteString(MutedStyle.Render("No tasks"))
		return b.String()
	}

	// Adjust scroll to keep cursor visible
	if m.allCursor < m.allScroll {
		m.allScroll = m.allCursor
	}
	if m.allCursor >= m.allScroll+viewportHeight {
		m.allScroll = m.allCursor - viewportHeight + 1
	}

	// Render visible nodes
	startIdx := m.allScroll
	endIdx := startIdx + viewportHeight
	if endIdx > len(m.allNodes) {
		endIdx = len(m.allNodes)
	}

	// Show "↑ N more" indicator if tasks are hidden above
	if startIdx > 0 {
		b.WriteString(ScrollUpIndicator(startIdx))
		b.WriteString("\n")
		endIdx = startIdx + viewportHeight - 1 // Account for indicator
		if endIdx > len(m.allNodes) {
			endIdx = len(m.allNodes)
		}
	}

	for i := startIdx; i < endIdx; i++ {
		node := m.allNodes[i]
		isSelected := i == m.allCursor

		line := m.renderAllTaskRow(node, isSelected)
		b.WriteString(line)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// Show "↓ N more" indicator if tasks are hidden below
	if endIdx < len(m.allNodes) {
		b.WriteString("\n")
		b.WriteString(ScrollDownIndicator(len(m.allNodes) - endIdx))
	}

	// Footer
	b.WriteString("\n")
	leftFooter := HelpStyle.Render("j/k: navigate | Space: expand/collapse | q: close")
	b.WriteString(renderStatusBarFooter(leftFooter, "", m.Width()))

	return b.String()
}

// renderAllTaskRow renders a single row in the all view with colored section badge.
func (m *KanbanViewModel) renderAllTaskRow(node *KanbanNode, isSelected bool) string {
	task := node.Task
	statusIcon := task.Status.Icon()
	title := task.Title

	// Build indent prefix
	var indent string
	if node.Level > 0 {
		indent = strings.Repeat("  ", node.Level) + "└"
	}

	// Build expand/collapse icon
	var expandIcon string
	if node.HasChildrenHere {
		if node.Expanded {
			expandIcon = "▼"
		} else {
			expandIcon = "▶"
		}
	} else {
		expandIcon = " "
	}

	// Build section badge with GTD section color
	sectionName := task.Section
	if sectionName == "" {
		sectionName = "Active"
	}
	sectionStyle := GetViewModeStyleForSection(task.Section)
	sectionBadge := sectionStyle.Render("@ " + sectionName)

	// Truncate title to fit (account for section badge)
	sectionBadgeWidth := len(sectionName) + 3                       // "@ " + name
	maxTitleLen := m.Width() - len(indent) - sectionBadgeWidth - 12 // space for cursor, expand, status, padding
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title = truncateWithEllipsis(title, maxTitleLen)

	statusStyle := GetStatusStyle(string(task.Status))

	// Check for incomplete dependencies
	if m.Project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	content := indent + expandIcon + " " + statusStyle.Render(statusIcon) + " " + title + " " + sectionBadge

	// Build cursor prefix with active marker (matches RenderSelectableRow)
	activeMarker := ActiveStyle.Bold(true).Render("*")
	if isSelected && task.Active {
		return SelectionCursorStyle.Render(">") + activeMarker + " " + content
	} else if isSelected {
		return SelectionCursorStyle.Render(">") + "  " + content
	} else if task.Active {
		return " " + activeMarker + " " + content
	}
	return "   " + content
}
