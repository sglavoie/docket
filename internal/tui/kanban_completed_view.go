package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

// toggleCompletedView toggles the completed view mode.
// When active, the normal 5-column view is replaced with a single full-width
// list of completed tasks (done + canceled).
func (m *KanbanViewModel) toggleCompletedView() {
	m.completedViewMode = !m.completedViewMode
	if m.completedViewMode {
		m.buildCompletedNodes()
		m.completedCursor = 0
		m.completedScroll = 0
		m.statusMessage = "Completed view (gc/F1/q to exit)"
	} else {
		m.completedNodes = nil
		m.statusMessage = ""
	}
}

// buildCompletedNodes builds the list of completed tasks for the completed view.
// Includes tasks with status Done or Canceled, preserving hierarchy.
func (m *KanbanViewModel) buildCompletedNodes() {
	m.completedNodes = nil
	if m.Project == nil {
		return
	}

	// Build a set of completed task IDs for hierarchy lookup
	completedIDs := make(map[string]bool)
	for _, task := range m.Project.Tasks {
		if task.Status.IsComplete() {
			completedIDs[task.ID] = true
		}
	}

	// Find root completed tasks (no parent or parent not in completed set)
	var rootTasks []*KanbanNode
	for _, task := range m.Project.Tasks {
		if !task.Status.IsComplete() {
			continue
		}
		isRoot := task.ParentID == nil || !completedIDs[*task.ParentID]
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
		m.addCompletedNode(root, 0, completedIDs)
	}
}

// addCompletedNode recursively adds a completed task and its children.
func (m *KanbanViewModel) addCompletedNode(node *KanbanNode, level int, completedIDs map[string]bool) {
	// Cap level at MaxKanbanDisplayLevel
	displayLevel := level
	if displayLevel > MaxKanbanDisplayLevel {
		displayLevel = MaxKanbanDisplayLevel
	}

	// Check if this task has completed children
	hasCompletedChildren := false
	for _, child := range node.Task.Children {
		if completedIDs[child.ID] {
			hasCompletedChildren = true
			break
		}
	}

	node.Level = displayLevel
	node.HasChildrenHere = hasCompletedChildren

	// Get or default expand state (default = expanded)
	expanded := true
	if state, ok := m.ExpandState[node.Task.ID]; ok {
		expanded = state
	} else {
		m.ExpandState[node.Task.ID] = true
	}
	node.Expanded = expanded
	node.Visible = true

	m.completedNodes = append(m.completedNodes, node)

	// Add children if expanded
	if expanded {
		for _, child := range node.Task.Children {
			if completedIDs[child.ID] {
				childNode := &KanbanNode{
					Task:    child,
					Level:   displayLevel + 1,
					Visible: true,
				}
				m.addCompletedNode(childNode, level+1, completedIDs)
			}
		}
	}
}

// handleCompletedViewKey handles key input in completed view mode.
func (m *KanbanViewModel) handleCompletedViewKey(key string) (bool, interface{}) {
	switch key {
	case "j", "down":
		// Move cursor down
		if len(m.completedNodes) > 0 && m.completedCursor < len(m.completedNodes)-1 {
			m.completedCursor++
		}
		return true, nil

	case "k", "up":
		// Move cursor up
		if m.completedCursor > 0 {
			m.completedCursor--
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
		if len(m.completedNodes) > 0 {
			m.completedCursor = len(m.completedNodes) - 1
		}
		return true, nil

	case "gc", "f1":
		// Exit completed view
		m.toggleCompletedView()
		return true, nil

	case "q", "esc":
		// Exit completed view
		m.toggleCompletedView()
		return true, nil

	case " ":
		// Toggle expand/collapse
		if m.completedCursor < len(m.completedNodes) {
			node := m.completedNodes[m.completedCursor]
			if node.HasChildrenHere {
				m.ExpandState[node.Task.ID] = !node.Expanded
				m.buildCompletedNodes()
				// Try to keep cursor on same task
				for i, n := range m.completedNodes {
					if n.Task.ID == node.Task.ID {
						m.completedCursor = i
						break
					}
				}
			}
		}
		return true, nil
	}

	return false, nil
}

// renderCompletedView renders the full-width completed tasks view.
func (m *KanbanViewModel) renderCompletedView() string {
	var b strings.Builder

	// Header
	b.WriteString(renderFullscreenTitle(
		fmt.Sprintf("Completed Tasks (%d)", len(m.completedNodes)),
		"",
	))
	b.WriteString("\n")
	b.WriteString(renderHorizontalDivider(m.Width()))
	b.WriteString("\n")

	// Calculate viewport height (full height minus header, divider, status)
	viewportHeight := m.Height() - 4 // header + divider + status + help hint

	if len(m.completedNodes) == 0 {
		b.WriteString(MutedStyle.Render("No completed tasks"))
		return b.String()
	}

	// Adjust scroll to keep cursor visible
	if m.completedCursor < m.completedScroll {
		m.completedScroll = m.completedCursor
	}
	if m.completedCursor >= m.completedScroll+viewportHeight {
		m.completedScroll = m.completedCursor - viewportHeight + 1
	}

	// Render visible nodes
	startIdx := m.completedScroll
	endIdx := startIdx + viewportHeight
	if endIdx > len(m.completedNodes) {
		endIdx = len(m.completedNodes)
	}

	var rowBuilder strings.Builder
	for i := startIdx; i < endIdx; i++ {
		node := m.completedNodes[i]
		isSelected := i == m.completedCursor

		line := m.renderCompletedTaskRow(node, isSelected)
		rowBuilder.WriteString(line)
		if i < endIdx-1 {
			rowBuilder.WriteString("\n")
		}
	}

	scrollbar := RenderScrollbar(len(m.completedNodes), viewportHeight, startIdx)
	b.WriteString(AttachScrollbar(rowBuilder.String(), scrollbar))

	// Footer
	b.WriteString("\n")
	leftFooter := HelpStyle.Render("j/k: navigate · Space: expand/collapse · q: close")
	b.WriteString(renderStatusBarFooter(leftFooter, "", m.Width()))

	return b.String()
}

// renderCompletedTaskRow renders a single row in the completed view.
func (m *KanbanViewModel) renderCompletedTaskRow(node *KanbanNode, isSelected bool) string {
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

	// Truncate title to fit
	maxTitleLen := m.Width() - len(indent) - 8 // space for cursor, expand, status, padding
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title = util.Truncate(title, maxTitleLen)

	statusStyle := GetStatusStyle(string(task.Status))
	content := indent + expandIcon + " " + statusStyle.Render(statusIcon) + " " + title

	// Add completion date if available
	if task.CompletedAt != nil {
		dateStr := task.CompletedAt.Format("2006-01-02")
		content += " " + MutedStyle.Render(dateStr)
	}

	if isSelected {
		return SelectionCursorStyle.Render(">") + " " + content
	}
	return "  " + content
}
