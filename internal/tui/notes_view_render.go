package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func (m *NotesViewModel) View() string {
	var b strings.Builder

	// Title
	var title string
	if m.isGlobal {
		title = "Global Notes"
	} else {
		title = fmt.Sprintf("%s - Notes", m.project.Name)
	}
	if len(m.filterTags) > 0 {
		title += " [filtered]"
	}
	b.WriteString(TitleStyle.Render(title))
	if len(m.filterTags) > 0 {
		tags := make([]string, 0, len(m.filterTags))
		for t := range m.filterTags {
			tags = append(tags, t)
		}
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" [tags: %s]", strings.Join(tags, ", "))))
	}
	b.WriteString("\n")

	// Tab bar and count info (hidden when bottom pane is fullscreen)
	if !m.fullscreenPane || !m.detailsPaneFocused {
		b.WriteString(m.renderNotesTabBar())
		b.WriteString("\n")

		// Note count and sort mode
		countInfo := fmt.Sprintf("%d notes", len(m.notes))
		if m.sortMode != noteSortModified {
			countInfo += fmt.Sprintf(" (sort: %s)", m.sortMode)
		}
		b.WriteString(MutedStyle.Render(countInfo))
		b.WriteString("\n\n")
	}

	// Fullscreen mode: show only the focused pane using full usable height
	usable := m.height - 8
	if usable < 4 {
		usable = 4
	}

	if m.fullscreenPane && m.detailsPaneFocused {
		// Fullscreen details pane
		fullHeight := usable
		var detailsContent string
		if len(m.notes) > 0 && m.cursor < len(m.notes) {
			detailsContent = m.renderDetailsPane(m.notes[m.cursor], m.width, fullHeight)
		}
		if detailsContent != "" {
			b.WriteString(detailsContent)
			detailLines := strings.Count(detailsContent, "\n") + 1
			for i := detailLines; i < fullHeight; i++ {
				b.WriteString("\n")
			}
		} else {
			for i := 0; i < fullHeight; i++ {
				b.WriteString("\n")
			}
		}
		b.WriteString(m.renderStatusBar())
	} else if m.fullscreenPane && !m.detailsPaneFocused {
		// Fullscreen list pane: temporarily override listHeight
		origListHeight := m.listHeight
		m.listHeight = usable
		b.WriteString(m.renderListPane())
		m.listHeight = origListHeight
		b.WriteString(m.renderFooter())
		b.WriteString("\n")
		b.WriteString(m.renderStatusBar())
	} else {
		// Normal split view
		b.WriteString(m.renderListPane())
		b.WriteString(m.renderFooter())
		b.WriteString("\n")

		// Details pane (render first so scroll values are clamped before building divider)
		detailsHeight := m.detailsHeight
		if detailsHeight < 1 {
			detailsHeight = 5
		}
		var detailsContent string
		if len(m.notes) > 0 && m.cursor < len(m.notes) {
			detailsContent = m.renderDetailsPane(m.notes[m.cursor], m.width, detailsHeight)
		}

		b.WriteString(m.renderDivider())
		b.WriteString("\n")

		// Output pre-rendered details
		if detailsContent != "" {
			b.WriteString(detailsContent)
			detailLines := strings.Count(detailsContent, "\n") + 1
			for i := detailLines; i < detailsHeight; i++ {
				b.WriteString("\n")
			}
		} else {
			for i := 0; i < detailsHeight; i++ {
				b.WriteString("\n")
			}
		}

		b.WriteString(m.renderStatusBar())
	}
	return b.String()
}

