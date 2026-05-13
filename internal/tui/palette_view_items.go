package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderPaletteItemWithHighlight renders a palette item with match highlighting.
// numberHint is 1-9 for items that can be selected with number keys, or -1 to hide.
func (m *CommandPaletteModel) renderPaletteItemWithHighlight(filtered filteredPaletteItem, selected bool, width int, numberHint int) string {
	item := filtered.Item

	checkbox := ""
	checkboxLen := 0
	if item.Category == ItemCategoryTasks && item.Task != nil {
		if m.selectedTasks[item.Task.ID] {
			checkbox = SelectedCheckboxStyle.Render(icons.Get("checkbox.checked") + " ")
		} else if len(m.selectedTasks) > 0 {
			checkbox = MutedStyle.Render(icons.Get("checkbox.unchecked") + " ")
		}
		if checkbox != "" {
			checkboxLen = 2
		}
	}

	cursor := "  "
	if numberHint > 0 && numberHint <= 9 {
		cursor = MutedStyle.Render(fmt.Sprintf("%d ", numberHint))
	}

	pinIndicator := ""
	pinLen := 0
	if m.isItemPinned(item) {
		pinIndicator = PinIndicatorStyle.Render("*")
		pinLen = 1
	}

	catBadge, catBadgeLen := m.renderCategoryBadge(item.Category, selected)

	shortcutText := ""
	if item.Shortcut != "" {
		shortcutText = item.Shortcut
	} else if item.Category != ItemCategoryCommands {
		shortcutText = "Tab"
	}

	const shortcutColWidth = 8
	shortcutReserved := 0
	if shortcutText != "" {
		shortcutReserved = shortcutColWidth
	}

	fixedWidth := 2 + checkboxLen + pinLen + catBadgeLen + 1 + shortcutReserved
	remainingWidth := max(width-fixedWidth, 20)
	leftContent, leftLen := m.renderItemContentWithHighlight(item, filtered.NamePositions, remainingWidth)

	gap := remainingWidth - leftLen
	shortcutDisplayLen := len(shortcutText)
	shortcutPad := max(shortcutColWidth-shortcutDisplayLen, 0)
	totalGap := max(gap+shortcutPad, 1)

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
func (m *CommandPaletteModel) renderCategoryBadge(cat ItemCategory, selected bool) (string, int) {
	var badge string
	var style lipgloss.Style

	switch cat {
	case ItemCategoryCommands:
		badge = "> CMD"
		style = CategoryCommandStyle
	case ItemCategoryTasks:
		badge = icons.Get("checkbox.unchecked") + " TSK"
		style = CategoryTaskStyle
	case ItemCategoryProjects:
		badge = icons.Get("blocked_dot") + " PRJ"
		style = CategoryProjectStyle
	case ItemCategoryTags:
		badge = "# TAG"
		style = CategoryTagStyle
	case ItemCategorySections:
		badge = "§ SEC"
		style = CategorySectionStyle
	case ItemCategorySavedSearch:
		badge = icons.Get("star") + " SAV"
		style = CategorySavedSearchStyle
	case ItemCategorySavedFilter:
		badge = "⊡ SVW"
		style = CategorySavedFilterStyle
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

	if selected {
		style = style.Foreground(ColorHighlight).Background(ColorSelectedBg).Bold(true)
	}

	return style.Render(badge), 5
}

// getDueDateIndicator returns a colored indicator for task due date urgency.
// Returns empty string if no due date or not urgent.
func getDueDateIndicator(task *models.Task) (string, int) {
	if task == nil || task.DueDate == nil {
		return "", 0
	}

	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*task.DueDate)

	if dueDay.Before(today) {
		return OverdueStyle.Render("!") + " ", 2
	}
	if dueDay.Equal(today) {
		return DueTodayStyle.Render("!") + " ", 2
	}
	weekFromNow := today.AddDate(0, 0, 7)
	if dueDay.Before(weekFromNow) {
		return DueThisWeekStyle.Render("~") + " ", 2
	}

	return "", 0
}

// renderItemContentWithHighlight renders item content with match highlighting on the name.
func (m *CommandPaletteModel) renderItemContentWithHighlight(item PaletteItem, namePositions []int, maxWidth int) (string, int) {
	var content strings.Builder
	totalLen := 0

	if item.Category == ItemCategoryTasks && item.Task != nil {
		dueIndicator, dueLen := getDueDateIndicator(item.Task)
		if dueLen > 0 {
			content.WriteString(dueIndicator)
			totalLen += dueLen
		}

		stageBadge := GetStageBadge(item.Task.Stage)
		stageSegment := ""
		if stageBadge != "" {
			stageSegment = stageBadge + " "
		}

		icons := fmt.Sprintf("%s %s%s ",
			GetTypeBadge(string(item.Task.Type), item.Task.Type.Icon()),
			stageSegment,
			GetStatusBadge(string(item.Task.Status), item.Task.Status.Icon()))
		content.WriteString(icons)
		totalLen += 4 + lipgloss.Width(stageSegment)
	}

	maxNameLen := max(maxWidth-totalLen, 10)
	name := truncateToVisualWidth(item.Name, maxNameLen)

	truncatedPositions := namePositions
	if lipgloss.Width(name) < lipgloss.Width(item.Name) {
		var validPositions []int
		nameRunes := []rune(name)
		for _, position := range namePositions {
			if position < len(nameRunes) {
				validPositions = append(validPositions, position)
			}
		}
		truncatedPositions = validPositions
	}

	content.WriteString(HighlightText(name, truncatedPositions, BoldStyle))
	totalLen += lipgloss.Width(name)

	if item.Description != "" {
		descMaxLen := maxWidth - totalLen - 3
		if descMaxLen > 8 {
			desc := truncateToVisualWidth(item.Description, descMaxLen)
			content.WriteString(" ")
			content.WriteString(MutedStyle.Render("· " + desc))
			totalLen += 3 + lipgloss.Width(desc)
		}
	}

	return content.String(), totalLen
}
