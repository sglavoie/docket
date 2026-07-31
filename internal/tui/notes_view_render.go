package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
)

func (m *NotesViewModel) View() tea.View {
	if m.locationPicker != nil {
		return m.locationPicker.View()
	}

	width := max(m.Width(), 1)
	height := max(m.Height(), 1)
	subtitle := m.scopeLabel()
	titleWidth := lipgloss.Width(fullscreenTitleStyle(m.AccentColor()).Render("Notes"))
	if width < 40 {
		subtitle = ""
	} else {
		subtitle = truncateToVisualWidth(subtitle, max(width-titleWidth-2, 1))
	}
	header := []string{
		renderFullscreenTitle("Notes", subtitle, m.AccentColor()),
		MutedStyle.Render(truncateToVisualWidth(m.scopeCountLabel(), width)),
	}
	if search := m.renderSearchLine(); search != "" {
		header = append(header, fitStyledLineToWidth(search, width))
	}
	errorLines := m.renderCollectionErrors()
	if errorLines != "" {
		header = append(header, errorLines)
	}

	var footer []string
	if m.renaming {
		footer = append(footer, fitStyledLineToWidth("Rename: "+m.renameInput.View(), width))
	} else if m.searchEditing {
		help := "Enter: apply · Esc: cancel · ↑/↓ or Ctrl+J/K: navigate"
		footer = append(footer, HelpStyle.Render(renderWrappingHelpLine(help, width)))
	} else {
		escHelp := "Esc: close"
		if m.searchApplied != "" {
			escHelp = "Esc: clear search"
		}
		help := "Enter: open/edit · n: new · v: scope · s: search · r: rename · m: move · x: delete · u: undo · Ctrl+R: redo · " + escHelp
		footer = append(footer, HelpStyle.Render(renderWrappingHelpLine(help, width)))
	}
	if m.statusMessage != "" {
		footer = append(footer, fitStyledLineToWidth(m.renderStatusBar(), width))
	}

	headerBlock := strings.Join(header, "\n")
	footerBlock := strings.Join(footer, "\n")
	contentHeight := fullscreenContentHeightForBlocks(height, headerBlock, footerBlock)
	content := fitBlockHeight(m.renderListPaneHeight(contentHeight), contentHeight)
	return tea.NewView(lipgloss.JoinVertical(
		lipgloss.Left,
		headerBlock,
		content,
		footerBlock,
	))
}

func (m *NotesViewModel) renderListPane() string {
	errorRows := len(m.collectionErrors)
	if m.snapshotError != nil {
		errorRows = 1
	}
	searchRows := 0
	if m.searchEditing || m.searchApplied != "" {
		searchRows = 1
	}
	return m.renderListPaneHeight(max(m.Height()-7-errorRows-searchRows, 1))
}

