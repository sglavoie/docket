package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// renderDefaultHint renders "(default: X)" in muted style when current differs from def.
// Returns empty string when def is empty or current matches def.
func renderDefaultHint(current, def string) string {
	if def == "" || current == def {
		return ""
	}
	return "  " + MutedStyle.Render("(default: "+def+")")
}

// renderTreeColumnToggles renders the Project Tree column visibility toggles.
// Iterates AllColumns filtered to exclude OutlinerOnly entries — the toggles
// array indices must align with that filtered slice.
// The Priority column is rendered as a 3-way enum selector instead of a toggle.
func (m *SettingsModel) renderTreeColumnToggles(b *strings.Builder, toggles [15]bool, startIdx, labelWidth int, priorityCursor int) {
	idx := 0
	for _, col := range AllColumns {
		if col.OutlinerOnly {
			continue
		}
		focusIdx := startIdx + idx
		b.WriteString(renderFormLabelWithWidth(col.Label+": ", labelWidth, m.focusIndex == focusIdx))
		if col.ID == ColPriority {
			b.WriteString(renderEnumSelector(priorityModeOptions, priorityCursor, m.focusIndex == focusIdx, m.width-(labelWidth+2)))
		} else {
			b.WriteString(renderFormToggle("Visible", "Hidden", toggles[idx], m.focusIndex == focusIdx))
		}
		b.WriteString("\n")
		idx++
	}
}

// renderOutlinerColumnToggles renders the Outliner column visibility toggles
// across the full AllColumns list (no filtering).
// The Priority column is rendered as a 3-way enum selector instead of a toggle.
func (m *SettingsModel) renderOutlinerColumnToggles(b *strings.Builder, toggles [15]bool, startIdx, labelWidth int, priorityCursor int) {
	for i, col := range AllColumns {
		focusIdx := startIdx + i
		b.WriteString(renderFormLabelWithWidth(col.Label+": ", labelWidth, m.focusIndex == focusIdx))
		if col.ID == ColPriority {
			b.WriteString(renderEnumSelector(priorityModeOptions, priorityCursor, m.focusIndex == focusIdx, m.width-(labelWidth+2)))
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
	// Subtract rows the visual sub-tab bar consumes (may wrap to >1 row when
	// labels overflow available width).
	if names := visualSubTabNamesForVTab(m.visualTab); names != nil {
		rows := max(wrappingTabBarRows(names, m.width-2), 1)
		h -= rows
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
		if strings.Contains(line, IconCursorRight) {
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

	visualSubNames := visualSubTabNamesForVTab(m.visualTab)
	if visualSubNames != nil {
		out.WriteString(renderTabBarWithOptionalSubTabs(settingsVisualTabNames, m.visualTab, visualSubNames, m.subTabRowFocused, m.width))
		out.WriteString("\n")
		out.WriteString(renderSubTabBarWithFocus(visualSubNames, m.visualSubTab, m.subTabRowFocused, m.width))
		out.WriteString("\n\n")
	} else {
		visualTabState := TabState{Names: settingsVisualTabNames, Active: m.visualTab}
		out.WriteString(visualTabState.RenderBar(m.width))
		out.WriteString("\n\n")
	}

	if m.searchActive {
		out.WriteString(m.renderSearchOverlay(visibleHeight))
	} else {
		out.WriteString(visibleContent)
	}
	out.WriteString("\n")

	scrollHint := ""
	if !m.searchActive && len(contentLines) > visibleHeight {
		scrollHint = fmt.Sprintf(" (%d/%d)", m.scrollOffset+1, maxScroll+1)
	}
	helpPrefix := "/ search · "
	if visualSubTabNamesForVTab(m.visualTab) != nil {
		helpPrefix += "shift+↑/↓ sub-tabs · "
	}
	helpText := helpPrefix + "shift+←/→: tabs · Tab: fields · Space: toggle · Enter: save · Esc: cancel · ctrl+d: reset · ?: Help" + scrollHint
	var left string
	if m.searchStatusMsg != "" {
		left = HelpStyle.Render(m.searchStatusMsg)
	} else if m.searchActive {
		left = HelpStyle.Render("↑/↓ navigate · Enter: jump · Esc: cancel")
	} else {
		if lipgloss.Width(helpText) > m.width-20 {
			line1 := helpPrefix + "shift+←/→: tabs · Tab: fields · Space: toggle · Enter: save"
			line2 := "ctrl+d: reset · Esc: cancel · ?: Help" + scrollHint
			left = HelpStyle.Render(line1) + "\n" + HelpStyle.Render(line2)
		} else {
			left = HelpStyle.Render(helpText)
		}
	}
	right := MutedStyle.Render("Settings")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(out.String())
}

// renderSearchOverlay renders the search input and filtered results list.
func (m *SettingsModel) renderSearchOverlay(visibleHeight int) string {
	var b strings.Builder

	b.WriteString("  ")
	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")

	if len(m.searchResults) == 0 {
		b.WriteString(MutedStyle.Render("  No matches"))
		b.WriteString("\n")
		return b.String()
	}

	maxVisible := max(visibleHeight-3, 1)
	start := 0
	if m.searchCursor >= maxVisible {
		start = m.searchCursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.searchResults))

	for i := start; i < end; i++ {
		result := m.searchResults[i]
		prefix := "  "
		label := result.TabLabel + " › " + result.Label
		if i == m.searchCursor {
			b.WriteString(BoldPrimaryStyle.Render(prefix+IconCursorRight+" ") + BoldStyle.Render(label))
		} else {
			b.WriteString(MutedStyle.Render(prefix+"  ") + label)
		}
		b.WriteString("\n")
	}

	if len(m.searchResults) > maxVisible {
		shown := fmt.Sprintf("  (%d/%d)", m.searchCursor+1, len(m.searchResults))
		b.WriteString(MutedStyle.Render(shown))
		b.WriteString("\n")
	}

	return b.String()
}
