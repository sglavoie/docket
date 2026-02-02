package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Note: HighlightStyle and HighlightText are defined in highlight.go

// View implements tea.Model.
func (m *CommandPaletteModel) View() string {
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
		return out.String()
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if !m.actionMode && len(m.filteredItems) == 0 && query != "" {
		var out strings.Builder
		out.WriteString(renderFullscreenTitle("Command Palette", fmt.Sprintf("0/%d matching", len(m.allItems))))
		out.WriteString("\n")
		out.WriteString(content.String())
		out.WriteString(MutedStyle.Render(fmt.Sprintf("No matches for \"%s\"", query)))
		out.WriteString("\n\n")
		out.WriteString(HelpStyle.Render("Try: :cmd for commands • :task for tasks • :prj for projects"))
		out.WriteString("\n")
		out.WriteString(HelpStyle.Render("     :tag :sec :note :view • or clear to browse all"))
		return out.String()
	}

	// Render content based on mode
	var listContent string
	if m.actionMode {
		listContent = m.renderActionList()
	} else {
		listContent = m.renderItemList()
	}

	paneHeight := m.height - 8 // title + input + spacing + footer
	if paneHeight < 5 {
		paneHeight = 5
	}

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

	return out.String()
}

// buildSubtitle builds the subtitle showing item counts.
func (m *CommandPaletteModel) buildSubtitle() string {
	if m.actionMode {
		return fmt.Sprintf("%d available", len(m.availableActions))
	}

	// Count by category
	counts := make(map[ItemCategory]int)
	for _, filtered := range m.filteredItems {
		counts[filtered.Item.Category]++
	}

	var parts []string
	if c := counts[ItemCategoryCommands]; c > 0 {
		parts = append(parts, fmt.Sprintf("%dc", c))
	}
	if c := counts[ItemCategoryTasks]; c > 0 {
		parts = append(parts, fmt.Sprintf("%dt", c))
	}
	if c := counts[ItemCategoryProjects]; c > 0 {
		parts = append(parts, fmt.Sprintf("%dp", c))
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%d items", len(m.filteredItems))
	}
	return strings.Join(parts, " ")
}

// buildFooter constructs the footer text with navigation hints and category counts.
// Adapts to available width by showing shorter hints when space is limited.
func (m *CommandPaletteModel) buildFooter() string {
	// Count items by category in the filtered results
	counts := make(map[ItemCategory]int)
	for _, filtered := range m.filteredItems {
		counts[filtered.Item.Category]++
	}

	// Build count summary (only non-zero categories, compact format)
	var countParts []string
	if c := counts[ItemCategorySavedSearch]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%d★", c))
	}
	if c := counts[ItemCategoryCommands]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%dc", c))
	}
	if c := counts[ItemCategoryProjects]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%dp", c))
	}
	if c := counts[ItemCategoryTasks]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%dt", c))
	}
	if c := counts[ItemCategoryTags]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%dg", c))
	}
	if c := counts[ItemCategorySections]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%ds", c))
	}
	if c := counts[ItemCategoryNotes]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%dn", c))
	}
	if c := counts[ItemCategoryViews]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%dv", c))
	}

	countSummary := strings.Join(countParts, " ")
	if countSummary == "" {
		countSummary = "0"
	}

	// Add toggle indicators (compact)
	var toggles []string
	if m.includeCompleted {
		toggles = append(toggles, "+H")
	}
	if m.sortField != SortByDefault {
		switch m.sortField {
		case SortByPriority:
			toggles = append(toggles, "P↓")
		case SortByDueDate:
			toggles = append(toggles, "D↓")
		}
	}
	if m.compactMode {
		toggles = append(toggles, "◫")
	}
	toggleSummary := ""
	if len(toggles) > 0 {
		toggleSummary = " [" + strings.Join(toggles, " ") + "]"
	}

	query := strings.TrimSpace(m.searchInput.Value())

	// When browsing (empty query), show syntax cheat sheet instead of key hints
	if query == "" {
		return "+ create | : filter | # jump | \"exact\" | -exclude | @due | priority:N"
	}

	// Show keyhint bar when direct actions are enabled and cursor is on a task
	if m.isDirectActionEnabled() && len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
		item := m.filteredItems[m.cursor].Item
		if item.Category == ItemCategoryTasks && item.Task != nil {
			return fmt.Sprintf("e=Edit g=View x=Complete s=Start b=Block r=Reopen | %s%s", countSummary, toggleSummary)
		}
	}

	// Build compact hints
	hints := "↑↓ • Enter • Tab • ^H • ^O • ^P • Esc"

	return fmt.Sprintf("%s | %s%s", hints, countSummary, toggleSummary)
}

