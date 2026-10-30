package help

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// Search results use one column so their cursor and up/down order agree at
// every width. Reference help keeps its grouped, two-column layout.
func (m *HelpViewModel) actionSearchDocument() (string, []int) {
	if len(m.filteredItems) == 0 {
		return m.renderBySectionContent(), nil
	}
	width := max(m.viewport.Width(), 1)
	var b strings.Builder
	starts := make([]int, 0, len(m.filteredItems))
	row := 0
	for index, item := range m.filteredItems {
		starts = append(starts, row)
		key := textfit.HighlightText(item.Key, item.keyPositions, theme.TagInlineStyle)
		description := textfit.HighlightText(item.Description, item.descPositions, theme.NormalStyle)
		if len(item.matchedAliases) > 0 {
			description += theme.MutedStyle.Render(" · " + strings.Join(item.matchedAliases, ", "))
		}
		lines := textfit.WrapStyled(key+"  "+description, max(width-2, 1))
		for i, line := range lines {
			marker := "  "
			if index == m.actionCursor && i == 0 {
				marker = "› "
			}
			line = marker + line
			if item.DisabledReason != "" || item.Action == nil {
				line = theme.MutedStyle.Render(line)
			}
			if index == m.actionCursor {
				line = chrome.SelectedLine(line, width)
			}
			b.WriteString(line)
			b.WriteString("\n")
			row++
		}
	}
	return b.String(), starts
}

func (m *HelpViewModel) renderActionContext() string {
	item, ok := m.selectedActionItem()
	if !ok {
		return ""
	}
	width := max(m.innerWidth(), 1)
	text := "Reference only: use this syntax in capture, edit, or search."
	if item.Action != nil {
		text = textfit.TruncateIntentional("Target: "+item.Target, width, "…")
		if item.DisabledReason != "" {
			text += "\n" + item.DisabledReason
		}
	}
	// Target names can be long; keep the action's reason and controls visible.
	// The source Outliner and the ordinary picker retain the full task title.
	lines := textfit.WrapStyled(text, width)
	for i := range lines {
		lines[i] = " " + theme.MutedStyle.Render(lines[i])
	}
	return strings.Join(lines, "\n") + "\n"
}

func (m *HelpViewModel) actionGuidance() chrome.GuidanceSpec {
	spec, _ := chrome.GuidanceSpecFromKeyMap(m.buildKeyMap(), 2)
	spec.Supplemental = fmt.Sprintf("%d results", len(m.filteredItems))
	return spec
}
