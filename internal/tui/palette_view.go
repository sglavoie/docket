package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Note: HighlightStyle and HighlightText are defined in highlight.go

// View implements tea.Model.
// StatusBarLabel implements StatusBarLabeler.
func (m *CommandPaletteModel) StatusBarLabel() string { return "Command Palette" }

func (m *CommandPaletteModel) View() tea.View {
	var content strings.Builder

	// In action mode, show which item we're acting on
	if m.actionMode {
		itemDesc := m.actionItem.Name
		if m.actionItem.Description != "" {
			itemDesc = fmt.Sprintf("%s · %s", m.actionItem.Name, m.actionItem.Description)
		}
		content.WriteString(MutedStyle.Render(itemDesc))
		content.WriteString("\n\n")
	} else {
		// Search input (only in search mode)
		content.WriteString(m.searchInput.View())
		// Show history indicator when browsing history
		if m.searchHistoryIndex >= 0 && len(m.searchHistory) > 0 {
			historyHint := fmt.Sprintf(" %s",
				MutedStyle.Render(fmt.Sprintf("(history %d/%d)", m.searchHistoryIndex+1, len(m.searchHistory))))
			content.WriteString(historyHint)
		}
		content.WriteString("\n\n")
	}

	// Handle edge cases
	if len(m.allItems) == 0 {
		var out strings.Builder
		out.WriteString(renderFullscreenTitle("Command Palette", "0 items"))
		out.WriteString("\n")
		out.WriteString(content.String())
		out.WriteString(MutedStyle.Render("No items available."))
		out.WriteString("\n\n")
		out.WriteString(HelpStyle.Render("Press Esc to close"))
		return tea.NewView(out.String())
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if !m.actionMode && len(m.filteredItems) == 0 && query != "" {
		var out strings.Builder
		out.WriteString(renderFullscreenTitle("Command Palette", fmt.Sprintf("0/%d matching", len(m.allItems))))
		out.WriteString("\n")
		out.WriteString(content.String())
		noMatch := truncateToVisualWidth(fmt.Sprintf("No matches for \"%s\"", query), m.width)
		out.WriteString(MutedStyle.Render(noMatch))
		out.WriteString("\n\n")
		out.WriteString(HelpStyle.Render("Try: :cmd for commands · :task for tasks · :prj for projects"))
		out.WriteString("\n")
		out.WriteString(HelpStyle.Render("     :tag :sec :note :view · or clear to browse all"))
		return tea.NewView(out.String())
	}

	// Render content based on mode
	var listContent string
	if m.actionMode {
		listContent = m.renderActionList()
	} else {
		listContent = m.renderItemList()
	}

	paneHeight := max(
		// title + input + spacing + footer
		m.height-8, 5)

	// Build title
	title := "Command Palette"
	if m.actionMode {
		title = "Actions"
	}
	subtitle := m.buildSubtitle()

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, subtitle))
	out.WriteString("\n")
	out.WriteString(content.String())

	// Compact mode: full-width list, no preview
	// Use m.listWidth + m.previewWidth for full inner width (calculated in SetSize)
	if m.compactMode {
		innerWidth := m.listWidth + m.previewWidth + 1 // +1 for separator that's not used
		listPane := TaskJumperListStyle.
			Width(innerWidth).
			Height(paneHeight).
			MaxHeight(paneHeight).
			Render(listContent)
		out.WriteString(listPane)
		out.WriteString("\n")
	} else {
		// Split pane: list on left, preview on right
		// Use m.listWidth and m.previewWidth which are calculated in SetSize
		// to account for card border and padding
		previewContent := m.renderPreview()
		splitPane := renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight)
		out.WriteString(splitPane)
		out.WriteString("\n")
	}

	// Footer with controls - different for each mode
	var footerText string
	if m.actionMode {
		footerText = m.buildActionModeFooter()
	} else {
		footerText = m.buildFooter()
	}
	out.WriteString(renderStatusBarFooter(HelpStyle.Render(footerText), "", m.width))

	return tea.NewView(out.String())
}