// buildActionModeFooter builds a compact footer for action mode.
func (m *CommandPaletteModel) buildActionModeFooter() string {
	return "↑↓ Enter Tab Esc"
}

// renderActionList renders the list of available actions for the selected item.
func (m *CommandPaletteModel) renderActionList() string {
	if len(m.availableActions) == 0 {
		return MutedStyle.Render("No actions available.")
	}

	var b strings.Builder
	contentWidth := m.listWidth - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Calculate max visible items based on pane height
	paneHeight := m.height - 8
	if paneHeight < 5 {
		paneHeight = 5
	}
	maxVisible := paneHeight - 2 // Account for item description header
	if maxVisible < 3 {
		maxVisible = 3
	}

	// Calculate scroll offset to keep cursor visible
	if m.actionCursor < m.actionScrollOffset {
		m.actionScrollOffset = m.actionCursor
	} else if m.actionCursor >= m.actionScrollOffset+maxVisible {
		m.actionScrollOffset = m.actionCursor - maxVisible + 1
	}

	// Render only visible items
	endIdx := m.actionScrollOffset + maxVisible
	if endIdx > len(m.availableActions) {
		endIdx = len(m.availableActions)
	}

	for i := m.actionScrollOffset; i < endIdx; i++ {
		line := m.renderActionItem(m.availableActions[i], i == m.actionCursor, contentWidth)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderActionItem renders a single action row.
// Layout: [cursor] [icon] [name] ... [key hint]
func (m *CommandPaletteModel) renderActionItem(action PaletteAction, selected bool, width int) string {
	// Cursor indicator (2 chars)
	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Icon with space after
	icon := action.Icon
	if icon == "" {
		icon = "•"
	}
	icon = icon + " "
	iconWidth := lipgloss.Width(icon)

	// Key hint for right alignment
	keyHint := ""
	keyHintLen := 0
	if action.KeyHint != "" {
		keyHint = action.KeyHint
		keyHintLen = len(keyHint) + 2 // +2 for spacing
	}

	// Calculate remaining width for name using actual icon width
	fixedWidth := 2 + iconWidth + keyHintLen + 2 // cursor + icon + key hint + gap
	nameWidth := width - fixedWidth
	if nameWidth < 10 {
		nameWidth = 10
	}

	// Truncate name if needed
	name := action.Name
	if len(name) > nameWidth {
		name = name[:nameWidth-1] + "…"
	}

	// Calculate gap for right-alignment
	gap := nameWidth - len(name)
	if gap < 1 {
		gap = 1
	}

	// Build the line
	var line strings.Builder
	line.WriteString(cursor)
	line.WriteString(icon)
	line.WriteString(BoldStyle.Render(name))

	if keyHint != "" {
		line.WriteString(strings.Repeat(" ", gap))
		line.WriteString(HelpKeyStyle.Render(keyHint))
	}

	result := line.String()
	if selected {
		result = TaskJumperSelectedStyle.Render(result)
	}

	return result
}

// countLinesForRange counts how many display lines are needed to render items
// from startIdx to endIdx (inclusive), including category headers and spacing.
// This is used to calculate scroll offsets that account for header overhead.
func (m *CommandPaletteModel) countLinesForRange(startIdx, endIdx int, showHeaders bool) int {
	if startIdx < 0 || endIdx < startIdx || len(m.filteredItems) == 0 {
		return 0
	}
	if endIdx >= len(m.filteredItems) {
		endIdx = len(m.filteredItems) - 1
	}

	lines := 0
	var currentCategory ItemCategory

	// If starting from middle, we need the category of the previous item
	// to know if the first item in our range triggers a header
	if startIdx > 0 {
		currentCategory = m.filteredItems[startIdx-1].Item.Category
	}

	for i := startIdx; i <= endIdx; i++ {
		item := m.filteredItems[i].Item

		if showHeaders && item.Category != currentCategory {
			if currentCategory != "" {
				lines++ // spacing before new category
			}
			lines++ // header line
			currentCategory = item.Category
		}
		lines++ // item line
	}

	return lines
}

// calculateScrollOffset determines the scroll offset needed to keep the cursor
// visible within maxVisible lines, accounting for category headers.
// It preserves the current scroll position when possible, only adjusting when
// the cursor would be outside the visible range.
func (m *CommandPaletteModel) calculateScrollOffset(maxVisible int, showHeaders bool) int {
	if m.cursor < 0 || len(m.filteredItems) == 0 {
		m.scrollOffset = 0
		return 0
	}

	// Handle scroll anchor (used by Ctrl+N section navigation)
	if m.scrollAnchor >= 0 {
		m.scrollOffset = m.scrollAnchor
		m.scrollAnchor = -1
		return m.scrollOffset
	}

	// Clamp scrollOffset to valid range
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if m.scrollOffset >= len(m.filteredItems) {
		m.scrollOffset = len(m.filteredItems) - 1
	}

	// Check if cursor is above the visible range (scrolled past the top)
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
		return m.scrollOffset
	}

	// Check if cursor is below the visible range
	linesNeeded := m.countLinesForRange(m.scrollOffset, m.cursor, showHeaders)
	if linesNeeded <= maxVisible {
		// Cursor is visible, keep current scroll position
		return m.scrollOffset
	}

	// Cursor is below visible range, scroll down until it's visible
	for m.scrollOffset < len(m.filteredItems) {
		linesNeeded = m.countLinesForRange(m.scrollOffset, m.cursor, showHeaders)
		if linesNeeded <= maxVisible {
			break
		}
		m.scrollOffset++
	}

	return m.scrollOffset
}

// renderItemList renders the filtered item list with cursor indicator.
// Each row displays: cursor + category badge + name (prominent) + extra info + shortcut (right-aligned)
// When no search query is active, items are grouped by category with section headers.
func (m *CommandPaletteModel) renderItemList() string {
	if len(m.filteredItems) == 0 {
		return MutedStyle.Render("Type to search commands, tasks, projects...")
	}

	// Determine if we should show category headers (only when browsing, not searching)
	query := strings.TrimSpace(m.searchInput.Value())
	categoryFilter, searchQuery := m.parseQueryPrefix(query)
	showHeaders := searchQuery == "" && categoryFilter == ""

	var b strings.Builder

	// Show contextual hints based on current state
	if hint := m.getContextualHint(); hint != "" {
		b.WriteString(HelpStyle.Render(hint))
		b.WriteString("\n\n")
	}

	// Calculate pane height (must match View's paneHeight calculation: m.height - 8)
	paneHeight := m.height - 8
	if paneHeight < 5 {
		paneHeight = 5
	}

	// maxVisible accounts for hints and category headers within the pane
	maxVisible := paneHeight
	if m.getContextualHint() != "" {
		maxVisible -= 2 // Account for hint lines
	}
	if maxVisible < 5 {
		maxVisible = 5
	}

	// Available width for content - use listWidth, not full width
	contentWidth := m.listWidth - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Calculate scroll offset to keep cursor visible (accounts for headers)
	scrollOffset := m.calculateScrollOffset(maxVisible, showHeaders)

	// Track current category for header insertion
	// Initialize to previous item's category so we correctly detect category changes
	var currentCategory ItemCategory
	if scrollOffset > 0 {
		currentCategory = m.filteredItems[scrollOffset-1].Item.Category
	}
	linesRendered := 0

	for i := scrollOffset; i < len(m.filteredItems) && linesRendered < maxVisible; i++ {
		filtered := m.filteredItems[i]
		item := filtered.Item

		// Insert category header when category changes (only in browse mode)
		if showHeaders && item.Category != currentCategory {
			// Don't count the header against visible items for first category
			if currentCategory != "" {
				// Add spacing before new category (except first)
				b.WriteString("\n")
				linesRendered++
				if linesRendered >= maxVisible {
					break
				}
			}
			header := m.renderCategoryHeader(item.Category)
			b.WriteString(header)
			b.WriteString("\n")
			linesRendered++
			currentCategory = item.Category
			if linesRendered >= maxVisible {
				break
			}
		}

		// Pass the index for number hint display (only first 9 items when jump is enabled)
		numberHint := -1
		if m.isNumberJumpEnabled() && i < 9 {
			numberHint = i + 1 // 1-9 for first 9 items
		}
		line := m.renderPaletteItemWithHighlight(filtered, i == m.cursor, contentWidth, numberHint)
		b.WriteString(line)
		b.WriteString("\n")
		linesRendered++
	}

	// Show scroll progress indicator if there are more items
	if len(m.filteredItems) > maxVisible {
		b.WriteString(m.renderScrollIndicator(scrollOffset, maxVisible, contentWidth))
	}

	return b.String()
}

// renderScrollIndicator renders a visual progress bar showing scroll position.
func (m *CommandPaletteModel) renderScrollIndicator(scrollOffset, maxVisible, width int) string {
	total := len(m.filteredItems)
	if total <= maxVisible {
		return ""
	}

	// Calculate progress percentage
	maxScroll := total - maxVisible
	if maxScroll <= 0 {
		maxScroll = 1
	}
	progress := float64(scrollOffset) / float64(maxScroll)
	if progress > 1 {
		progress = 1
	}

	// Build progress bar
	barWidth := width - 20 // Leave room for text
	if barWidth < 10 {
		barWidth = 10
	}
	filled := int(float64(barWidth) * progress)
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}

	// Create the bar
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Position info
	showing := scrollOffset + 1
	showingEnd := scrollOffset + maxVisible
	if showingEnd > total {
		showingEnd = total
	}
	posInfo := fmt.Sprintf(" %d-%d/%d", showing, showingEnd, total)

	return MutedStyle.Render(bar + posInfo)
}