func (m *NotesViewModel) renderListPane() string {
	if len(m.notes) == 0 {
		return MutedStyle.Render("No notes yet. Press 'n' to create one.") + "\n"
	}

	var b strings.Builder
	pinnedStyle := lipgloss.NewStyle().Foreground(ColorSuccess)

	listHeight := m.listHeight
	if listHeight < 1 {
		listHeight = 10
	}
	noteListHeight := listHeight - 1 // -1 for footer in list pane
	if noteListHeight < 1 {
		noteListHeight = 1
	}

	startIdx := 0
	if m.cursor >= noteListHeight {
		startIdx = m.cursor - noteListHeight + 1
	}
	endIdx := startIdx + noteListHeight
	if endIdx > len(m.notes) {
		endIdx = len(m.notes)
	}

	selectedStyle := TaskJumperSelectedStyle
	if m.detailsPaneFocused {
		selectedStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	}

	for i := startIdx; i < endIdx; i++ {
		note := m.notes[i]
		isCurrent := i == m.cursor

		checkbox := ""
		if m.multiSelectMode {
			if m.isNoteSelected(note.Filename) {
				checkbox = "[x] "
			} else {
				checkbox = "[ ] "
			}
		}

		pin := " "
		if note.Pinned {
			pin = pinnedStyle.Render("P")
		}

		titleStr := note.Title
		maxTitle := 40
		if m.multiSelectMode {
			maxTitle = 36
		}
		if len(titleStr) > maxTitle {
			titleStr = titleStr[:maxTitle-3] + "..."
		}
		paddedTitle := fmt.Sprintf("%-40s", checkbox+titleStr)

		tags := ""
		if len(note.Tags) > 0 {
			tags = " [" + strings.Join(note.Tags, ", ") + "]"
		}

		linked := ""
		if len(note.LinkedTasks) > 0 && !m.isGlobal {
			linked = fmt.Sprintf(" (%d)", len(note.LinkedTasks))
		}

		age := formatNoteAge(note.ModifiedAt)

		if note.Pinned {
			paddedTitle = pinnedStyle.Render(paddedTitle)
		}

		row := fmt.Sprintf("%s %s %8s%s%s", pin, paddedTitle, age, tags, linked)

		if isCurrent {
			b.WriteString(selectedStyle.Render(row))
		} else {
			b.WriteString(row)
		}
		b.WriteString("\n")
	}
	// Pad remaining list lines
	rendered := endIdx - startIdx
	for i := rendered; i < noteListHeight; i++ {
		b.WriteString("\n")
	}
	return b.String()
}

func (m *NotesViewModel) renderFooter() string {
	if m.renaming {
		return MutedStyle.Render("Enter: confirm  Esc: cancel")
	}
	if m.detailsPaneFocused {
		zHint := "Z:fullscreen"
		if m.fullscreenPane {
			zHint = "Z:exit fullscreen"
		}
		return MutedStyle.Render("j/k: scroll  g/G: top/bottom  Ctrl+d/u: half page  Tab/Esc: list pane  Enter: view  e: edit  " + zHint + "  q: close")
	}
	if m.multiSelectMode {
		return MutedStyle.Render("space: toggle  a: all  gX: clear  d: delete  t: tag  l: link  y: copy  V/Esc: exit multi-select")
	}
	zHint := "Z:fullscreen"
	if m.fullscreenPane {
		zHint = "Z:exit fullscreen"
	}
	footer := "j/k: nav  Tab: pane  e: edit  n: new  d: del  r: rename  s: sort  y: copy  " + zHint
	if m.isGlobal {
		footer += "  m: move to project"
	} else {
		footer += "  /: search"
		if len(m.filterTags) > 0 {
			footer += "  C: clear filter"
		}
	}
	return MutedStyle.Render(footer)
}

func (m *NotesViewModel) renderDivider() string {
	dividerColor := lipgloss.TerminalColor(ColorMuted)
	if m.detailsPaneFocused {
		dividerColor = ColorPrimary
	}

	var opts []DividerOption
	opts = append(opts, WithDividerColor(dividerColor))

	if m.detailsMaxScroll > 0 && len(m.notes) > 0 && m.cursor < len(m.notes) {
		indicator := fmt.Sprintf(" [%d/%d] ", m.detailsScroll+1, m.detailsMaxScroll+1)
		opts = append(opts, WithRightIndicator(indicator))
	}

	return renderHorizontalDivider(m.width, opts...)
}

func (m *NotesViewModel) renderStatusBar() string {
	if m.renaming {
		return "Rename: " + m.renameInput.View()
	}
	if m.confirmDelete {
		if m.pendingDeleteIdx < len(m.notes) {
			return lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("Press d again to delete: %s", m.notes[m.pendingDeleteIdx].Title))
		}
		return ""
	}
	if m.statusMessage != "" {
		if m.statusIsError {
			return lipgloss.NewStyle().Foreground(ColorWarning).Render(m.statusMessage)
		}
		return SuccessStyle.Render(m.statusMessage)
	}
	return ""
}

