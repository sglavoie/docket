package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// UndoHistoryModel displays the undo history for a project.
type UndoHistoryModel struct {
	BaseView
	InteractiveViewState
	project *models.Project
	entries []storage.UndoEntry

	// Diff preview
	diffLines   []DiffLine
	diffScroll  int
	diffCursor  int // Currently selected diff line when diff pane focused
	diffHScroll int // Horizontal scroll offset for diff pane

	// Detail view
	showingDetail bool
	detailLine    DiffLine // The line being shown in detail

	// Layout
	focusedPane int // 0 = entry list, 1 = diff preview
}

// NewUndoHistoryModel creates a new undo history model.
func NewUndoHistoryModel(project *models.Project) *UndoHistoryModel {
	entries, err := storage.LoadUndoHistory(project)
	if err != nil {
		m := &UndoHistoryModel{
			project: project,
			entries: []storage.UndoEntry{},
		}
		m.SetError(fmt.Errorf("failed to load undo history: %w", err))
		return m
	}

	m := &UndoHistoryModel{
		project: project,
		entries: entries,
	}
	m.BaseView.SetSize(80, 24)

	// Compute initial diff if there are entries
	if len(entries) > 0 {
		m.computeDiff()
	}

	return m
}

// SetSize sets the view dimensions.
func (m *UndoHistoryModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
}

func (m *UndoHistoryModel) Init() tea.Cmd {
	return nil
}

func (m *UndoHistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)

	case tea.KeyPressMsg:
		// Handle detail view first
		if m.showingDetail {
			switch msg.String() {
			case "esc", "q", "enter":
				m.showingDetail = false
			}
			return m, nil
		}

		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			if msg.String() == "g" {
				if m.focusedPane == 0 {
					m.cursor = 0
					m.computeDiff()
					m.diffScroll = 0
					m.diffCursor = 0
					m.diffHScroll = 0
				} else {
					m.diffScroll = 0
					m.diffCursor = 0
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseUndoHistoryMsg{} }

		case "j", "down":
			if m.focusedPane == 0 {
				if m.cursor < len(m.entries)-1 {
					m.cursor++
					m.computeDiff()
					m.diffScroll = 0
					m.diffCursor = 0
					m.diffHScroll = 0
				}
			} else {
				// Move diff cursor down
				if m.diffCursor < len(m.diffLines)-1 {
					m.diffCursor++
					// Auto-scroll to keep cursor visible
					m.ensureDiffCursorVisible()
				}
			}

		case "k", "up":
			if m.focusedPane == 0 {
				if m.cursor > 0 {
					m.cursor--
					m.computeDiff()
					m.diffScroll = 0
					m.diffCursor = 0
					m.diffHScroll = 0
				}
			} else {
				// Move diff cursor up
				if m.diffCursor > 0 {
					m.diffCursor--
					m.ensureDiffCursorVisible()
				}
			}

		case "h", "left":
			if m.focusedPane == 1 {
				// Scroll left
				if m.diffHScroll > 0 {
					m.diffHScroll -= 10
					if m.diffHScroll < 0 {
						m.diffHScroll = 0
					}
				}
			}

		case "l", "right":
			if m.focusedPane == 1 {
				// Scroll right
				m.diffHScroll += 10
			}

		case "tab":
			m.focusedPane = (m.focusedPane + 1) % 2
			// Reset horizontal scroll when switching panes
			if m.focusedPane == 1 {
				m.diffHScroll = 0
			}

		case "enter":
			if m.focusedPane == 1 && len(m.diffLines) > 0 && m.diffCursor < len(m.diffLines) {
				// Show detail view for current diff line
				m.showingDetail = true
				m.detailLine = m.diffLines[m.diffCursor]
			} else if m.focusedPane == 0 && len(m.entries) > 0 {
				// Perform undo of the top entry (multi-step, one at a time)
				return m, m.performUndo()
			}

		case "g":
			m.pendingPrefix = "g"
			return m, nil

		case "G":
			// Go to bottom
			if m.focusedPane == 0 {
				if len(m.entries) > 0 {
					m.cursor = len(m.entries) - 1
					m.computeDiff()
					m.diffScroll = 0
					m.diffCursor = 0
					m.diffHScroll = 0
				}
			} else {
				if len(m.diffLines) > 0 {
					m.diffCursor = len(m.diffLines) - 1
					m.ensureDiffCursorVisible()
				}
			}
		}
	}

	return m, nil
}