// renderCategoryHeader renders a section header for a category.
func (m *CommandPaletteModel) renderCategoryHeader(cat ItemCategory) string {
	name := getCategoryDisplayName(cat)
	// Count items in this category
	count := 0
	for _, filtered := range m.filteredItems {
		if filtered.Item.Category == cat {
			count++
		}
	}
	header := fmt.Sprintf("── %s (%d) ", name, count)
	return MutedStyle.Render(header)
}

// renderPaletteItemWithHighlight renders a palette item with match highlighting.
// numberHint is 1-9 for items that can be selected with number keys, or -1 to hide.
func (m *CommandPaletteModel) renderPaletteItemWithHighlight(filtered filteredPaletteItem, selected bool, width int, numberHint int) string {
	item := filtered.Item

	// Selection checkbox for tasks (2 chars)
	checkbox := ""
	checkboxLen := 0
	if item.Category == ItemCategoryTasks && item.Task != nil {
		if m.selectedTasks[item.Task.ID] {
			checkbox = SelectedCheckboxStyle.Render("☑ ")
		} else if len(m.selectedTasks) > 0 {
			// Show empty checkbox when in multi-select mode
			checkbox = MutedStyle.Render("☐ ")
		}
		if checkbox != "" {
			checkboxLen = 2
		}
	}

	// Cursor/number indicator (2 chars)
	cursor := "  "
	if selected {
		cursor = "> "
	} else if numberHint > 0 && numberHint <= 9 {
		// Show number hint in muted style
		cursor = MutedStyle.Render(fmt.Sprintf("%d ", numberHint))
	}

	// Pin indicator for pinned commands (shown before badge)
	pinIndicator := ""
	pinLen := 0
	if m.isItemPinned(item) {
		pinIndicator = PinIndicatorStyle.Render("*")
		pinLen = 1
	}

	// Category badge with color (5-6 chars including brackets)
	catBadge, catBadgeLen := m.renderCategoryBadge(item.Category)

	// Shortcut string for right alignment
	shortcutText := ""
	if item.Shortcut != "" {
		shortcutText = item.Shortcut
	} else if item.Category != ItemCategoryCommands {
		shortcutText = "Tab"
	}

	// Use a fixed shortcut column width so all shortcuts right-align neatly
	const shortcutColWidth = 8
	shortcutReserved := 0
	if shortcutText != "" {
		shortcutReserved = shortcutColWidth
	}

	// Calculate space for name + extra info + description
	fixedWidth := 2 + checkboxLen + pinLen + catBadgeLen + 1 + shortcutReserved
	remainingWidth := width - fixedWidth
	if remainingWidth < 20 {
		remainingWidth = 20
	}

	// Build the left content with highlighting
	leftContent, leftLen := m.renderItemContentWithHighlight(item, filtered.NamePositions, remainingWidth)

	// Calculate gap to right-align shortcut within the fixed column
	gap := remainingWidth - leftLen
	shortcutDisplayLen := len(shortcutText)
	// Right-align the shortcut text within the reserved column
	shortcutPad := shortcutColWidth - shortcutDisplayLen
	if shortcutPad < 0 {
		shortcutPad = 0
	}
	totalGap := gap + shortcutPad
	if totalGap < 1 {
		totalGap = 1
	}

	// Build the complete line
	var line strings.Builder
	line.WriteString(cursor)
	line.WriteString(checkbox)
	line.WriteString(pinIndicator)
	line.WriteString(catBadge)
	line.WriteString(" ")
	line.WriteString(leftContent)

	if shortcutText != "" {
		line.WriteString(strings.Repeat(" ", totalGap))
		line.WriteString(HelpKeyStyle.Render(shortcutText))
	}

	result := line.String()
	if selected {
		result = TaskJumperSelectedStyle.Render(result)
	}

	return result
}