func (m *NotesViewModel) renderNotesTabBar() string {
	return RenderTabBar(noteTabNames, m.activeTab, min(m.width-6, 54)-2, 1)
}

func (m *NotesViewModel) renderDetailsPane(note *models.Note, width, height int) string {
	if note == nil || height <= 0 {
		return ""
	}

	var lines []string

	// Title
	titleLine := TitleStyle.Render(note.Title)
	lines = append(lines, titleLine)

	// Tags
	if len(note.Tags) > 0 {
		lines = append(lines, MutedStyle.Render("Tags: "+strings.Join(note.Tags, ", ")))
	}

	// Linked tasks
	if len(note.LinkedTasks) > 0 && !m.isGlobal {
		for _, taskID := range note.LinkedTasks {
			taskTitle := "[deleted]"
			if m.project != nil {
				if task := m.project.GetTaskByID(taskID); task != nil {
					taskTitle = task.Title
				}
			}
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  → %s", taskTitle)))
		}
	}

	lines = append(lines, "")

	// Body: wrap lines to width
	if note.Body != "" {
		bodyLines := strings.Split(note.Body, "\n")
		for _, bl := range bodyLines {
			if len(bl) <= width && width > 0 {
				lines = append(lines, bl)
			} else if width > 0 {
				// Word wrap; forces break when no space found (e.g. URLs)
				for len(bl) > width {
					cut := width
					if idx := strings.LastIndex(bl[:cut], " "); idx > 0 {
						cut = idx + 1
					}
					lines = append(lines, bl[:cut])
					bl = bl[cut:]
				}
				lines = append(lines, bl)
			}
		}
	}

	// Clamp scroll so the last line is always visible at the bottom
	maxScroll := len(lines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.detailsMaxScroll = maxScroll
	if m.detailsScroll > maxScroll {
		m.detailsScroll = maxScroll
	}
	if m.detailsScroll < 0 {
		m.detailsScroll = 0
	}

	// Apply scroll offset and cap to available height
	lines = lines[m.detailsScroll:]
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// GetHelpItems returns keybinding help for the notes view.
func (m *NotesViewModel) GetHelpItems() []HelpItem {
	items := []HelpItem{
		{Key: "[/]", Description: "Switch tab"},
		{Key: "j/k", Description: "Navigate up/down"},
		{Key: "Tab", Description: "Switch pane (list/details)"},
		{Key: "Enter", Description: "View note (glow)"},
		{Key: "e", Description: "Edit note"},
		{Key: "n", Description: "Create new note"},
		{Key: "d", Description: "Delete note (confirm)"},
		{Key: "u", Description: "Undo last delete"},
		{Key: "r", Description: "Rename note"},
		{Key: "s", Description: "Cycle sort mode"},
		{Key: "y", Description: "Copy title to clipboard"},
		{Key: "Y", Description: "Copy body to clipboard"},
		{Key: "Ctrl+y", Description: "Copy title and body to clipboard"},
	}
	if m.isGlobal {
		items = append(items, HelpItem{Key: "m", Description: "Move note to project"})
	} else {
		items = append(items, HelpItem{Key: "l", Description: "Link note to task"})
	}
	items = append(items,
		HelpItem{Key: "p", Description: "Toggle pin"},
		HelpItem{Key: "t", Description: "Edit note tags"},
	)
	if !m.isGlobal {
		items = append(items,
			HelpItem{Key: "T", Description: "Filter by tags"},
			HelpItem{Key: "C", Description: "Clear tag filter"},
			HelpItem{Key: "/", Description: "Search notes"},
		)
	}
	items = append(items,
		HelpItem{Key: "g/G", Description: "Jump to top/bottom"},
		HelpItem{Key: "V", Description: "Toggle multi-select mode"},
		HelpItem{Key: "space", Description: "Toggle selection (multi-select)"},
		HelpItem{Key: "a", Description: "Select all (multi-select)"},
		HelpItem{Key: "gX", Description: "Deselect all (multi-select)"},
		HelpItem{Key: "Z", Description: "Toggle fullscreen pane"},
		HelpItem{Key: "q", Description: "Close notes view"},
	)
	return items
}

func formatNoteAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < util.Day:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
