package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
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
		subtitle = textfit.TruncateToVisualWidth(subtitle, max(width-titleWidth-2, 1))
	}
	header := []string{
		renderFullscreenTitleWithPulse("Notes", subtitle, m.currentWorkspacePulse(), width, m.AccentColor()),
		theme.MutedStyle.Render(textfit.TruncateToVisualWidth(m.scopeCountLabel(), width)),
	}
	if search := m.renderSearchLine(); search != "" {
		header = append(header, textfit.FitStyledLineToWidth(search, width))
	}
	errorLines := m.renderCollectionErrors()
	if errorLines != "" {
		header = append(header, errorLines)
	}

	var footer []string
	if m.renaming {
		footer = append(footer, textfit.FitStyledLineToWidth("Rename: "+m.renameInput.View(), width))
	}
	if guidance, ok := m.ContextualGuidance(); ok {
		footer = append(footer, renderGuidance(guidance, width, m.AccentColor()))
	}
	if m.statusMessage != "" {
		footer = append(footer, textfit.FitStyledLineToWidth(m.renderStatusBar(), width))
	}

	headerBlock := strings.Join(header, "\n")
	footerBlock := strings.Join(footer, "\n")
	contentHeight := fullscreenContentHeightForBlocks(height, headerBlock, footerBlock)
	content := fitBlockHeight(m.renderNotesContent(width, contentHeight), contentHeight)
	return tea.NewView(lipgloss.JoinVertical(
		lipgloss.Left,
		headerBlock,
		content,
		footerBlock,
	))
}

// renderNotesContent chooses between the shared two-pane layout and the list
// alone. The split comes from listSurfacePreviewGeometry rather than a Notes
// copy of the 72-column rule, so this surface widens and narrows exactly when
// every other list surface does. An empty list, a workspace error, or an open
// picker has no note to preview, so those states keep the full width the way the
// compact list renderer does.
func (m *NotesViewModel) renderNotesContent(width, height int) string {
	listWidth, previewWidth := m.notesPreviewGeometry(width)
	row := m.notesPreviewRow()
	if previewWidth <= 0 || row == nil {
		return m.renderListPane(width, height)
	}
	listPane := strings.TrimRight(m.renderListPane(listWidth, height), "\n")
	previewPane := renderListSurfacePreview(
		notesPreviewHeader(row),
		previewWidth,
		height,
		m.renderNotesPreviewBody,
	)
	return joinListSurfacePanes(listPane, previewPane, listWidth, previewWidth)
}

// renderListPane draws the note rows inside the pane width it is given. The
// width is a parameter rather than m.Width() because the same rows are drawn
// into the narrow list column of the wide layout, where every column budget
// below — the owner scope, the body-match indicator, and the title — must be
// charged against the pane rather than the terminal.
func (m *NotesViewModel) renderListPane(width, listHeight int) string {
	if m.snapshotError != nil {
		return renderEmptyState("Notes are unavailable.", "Resolve the workspace error and reopen Notes.") + "\n"
	}
	if len(m.visibleRows) == 0 {
		if strings.TrimSpace(m.activeSearchQuery()) != "" {
			hint := "Press / to edit the search or Esc to clear it."
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
		return renderEmptyState(message, "Press a to choose a location and create one.") + "\n"
	}

	var b strings.Builder
	listHeight = max(listHeight, 1)
	start, end := persistentIndexWindow(m.cursor, len(m.visibleRows), listHeight, &m.scrollOffset)
	width = max(width, 1)
	scopeWidth := min(24, max(width/3, 6))
	for i := start; i < end; i++ {
		row := m.visibleRows[i]
		owner := truncateAndPad(row.OwnerLabel, scopeWidth)
		match := m.searchMatches[row.Identity()]
		bodyIndicator := ""
		if match.bodyMatched {
			bodyIndicator = "  " + theme.MutedStyle.Render("· body")
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
		title := textfit.TruncateToVisualWidth(row.Note.Title, titleWidth)
		renderedTitle := title
		if len(match.titlePositions) > 0 {
			renderedTitle = textfit.HighlightText(title, match.titlePositions, theme.NormalStyle)
		}
		line := fmt.Sprintf("%s  %s%s", theme.MutedStyle.Render(owner), renderedTitle, bodyIndicator)
		if row.Note.ParseError != "" {
			line = theme.WarningIconStyle.Render("! ") + line
		}
		line = textfit.FitStyledLineToWidth(line, width)
		if i == m.cursor {
			b.WriteString(theme.PickerSelectedStyle.Render(line))
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
		line := textfit.TruncateToVisualWidth("! Workspace: "+m.snapshotError.Error(), width)
		return theme.WarningIconStyle.Render(line)
	}
	const maxErrorLines = 2
	var lines []string
	visibleFailures := min(len(m.collectionErrors), maxErrorLines)
	if len(m.collectionErrors) > maxErrorLines {
		visibleFailures = maxErrorLines - 1
	}
	for _, failure := range m.collectionErrors[:visibleFailures] {
		line := textfit.TruncateToVisualWidth(
			fmt.Sprintf("! %s: %v", failure.OwnerLabel, failure.Err),
			width,
		)
		lines = append(lines, theme.WarningIconStyle.Render(
			line,
		))
	}
	if remaining := len(m.collectionErrors) - visibleFailures; remaining > 0 {
		summary := fmt.Sprintf(
			"! %d more %s unavailable",
			remaining,
			pluralize(remaining, "owner", "owners"),
		)
		lines = append(lines, theme.WarningIconStyle.Render(textfit.TruncateToVisualWidth(summary, width)))
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
		caseNote := ""
		if m.searchCaseRelaxed {
			caseNote = " (case ignored)"
		}
		return fmt.Sprintf(
			"%d of %d notes match %q%s in %s",
			count,
			m.scopedRowCount(),
			query,
			caseNote,
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
		return theme.MutedStyle.Render(fmt.Sprintf("Search: %q", m.searchApplied))
	}
	return ""
}

func (m *NotesViewModel) locationLabel(location app.NoteLocation) string {
	return m.ownerForLocation(location).Label
}

func (m *NotesViewModel) renderStatusBar() string {
	if m.statusMessage == "" {
		return ""
	}
	if m.statusIsError {
		return theme.WarningIconStyle.Render(m.statusMessage)
	}
	return theme.SuccessStyle.Render(m.statusMessage)
}

// GetHelpItems documents the Notes grammar from the two sources that own it:
// the registry describes every operation the strategy binds, and the keymap
// names the cursor and undo/redo keys whose descriptors answer only to the
// Outliner. Keys documented by both keep the registry's spelling.
func (m *NotesViewModel) GetHelpItems() []HelpItem {
	if m.locationPicker != nil {
		return m.locationPicker.GetHelpItems()
	}
	if m.registry == nil {
		return nil
	}
	items := m.registry.HelpItemsForInputStrategy(InputStrategyNotes, m.notesCommandContext())
	documented := make(map[string]bool, len(items))
	for _, item := range items {
		documented[strings.ToLower(item.Key)] = true
	}
	for _, item := range helpItemsFromKeyMap(m.buildKeyMap()) {
		if !documented[strings.ToLower(item.Key)] {
			items = append(items, item)
		}
	}
	return items
}