// renderCategoryBadge returns the styled category badge and its display length.
// Badges include an icon prefix for faster visual scanning.
func (m *CommandPaletteModel) renderCategoryBadge(cat ItemCategory) (string, int) {
	var badge string
	var style lipgloss.Style

	switch cat {
	case ItemCategoryCommands:
		badge = "> CMD"
		style = CategoryCommandStyle
	case ItemCategoryTasks:
		badge = "☐ TSK"
		style = CategoryTaskStyle
	case ItemCategoryProjects:
		badge = "◈ PRJ"
		style = CategoryProjectStyle
	case ItemCategoryTags:
		badge = "# TAG"
		style = CategoryTagStyle
	case ItemCategorySections:
		badge = "§ SEC"
		style = CategorySectionStyle
	case ItemCategorySavedSearch:
		badge = "★ SAV"
		style = CategorySavedSearchStyle
	case ItemCategoryNotes:
		badge = "✎ NTE"
		style = CategoryNoteStyle
	case ItemCategoryViews:
		badge = "⊞ VEW"
		style = CategoryViewStyle
	default:
		badge = "? ???"
		style = MutedStyle
	}

	return style.Render(badge), 5 // All badges are 5 display characters
}

// getDueDateIndicator returns a colored indicator for task due date urgency.
// Returns empty string if no due date or not urgent.
func getDueDateIndicator(task *models.Task) (string, int) {
	if task == nil || task.DueDate == nil {
		return "", 0
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	if dueDay.Before(today) {
		// Overdue - red exclamation
		return OverdueStyle.Render("!") + " ", 2
	}
	if dueDay.Equal(today) {
		// Due today - orange exclamation
		return DueTodayStyle.Render("!") + " ", 2
	}
	weekFromNow := today.AddDate(0, 0, 7)
	if dueDay.Before(weekFromNow) {
		// Due this week - yellow tilde
		return DueThisWeekStyle.Render("~") + " ", 2
	}

	return "", 0
}

// renderItemContentWithHighlight renders item content with match highlighting on the name.
func (m *CommandPaletteModel) renderItemContentWithHighlight(item PaletteItem, namePositions []int, maxWidth int) (string, int) {
	var content strings.Builder
	totalLen := 0

	// Task-specific: show due date indicator and type/status icons before name
	if item.Category == ItemCategoryTasks && item.Task != nil {
		dueIndicator, dueLen := getDueDateIndicator(item.Task)
		if dueLen > 0 {
			content.WriteString(dueIndicator)
			totalLen += dueLen
		}

		icons := fmt.Sprintf("%s %s ",
			GetTypeBadge(string(item.Task.Type), item.Task.Type.Icon()),
			GetStatusBadge(string(item.Task.Status), item.Task.Status.Icon()))
		content.WriteString(icons)
		totalLen += 4
	}

	// Give the name its full length; description gets whatever space remains
	maxNameLen := maxWidth - totalLen
	if maxNameLen < 10 {
		maxNameLen = 10
	}

	// Name with highlighting (truncated if needed)
	name := util.Truncate(item.Name, maxNameLen)
	// Adjust positions if name was truncated
	truncatedPositions := namePositions
	if len(name) < len(item.Name) {
		var validPositions []int
		nameRunes := []rune(name)
		for _, p := range namePositions {
			if p < len(nameRunes) {
				validPositions = append(validPositions, p)
			}
		}
		truncatedPositions = validPositions
	}
	content.WriteString(HighlightText(name, truncatedPositions, BoldStyle))
	totalLen += len([]rune(name))

	// Description (dimmed, if space allows)
	if item.Description != "" {
		descMaxLen := maxWidth - totalLen - 3
		if descMaxLen > 8 {
			desc := util.Truncate(item.Description, descMaxLen)
			content.WriteString(" ")
			content.WriteString(MutedStyle.Render("· " + desc))
			totalLen += 3 + len(desc)
		}
	}

	return content.String(), totalLen
}

// getContextualHint returns a context-aware hint based on current palette state.
// Returns empty string if no hint should be shown.
func (m *CommandPaletteModel) getContextualHint() string {
	query := strings.TrimSpace(m.searchInput.Value())

	// Show hints only when palette first opens or when browsing without search
	if !m.showTips {
		return ""
	}

	// If user has typed something, hide tips
	if query != "" && !strings.HasPrefix(query, ":") {
		return ""
	}

	// Context-aware hints based on current selection
	if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
		item := m.filteredItems[m.cursor].Item
		switch item.Category {
		case ItemCategoryTasks:
			if m.isDirectActionEnabled() {
				return "Tip: e=edit  g=view  x=complete  s=start  b=block  r=reopen  Tab=more"
			}
			return "Tip: Press Tab for quick actions, or Enter to go to task"
		case ItemCategoryCommands:
			if item.Command != nil && item.Command.Shortcut != "" {
				return fmt.Sprintf("Tip: Press %s to execute, or Enter to confirm", item.Command.Shortcut)
			}
			return "Tip: Press Enter to execute command"
		case ItemCategoryProjects:
			return "Tip: Enter=open project  Tab=more actions (edit, archive)"
		case ItemCategoryTags:
			return "Tip: Enter=filter by tag  Tab=manage tags"
		case ItemCategorySections:
			return "Tip: Enter=filter by section"
		}
	}

	// Default tips when browsing with no specific selection context
	if query == "" {
		return "Tip: Type to search, or use :cmd :task :prj :tag :sec :note :view to filter"
	}

	// Category prefix hints
	categoryFilter, _ := m.parseQueryPrefix(query)
	if categoryFilter != "" {
		switch categoryFilter {
		case ItemCategoryTasks:
			return "Tip: Use priority:1-2 for high priority, @overdue for overdue tasks"
		case ItemCategoryCommands:
			return "Tip: Commands show their keyboard shortcut on the right"
		}
	}

	return ""
}
