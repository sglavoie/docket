package outliner

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// A folded section hides its members' badges, so its Peek names the winning
// attention signal using the same admitted task pointers as the header band.
func inlinePeekSectionDocumentFor(row OutlinerRow, attention, indent string, width int, window peekDockGeometry) inlinePeekDocument {
	if !row.IsSectionHeader || width <= 0 {
		return inlinePeekDocument{}
	}
	available := max(width-lipgloss.Width(indent), 1)
	title := fmt.Sprintf("%s (%d)", row.SectionName, row.SectionCount)
	sections := []inlinePeekSection{
		{key: "section:" + row.EffectiveCollapseKey(), lines: inlinePeekStyledLines("", theme.NormalStyle.Bold(true).Render(title), indent, available)},
	}
	if attention == "" {
		attention = "None"
	}
	sections = append(sections, inlinePeekSection{
		key: "attention:" + attention, lines: inlinePeekEntryLines("Attention: ", []string{attention}, indent, available),
	})
	body := make([]string, 0, len(sections))
	keys := make([]string, 0, len(sections))
	for _, section := range sections {
		body = append(body, section.lines...)
		keys = append(keys, section.key)
	}
	return inlinePeekDocument{
		Body: body, sections: sections, Fingerprint: strings.Join(keys, "\x1f"),
		indent: indent, width: available, window: window,
	}
}
