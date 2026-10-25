package listsurface

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// selectionMark is the Outliner's selection glyph, worn by a selected row of a
// ListSelectionMark surface.
const selectionMark = "✓"

// NaturalRowWidth is the width the widest row would draw at with nothing
// truncated: the cursor and checkbox columns, the marker, the primary text, the
// gap, and the secondary text, as RenderListSurfaceRow lays them out, and every
// category header. It reads every source row rather than the rows a query left,
// so a split sized from it holds still while the user types.
func (model *ListSurfaceModel) NaturalRowWidth() int {
	if model == nil {
		return 0
	}
	widest := 0
	for _, category := range model.categories {
		widest = max(widest, lipgloss.Width(strings.ToUpper(category.Label)))
	}
	chromeWidth := 2 // the cursor column
	if model.config.SelectionMode == ListSelectionMulti {
		chromeWidth += lipgloss.Width("[ ] ")
	}
	for _, item := range model.items {
		width := chromeWidth + lipgloss.Width(item.PrimaryMarker) + lipgloss.Width(item.PrimaryText)
		secondary := item.SecondaryText
		if !item.Enabled() {
			secondary = strings.TrimPrefix(secondary+" · disabled: "+item.DisabledReason, " · ")
		}
		if secondary != "" {
			width += lipgloss.Width(ListTextGap) + lipgloss.Width(secondary)
		}
		widest = max(widest, width)
	}
	return widest
}

func RenderListSurfaceRow(
	row ListSurfaceRow,
	selectionMode ListSelectionMode,
	width int,
) string {
	if row.Kind == ListRowCategory {
		label := strings.ToUpper(row.Category.Label)
		return theme.SectionHeaderStyle.Render(textfit.TruncateToVisualWidth(label, width))
	}

	item := row.Match.Item
	cursor := "  "
	if row.Cursor {
		cursor = "› "
	}
	marker := cursor
	switch selectionMode {
	case ListSelectionMark:
		// The mark takes the cursor column rather than one of its own, as the
		// Outliner's does, so selecting moves no text; the cursor row still reads
		// as the cursor because SelectedLine owns its whole line.
		if row.Selected {
			marker = selectionMark + " "
		}
	case ListSelectionMulti:
		checkbox := "[ ] "
		if item.Indeterminate {
			checkbox = "[-] "
		} else if row.Selected {
			checkbox = "[x] "
		}
		marker += checkbox
	case ListSelectionSingle:
	}
	// PrimaryMarker is charged to markerWidth rather than to the primary text, so
	// LayoutListText budgets around chrome it never sees and the marker survives
	// the truncation the text takes.
	markerWidth := lipgloss.Width(marker) + lipgloss.Width(item.PrimaryMarker)
	secondary := item.SecondaryText
	if !item.Enabled() {
		disabled := "disabled"
		if item.DisabledReason != "" {
			disabled += ": " + item.DisabledReason
		}
		if secondary == "" {
			secondary = disabled
		} else {
			secondary += " · " + disabled
		}
	}
	layout := LayoutListText(item.PrimaryText, secondary, max(width-markerWidth, 0))
	if item.Enabled() && (len(item.PrimaryMatchPositions) > 0 || len(item.SecondaryMatchPositions) > 0) {
		return renderMatchedListSurfaceRow(row, marker, layout, width)
	}
	// A checked row keeps its color because "applied" is the state worth
	// previewing; the [x] still says it is applied. The cursor row does not,
	// because SelectedLine owns the whole line on every list surface, and a
	// disabled row must not advertise a color it cannot deliver. The Outliner
	// cursor row is the deliberate exception: it has structural margins to paint
	// instead, which a flat list row does not.
	if item.PrimaryStyle != nil && item.Enabled() && !row.Cursor {
		chrome := func(text string) string { return text }
		if row.Selected {
			chrome = func(text string) string { return theme.SelectionStyle.Render(text) }
		}
		styled := chrome(marker) + item.PrimaryMarker + item.PrimaryStyle.Render(layout.Primary)
		if layout.Secondary != "" {
			if row.Selected {
				styled += chrome(layout.Gap + layout.Secondary)
			} else {
				styled += layout.Gap + paintSecondaryAccent(item, layout.Secondary)
			}
		}
		return textfit.TruncateToVisualWidth(styled, width)
	}
	line := marker + item.PrimaryMarker + layout.Text()
	if item.Enabled() && !row.Selected && !row.Cursor && layout.Secondary != "" {
		line = marker + item.PrimaryMarker + layout.Primary + layout.Gap + paintSecondaryAccent(item, layout.Secondary)
	}
	if !item.Enabled() {
		line = theme.MutedStyle.Render(line)
	}
	if row.Selected && !row.Cursor {
		line = theme.SelectionStyle.Render(line)
	}
	if row.Cursor {
		line = chrome.SelectedLine(line, width)
	}
	return textfit.TruncateToVisualWidth(line, width)
}

// paintSecondaryAccent paints the item's accent run inside the secondary text
// the layout kept. A run the truncation cut is left plain, because a clipped
// count painted as a warning reads as a different number.
func paintSecondaryAccent(item ListItem, secondary string) string {
	if item.SecondaryAccent == "" || item.SecondaryAccentStyle == nil {
		return secondary
	}
	index := strings.Index(secondary, item.SecondaryAccent)
	if index < 0 {
		return secondary
	}
	end := index + len(item.SecondaryAccent)
	return secondary[:index] + item.SecondaryAccentStyle.Render(item.SecondaryAccent) + secondary[end:]
}

func renderMatchedListSurfaceRow(row ListSurfaceRow, marker string, layout ListTextLayout, width int) string {
	item := row.Match.Item
	base, match := lipgloss.NewStyle(), theme.HighlightStyle
	if row.Selected {
		base = theme.SelectionStyle
	}
	primary := base
	if item.PrimaryStyle != nil {
		primary = *item.PrimaryStyle
	}
	marker += item.PrimaryMarker
	if row.Cursor {
		base, primary = theme.SelectedStyle, theme.SelectedStyle
		match = theme.SelectedStyle.Underline(true)
		marker = ansi.Strip(marker)
	}
	line := base.Render(marker) + highlightListField(layout.Primary, item.PrimaryText, item.PrimaryMatchPositions, primary, match)
	if layout.Secondary != "" {
		line += base.Render(layout.Gap) + highlightListField(layout.Secondary, item.SecondaryText, item.SecondaryMatchPositions, base, match)
	}
	if row.Cursor {
		line = theme.SelectedStyle.Width(width).Render(line)
	}
	return textfit.TruncateToVisualWidth(line, width)
}

// A truncation's ellipsis is chrome, not the character it displaced. Keep only
// positions in the unchanged prefix, with rune offsets even for wide text.
func highlightListField(shown, original string, positions []int, base, match lipgloss.Style) string {
	visible, source := []rune(shown), []rune(original)
	prefix := 0
	for prefix < len(visible) && prefix < len(source) && visible[prefix] == source[prefix] {
		prefix++
	}
	kept := make([]int, 0, len(positions))
	for _, position := range positions {
		if position >= 0 && position < prefix {
			kept = append(kept, position)
		}
	}
	return textfit.HighlightTextWithStyle(shown, kept, base, match)
}
