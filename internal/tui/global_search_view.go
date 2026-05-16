package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// View renders the global search view.
func (m *GlobalSearchModel) View() tea.View {
	var out strings.Builder

	// Title
	subtitle := fmt.Sprintf("%d tasks", len(m.allTasks))
	if len(m.filtered) != len(m.allTasks) {
		subtitle = fmt.Sprintf("%d/%d matching", len(m.filtered), len(m.allTasks))
	}
	out.WriteString(renderFullscreenTitle("Global Search", subtitle, m.AccentColor()))
	out.WriteString("\n")

	// Tab bar
	out.WriteString(m.renderTabBar())
	out.WriteString("\n\n")

	// Search input
	out.WriteString(m.searchInput.View())
	out.WriteString("\n")

	// Show autocomplete suggestions if active
	if m.ac.IsActive() {
		out.WriteString(m.ac.RenderDropdown(""))
	} else {
		out.WriteString(MutedStyle.Render("!priority  #tag  @due  $status  ^type  /section  *active  has:  is:  no:  p:  ?: help"))
	}
	out.WriteString("\n")

	// Show active filters as badges
	if m.hideCompleted || (m.parsedFilters != nil && m.parsedFilters.HasFilters()) {
		out.WriteString(m.renderFilterBadges())
		out.WriteString("\n\n")
	}

	// Handle edge cases
	if len(m.allTasks) == 0 {
		out.WriteString(MutedStyle.Render(EmptyStateCrossProject))
		out.WriteString("\n\n")
		out.WriteString(m.renderHelpText())
		return tea.NewView(out.String())
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" && !m.hideCompleted && m.tabs.Active != globalSearchTabNotes {
		out.WriteString(MutedStyle.Render(EmptyStateSearchPrompt))
		out.WriteString("\n\n")
		out.WriteString(m.renderHelpText())
		return tea.NewView(out.String())
	}

	if len(m.filtered) == 0 {
		itemType := "tasks"
		if m.tabs.Active == globalSearchTabNotes {
			itemType = "notes"
		}
		if query != "" {
			out.WriteString(MutedStyle.Render(fmt.Sprintf("No %s match \"%s\".", itemType, query)))
		} else {
			out.WriteString(MutedStyle.Render(fmt.Sprintf("No %s found.", itemType)))
		}
		out.WriteString("\n\n")
		out.WriteString(m.renderHelpText())
		return tea.NewView(out.String())
	}

	// Calculate pane height for split view
	paneHeight := max(
		// title + tab bar + input + hint + footer
		m.Height()-9, 5)

	// Adjust for autocomplete and filter badges
	if m.ac.IsActive() { //nolint:nestif
		paneHeight++ // hint line is hidden when autocomplete is active
		visible := min(len(m.ac.Items), acVisibleCount)
		// Account for items + scroll indicators
		if m.ac.ViewportTop > 0 {
			visible++
		}
		if m.ac.ViewportTop+acVisibleCount < len(m.ac.Items) {
			visible++
		}
		paneHeight -= visible
	}
	if m.hideCompleted || (m.parsedFilters != nil && m.parsedFilters.HasFilters()) {
		paneHeight -= 2
	}

	// Split pane: list on left, preview/help on right
	listContent := m.renderTaskList(paneHeight)
	var previewContent string
	if m.showHelp {
		previewContent = m.renderSearchHelp()
	} else {
		previewContent = m.renderPreview()
	}

	splitPane := renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight)
	out.WriteString(splitPane)
	out.WriteString("\n")

	// Footer
	out.WriteString(m.renderHelpText())

	return tea.NewView(out.String())
}

// renderTabBar renders the tab bar for filtering.
func (m *GlobalSearchModel) renderTabBar() string {
	return m.tabs.RenderBar(m.Width(), m.AccentColor())
}

// renderHelpText renders context-appropriate help text.
func (m *GlobalSearchModel) renderHelpText() string {
	if m.ac.IsActive() {
		return HelpStyle.Render("Tab: accept · ↑↓: navigate · Esc: dismiss")
	}
	return HelpStyle.Render(fmt.Sprintf("shift+←/→ tabs · ↑↓ nav · Enter · Ctrl+O completed · ?: help · Esc · %d results", len(m.filtered)))
}
