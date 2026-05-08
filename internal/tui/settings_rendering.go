package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// renderColumnToggles renders 13 column visibility toggles.
// The Priority column (index 1) is rendered as a 3-way enum selector instead of a toggle.
func (m *SettingsModel) renderColumnToggles(b *strings.Builder, toggles [14]bool, startIdx, labelWidth int, priorityCursor int) {
	for i, col := range AllColumns {
		focusIdx := startIdx + i
		b.WriteString(renderFormLabelWithWidth(col.Label+": ", labelWidth, m.focusIndex == focusIdx))
		if col.ID == ColPriority {
			b.WriteString(renderEnumSelector(priorityModeOptions, priorityCursor, m.focusIndex == focusIdx))
		} else {
			b.WriteString(renderFormToggle("Visible", "Hidden", toggles[i], m.focusIndex == focusIdx))
		}
		b.WriteString("\n")
	}
}

// settingsContentHeight returns the number of lines available for tab content,
// accounting for the title, tab bar, blank line, and footer.
func (m *SettingsModel) settingsContentHeight() int {
	// Title (1) + tab bar with border (2) + blank line (1) + footer (1) + padding (1)
	h := m.height - TabbedViewChrome
	// Subtract 1 extra line when sub-tab row is visible
	if subTabNamesForTab(m.tabs.Active) != nil {
		h--
	}
	if h < 3 {
		return 3
	}
	return h
}

// focusLineInContent returns the approximate line number within the rendered
// tab content where the currently focused field appears. This drives auto-scroll.
func (m *SettingsModel) focusLineInContent() int {
	tab := tabForField(m.focusIndex)
	localIdx := m.focusIndex - subTabFieldStart(tab, m.subTab)

	switch tab {
	case tabGeneral:
		return localIdx * 5
	case tabPalette:
		return localIdx * 3
	case tabReview:
		return 3 + localIdx*4
	case tabBehavior:
		return localIdx * 4
	case tabAttachments:
		return 3 + localIdx*5
	case tabProjectTree:
		return 3 + localIdx
	case tabOutliner:
		switch m.subTab {
		case 0:
			return localIdx * 3
		case 1:
			if localIdx == 0 {
				return 0
			}
			return 1 + localIdx
		case 2:
			return localIdx
		}
	case tabKanban:
		return localIdx * 3
	case tabNotifications:
		switch m.subTab {
		case 0:
			return localIdx * 4
		case 1:
			switch localIdx {
			case 0:
				return 0
			case 1:
				return 4
			case 2:
				return 6
			case 3:
				return 10
			case 4:
				return 12
			case 5:
				return 16
			case 6:
				return 18
			case 7:
				return 22
			case 8:
				return 24
			}
			return localIdx * 3
		case 2:
			return localIdx * 2
		}
	case tabStages:
		return 3
	case tabAppearance:
		return 0
	}
	return 0
}

func (m *SettingsModel) View() tea.View {
	var content string
	switch m.tabs.Active {
	case tabGeneral:
		content = m.renderGeneralTab()
	case tabPalette:
		content = m.renderPaletteTab()
	case tabReview:
		content = m.renderReviewTab()
	case tabBehavior:
		content = m.renderBehaviorTab()
	case tabAttachments:
		content = m.renderAttachmentsTab()
	case tabProjectTree:
		content = m.renderProjectTreeTab()
	case tabOutliner:
		content = m.renderOutlinerTab()
	case tabKanban:
		content = m.renderKanbanTab()
	case tabNotifications:
		content = m.renderNotificationsTab()
	case tabStages:
		content = m.renderStagesTab()
	case tabAppearance:
		content = m.renderAppearanceTab()
	}

	contentLines := strings.Split(content, "\n")
	visibleHeight := m.settingsContentHeight()

	focusLine := -1
	for i, line := range contentLines {
		if strings.Contains(line, "▸") {
			focusLine = i
			break
		}
	}
	if focusLine < 0 {
		focusLine = m.focusLineInContent()
	}
	if focusLine < m.scrollOffset {
		m.scrollOffset = focusLine
	}
	if focusLine >= m.scrollOffset+visibleHeight {
		m.scrollOffset = focusLine - visibleHeight + 1
	}

	maxScroll := max(len(contentLines)-visibleHeight, 0)
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}

	end := min(m.scrollOffset+visibleHeight, len(contentLines))
	visibleContent := strings.Join(contentLines[m.scrollOffset:end], "\n")

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Settings", ""))
	out.WriteString("\n")

	subNames := subTabNamesForTab(m.tabs.Active)
	if subNames != nil {
		out.WriteString(renderTabBarWithOptionalSubTabs(settingsTabNames, m.tabs.Active, subNames, m.subTabRowFocused, m.width))
		out.WriteString("\n")
		out.WriteString(renderSubTabBarWithFocus(subNames, m.subTab, m.subTabRowFocused, m.width))
		out.WriteString("\n\n")
	} else {
		out.WriteString(m.tabs.RenderBar(m.width))
		out.WriteString("\n\n")
	}

	out.WriteString(visibleContent)
	out.WriteString("\n")

	scrollHint := ""
	if len(contentLines) > visibleHeight {
		scrollHint = fmt.Sprintf(" (%d/%d)", m.scrollOffset+1, maxScroll+1)
	}
	helpPrefix := ""
	if subTabNamesForTab(m.tabs.Active) != nil {
		helpPrefix = "shift+↑/↓ sub-tabs · "
	}
	left := HelpStyle.Render(helpPrefix + "shift+←/→: tabs · Tab: fields · Space: toggle · Enter: save · Esc: cancel · ?: Help" + scrollHint)
	right := MutedStyle.Render("Settings")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(out.String())
}