func (m *NotesViewModel) renderListPaneHeight(listHeight int) string {
	if m.snapshotError != nil {
		return renderEmptyState("Notes are unavailable.", "Resolve the workspace error and reopen Notes.") + "\n"
	}
	if len(m.visibleRows) == 0 {
		if strings.TrimSpace(m.activeSearchQuery()) != "" {
			hint := "Press s to edit the search or Esc to clear it."
			if m.searchEditing {
				hint = "Keep typing or press Esc to restore the accepted search."
			}
			return renderEmptyState(
				"No matching notes.",
				hint,
			) + "\n"
		}
		message := "No notes yet."
		if len(m.collectionErrors) > 0 {
			message = "No notes loaded from the available owners."
		}
		return renderEmptyState(message, "Press n to choose a location and create one.") + "\n"
	}

	var b strings.Builder
	listHeight = max(listHeight, 1)
	start, end := persistentIndexWindow(m.cursor, len(m.visibleRows), listHeight, &m.scrollOffset)
	width := max(m.Width(), 1)
	scopeWidth := min(24, max(width/3, 6))
	for i := start; i < end; i++ {
		row := m.visibleRows[i]
		owner := truncateAndPad(row.OwnerLabel, scopeWidth)
		match := m.searchMatches[row.Identity()]
		bodyIndicator := ""
		if match.bodyMatched {
			bodyIndicator = "  " + MutedStyle.Render("· body")
		}
		bodyColumns := 0
		if match.bodyMatched {
			bodyColumns = 8
		}
		parseColumns := 0
		if row.Note.ParseError != "" {
			parseColumns = 2
		}
		titleWidth := max(width-scopeWidth-2-bodyColumns-parseColumns, 1)
		title := truncateToVisualWidth(row.Note.Title, titleWidth)
		renderedTitle := title
		if len(match.titlePositions) > 0 {
			renderedTitle = HighlightText(title, match.titlePositions, NormalStyle)
		}
		line := fmt.Sprintf("%s  %s%s", MutedStyle.Render(owner), renderedTitle, bodyIndicator)
		if row.Note.ParseError != "" {
			line = WarningIconStyle.Render("! ") + line
		}
		line = fitStyledLineToWidth(line, width)
		if i == m.cursor {
			b.WriteString(PickerSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m *NotesViewModel) renderCollectionErrors() string {
	width := max(m.Width(), 1)
	if m.snapshotError != nil {
		line := truncateToVisualWidth("! Workspace: "+m.snapshotError.Error(), width)
		return WarningIconStyle.Render(line)
	}
	const maxErrorLines = 2
	var lines []string
	visibleFailures := min(len(m.collectionErrors), maxErrorLines)
	if len(m.collectionErrors) > maxErrorLines {
		visibleFailures = maxErrorLines - 1
	}
	for _, failure := range m.collectionErrors[:visibleFailures] {
		line := truncateToVisualWidth(
			fmt.Sprintf("! %s: %v", failure.OwnerLabel, failure.Err),
			width,
		)
		lines = append(lines, WarningIconStyle.Render(
			line,
		))
	}
	if remaining := len(m.collectionErrors) - visibleFailures; remaining > 0 {
		summary := fmt.Sprintf(
			"! %d more %s unavailable",
			remaining,
			pluralize(remaining, "owner", "owners"),
		)
		lines = append(lines, WarningIconStyle.Render(truncateToVisualWidth(summary, width)))
	}
	return strings.Join(lines, "\n")
}

func (m *NotesViewModel) scopeLabel() string {
	switch m.scope.Kind {
	case NotesScopeGlobal:
		return "Global"
	case NotesScopeProject:
		return m.locationLabel(app.ProjectNoteLocation(m.scope.ProjectID))
	default:
		return "All Notes"
	}
}

func (m *NotesViewModel) scopeCountLabel() string {
	count := len(m.visibleRows)
	if query := strings.TrimSpace(m.activeSearchQuery()); query != "" {
		return fmt.Sprintf(
			"%d of %d notes match %q in %s",
			count,
			m.scopedRowCount(),
			query,
			m.scopeLabel(),
		)
	}
	switch m.scope.Kind {
	case NotesScopeGlobal:
		return fmt.Sprintf("%d Global notes", count)
	case NotesScopeProject:
		return fmt.Sprintf("%d notes in %s", count, m.scopeLabel())
	default:
		ownerCount := len(m.requestedLocations()) - len(m.collectionErrors)
		if ownerCount < 0 {
			ownerCount = 0
		}
		return fmt.Sprintf("%d notes across %d owners", count, ownerCount)
	}
}

func (m *NotesViewModel) renderSearchLine() string {
	if m.searchEditing {
		return "Search: " + m.searchInput.View()
	}
	if m.searchApplied != "" {
		return MutedStyle.Render(fmt.Sprintf("Search: %q", m.searchApplied))
	}
	return ""
}

func (m *NotesViewModel) locationLabel(location app.NoteLocation) string {
	return m.ownerForLocation(location).Label
}

func (m *NotesViewModel) renderStatusBar() string {
	if m.renaming {
		return "Rename: " + m.renameInput.View()
	}
	if m.statusMessage == "" {
		return ""
	}
	if m.statusIsError {
		return WarningIconStyle.Render(m.statusMessage)
	}
	return SuccessStyle.Render(m.statusMessage)
}

// GetHelpItems returns keybinding help for the Notes manager.
func (m *NotesViewModel) GetHelpItems() []HelpItem {
	if m.locationPicker != nil {
		return m.locationPicker.GetHelpItems()
	}
	return []HelpItem{
		{Key: "j/k", Description: "Navigate"},
		{Key: "Enter", Description: "Open in external editor"},
		{Key: "n", Description: "Create in Global or an active project"},
		{Key: "v", Description: "Choose All, Global, or active-project scope"},
		{Key: "s", Description: "Search note titles and bodies"},
		{Key: "r", Description: "Rename note"},
		{Key: "m", Description: "Move to Global or an active project"},
		{Key: "x", Description: "Delete note"},
		{Key: "u", Description: "Undo workspace change"},
		{Key: "Ctrl+R", Description: "Redo workspace change"},
		{Key: "Esc", Description: "Cancel search, clear it, or close Notes"},
	}
}