// ShouldCaptureEsc returns true when detail view is active or a pending prefix is set.
func (m *UndoHistoryModel) ShouldCaptureEsc() bool {
	return m.showingDetail || m.DefaultShouldCaptureEsc()
}

// ensureDiffCursorVisible adjusts diffScroll to keep the cursor visible.
func (m *UndoHistoryModel) ensureDiffCursorVisible() {
	availableHeight := max(m.Height()-TabbedViewChrome, 1)
	visibleLines := max(availableHeight-5, 1)

	// If cursor is above visible area, scroll up
	if m.diffCursor < m.diffScroll {
		m.diffScroll = m.diffCursor
	}
	// If cursor is below visible area, scroll down
	if m.diffCursor >= m.diffScroll+visibleLines {
		m.diffScroll = m.diffCursor - visibleLines + 1
	}
}

// computeDiff computes the diff for the currently selected entry.
func (m *UndoHistoryModel) computeDiff() {
	if len(m.entries) == 0 || m.cursor >= len(m.entries) {
		m.diffLines = nil
		return
	}

	entry := m.entries[m.cursor]

	// Get current content
	currentContent, err := storage.GetCurrentContent(m.project)
	if err != nil {
		m.diffLines = []DiffLine{{Kind: DiffContext, Content: fmt.Sprintf("Error reading current: %v", err)}}
		return
	}

	// Get snapshot content
	snapshotContent, err := storage.GetSnapshotContent(m.project, entry.Num)
	if err != nil {
		m.diffLines = []DiffLine{{Kind: DiffContext, Content: fmt.Sprintf("Error reading snapshot: %v", err)}}
		return
	}

	// Compute diff (show what changed: snapshot → current, i.e., old → new)
	m.diffLines = ComputeContextualDiff(snapshotContent, currentContent, 3)

	// Filter to show only changes if too many lines
	if len(m.diffLines) > 100 {
		m.diffLines = FilterDiffToChanges(m.diffLines)
	}
}

// performUndo performs the undo operation.
func (m *UndoHistoryModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		actionDesc, err := storage.PerformUndo(m.project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("undo failed: %w", err)}
		}

		// Reload project
		if err := storage.LoadProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after undo: %w", err)}
		}

		return UndoPerformedMsg{Action: actionDesc}
	}
}

func (m *UndoHistoryModel) View() tea.View {
	// Show detail view if active
	if m.showingDetail {
		return tea.NewView(m.renderDetailView())
	}

	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle("Undo History", ""))
	b.WriteString("\n\n")

	// Handle error state
	if m.HasError() {
		b.WriteString(ErrorIconStyle.Render(m.Error().Error()))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
		return tea.NewView(b.String())
	}

	// Handle empty state
	if len(m.entries) == 0 {
		b.WriteString(MutedStyle.Render("No undo history available."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
		return tea.NewView(b.String())
	}

	// Calculate layout
	listWidth := max(m.Width()*35/100, 30)
	diffWidth := m.Width() - listWidth - SeparatorWidth

	availableHeight := max(m.Height()-TabbedViewChrome, 1)

	// Render entry list
	listContent := m.renderEntryList(listWidth, availableHeight)

	// Render diff preview
	diffContent := m.renderDiffPreview(diffWidth, availableHeight)

	// Separator
	separator := SeparatorStyle.Render(strings.Repeat("┃\n", availableHeight))

	// Join horizontally
	row := lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", separator, " ", diffContent)
	b.WriteString(row)
	b.WriteString("\n")

	// Help footer - context-sensitive
	var helpParts []string
	helpParts = append(helpParts, "j/k: navigate")
	if m.focusedPane == 1 {
		helpParts = append(helpParts, "h/l: scroll left/right")
		helpParts = append(helpParts, "Enter: view full line")
	} else {
		helpParts = append(helpParts, "Enter: undo most recent")
	}
	helpParts = append(helpParts, "Tab: switch pane")
	helpParts = append(helpParts, "Esc: close")
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(strings.Join(helpParts, " · "), m.Width())))

	return tea.NewView(b.String())
}

// renderDetailView renders a modal showing the full content of the selected diff line.
// For modifications (remove followed by add), shows both lines together.
func (m *UndoHistoryModel) renderDetailView() string {
	var b strings.Builder

	wrapWidth := max(m.Width()-4, 20)

	// Check if this is part of a modification (remove followed by add or add preceded by remove)
	var pairedLine *DiffLine
	isModification := false

	if m.detailLine.Kind == DiffRemove && m.diffCursor+1 < len(m.diffLines) {
		next := m.diffLines[m.diffCursor+1]
		if next.Kind == DiffAdd {
			pairedLine = &next
			isModification = true
		}
	} else if m.detailLine.Kind == DiffAdd && m.diffCursor > 0 {
		prev := m.diffLines[m.diffCursor-1]
		if prev.Kind == DiffRemove {
			pairedLine = &prev
			isModification = true
		}
	}

	if isModification && pairedLine != nil {
		// Show as modification with both lines
		b.WriteString(TitleStyle.Render("Modified Line (Full Content)"))
		b.WriteString("\n\n")

		// Determine which is old and which is new
		var oldContent, newContent string
		if m.detailLine.Kind == DiffRemove {
			oldContent = m.detailLine.Content
			newContent = pairedLine.Content
		} else {
			oldContent = pairedLine.Content
			newContent = m.detailLine.Content
		}

		// Show old version
		b.WriteString(DiffRemoveStyle.Render("[-] OLD:"))
		b.WriteString("\n")
		for _, line := range wrapText(oldContent, wrapWidth) {
			b.WriteString("  ")
			b.WriteString(DiffRemoveStyle.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")

		// Show new version
		b.WriteString(DiffAddStyle.Render("[+] NEW:"))
		b.WriteString("\n")
		for _, line := range wrapText(newContent, wrapWidth) {
			b.WriteString("  ")
			b.WriteString(DiffAddStyle.Render(line))
			b.WriteString("\n")
		}
	} else {
		// Show single line
		title := "Line Detail"
		switch m.detailLine.Kind {
		case DiffAdd:
			title = "Added Line (Full Content)"
		case DiffRemove:
			title = "Removed Line (Full Content)"
		case DiffContext:
			title = "Context Line (Full Content)"
		}
		b.WriteString(TitleStyle.Render(title))
		b.WriteString("\n\n")

		// Line type indicator
		var kindStyle lipgloss.Style
		var kindLabel string
		switch m.detailLine.Kind {
		case DiffAdd:
			kindStyle = DiffAddStyle
			kindLabel = "[+] ADDED"
		case DiffRemove:
			kindStyle = DiffRemoveStyle
			kindLabel = "[-] REMOVED"
		case DiffContext:
			kindStyle = DiffContextStyle
			kindLabel = "[ ] CONTEXT"
		}
		b.WriteString(kindStyle.Render(kindLabel))
		b.WriteString("\n\n")

		// Content with word wrap
		for _, line := range wrapText(m.detailLine.Content, wrapWidth) {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press Esc/Enter/q to close"))

	return b.String()
}

// wrapText wraps text to the specified width.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	for len(text) > 0 {
		if len(text) <= width {
			lines = append(lines, text)
			break
		}

		// Find a good break point (space)
		breakAt := width
		for i := width; i > 0; i-- {
			if text[i] == ' ' {
				breakAt = i
				break
			}
		}

		// If no space found, just break at width
		if breakAt == width && len(text) > width {
			// Check if we can break earlier at a space
			for i := width - 1; i > width/2; i-- {
				if text[i] == ' ' {
					breakAt = i
					break
				}
			}
		}

		lines = append(lines, text[:breakAt])
		text = strings.TrimLeft(text[breakAt:], " ")
	}

	return lines
}

func (m *UndoHistoryModel) renderEntryList(width, height int) string {
	var b strings.Builder

	// Header
	headerStyle := BoldSecondaryStyle
	if m.focusedPane == 0 {
		headerStyle = BoldPrimaryUnderlineStyle
	}
	b.WriteString(headerStyle.Render("Actions"))
	b.WriteString("\n")

	// Calculate visible range
	// Each entry takes 2 lines (action text + timestamp), header takes 1 line
	visibleEntries := max((height-2)/2, 1)

	startIdx := 0
	if m.cursor >= visibleEntries {
		startIdx = m.cursor - visibleEntries + 1
	}

	endIdx := min(startIdx+visibleEntries, len(m.entries))

	// Render entries
	for i := startIdx; i < endIdx; i++ {
		entry := m.entries[i]

		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Format entry
		actionText := entry.Action
		if entry.Context != "" {
			actionText = fmt.Sprintf("%s: %s", entry.Action, entry.Context)
		}

		// Truncate if too long
		maxLen := width - 4
		if len(actionText) > maxLen {
			actionText = actionText[:maxLen-3] + "..."
		}

		// Time ago
		timeAgo := storage.FormatRelativeTime(entry.Timestamp)

		line := fmt.Sprintf("%s%s", cursor, actionText)
		if i == m.cursor {
			line = BoldStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
		fmt.Fprintf(&b, "   %s", MutedStyle.Render(timeAgo))
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.entries) > visibleEntries {
		remaining := len(m.entries) - endIdx
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *UndoHistoryModel) renderDiffPreview(width, height int) string {
	var b strings.Builder

	// Header
	headerStyle := BoldSecondaryStyle
	if m.focusedPane == 1 {
		headerStyle = BoldPrimaryUnderlineStyle
	}
	b.WriteString(headerStyle.Render("What Changed"))
	b.WriteString("\n")

	if len(m.diffLines) == 0 {
		b.WriteString(MutedStyle.Render("No changes to show"))
		return b.String()
	}

	// Summary with horizontal scroll indicator
	added, removed := SummarizeDiff(m.diffLines)
	summaryText := fmt.Sprintf("+%d / -%d lines", added, removed)
	if m.diffHScroll > 0 {
		summaryText += fmt.Sprintf(" (h-scroll: %d)", m.diffHScroll)
	}
	b.WriteString(MutedStyle.Render(summaryText))
	b.WriteString("\n\n")

	// Calculate visible range
	visibleLines := max(height-5, 1)

	startIdx := m.diffScroll
	if startIdx >= len(m.diffLines) {
		startIdx = max(len(m.diffLines)-1, 0)
	}

	endIdx := min(startIdx+visibleLines, len(m.diffLines))

	// Available width for content (minus prefix)
	contentWidth := width - 3

	// Render diff lines
	for i := startIdx; i < endIdx; i++ {
		line := m.diffLines[i]
		content := line.Content

		// Apply horizontal scroll offset
		if m.diffHScroll > 0 && len(content) > m.diffHScroll {
			content = content[m.diffHScroll:]
		} else if m.diffHScroll > 0 && len(content) <= m.diffHScroll {
			content = ""
		}

		// Check if line extends beyond visible area (for indicator)
		hasMore := len(line.Content) > m.diffHScroll+contentWidth

		// Truncate to fit width
		if len(content) > contentWidth {
			content = content[:contentWidth]
		}

		// Add continuation indicator if content extends beyond view
		if hasMore && len(content) >= 3 {
			content = content[:len(content)-1] + "→"
		}

		// Determine if this line is selected (when diff pane is focused)
		isSelected := m.focusedPane == 1 && i == m.diffCursor

		var lineStyle lipgloss.Style
		prefix := "  "
		switch line.Kind {
		case DiffRemove:
			lineStyle = DiffRemoveStyle
			prefix = "- "
		case DiffAdd:
			lineStyle = DiffAddStyle
			prefix = "+ "
		case DiffContext:
			lineStyle = DiffContextStyle
		}

		// Apply selection highlighting
		if isSelected {
			lineStyle = lineStyle.Background(ColorSelectedBg)
		}

		b.WriteString(lineStyle.Render(prefix + content))
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.diffLines) > visibleLines {
		remaining := len(m.diffLines) - endIdx
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("... %d more lines", remaining)))
		}
	}

	return b.String()
}

func (m *UndoHistoryModel) StatusBarLabel() string { return "Undo History" }
