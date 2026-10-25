package outliner

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

// statusStripBody builds the outstanding-conflict indicator, the focus notice,
// the status message, and the expanded query readout the shell pins above the
// footer. It is pure: the height reservation and the strip itself must measure
// the same string.
//
// The indicator is an ambient advisory, so it is gated by advisoryLine: hiding
// it drops the row it reserved too, because the reservation measures this same
// body.
//
// The indicator leads because it is the one durable line here — it stands until
// the CLI resolves the last bundle, while the lines below it come and go — so a
// transient line arriving never shifts it to another row. The readout sits last
// for the same reason from the other end: it is the tallest thing the strip can
// hold, and growing it must not push the three one-line signals around.
func (m *OutlinerModel) StatusStripBody(width int) string {
	lines := m.statusStripSignals(width)
	// The expanded stack changes this return value rather than painting outside
	// it, so statusStripHeight reserves the rows it needs from the list.
	if readout := m.QueryReadoutBody(width); readout != "" {
		lines = append(lines, readout)
	}
	return strings.Join(lines, "\n")
}

// statusStripSignals is the strip without the readout: the three one-line
// signals that stand above it. They are separated out because the readout has
// to budget its own viewport against them, and it cannot ask statusStripHeight
// for that — the height it would answer with is the one the readout is still
// deciding.
func (m *OutlinerModel) statusStripSignals(width int) []string {
	width = max(width, 1)
	lines := make([]string, 0, 4)
	if indicator := m.AdvisoryLine(m.conflictIndicator()); indicator != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(indicator))
	}
	if indicator := m.AdvisoryLine(m.syncIndicator()); indicator != "" {
		lines = append(lines, theme.MutedStyle.Width(width).Render(indicator))
	}
	if m.Focus.Notice != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(m.Focus.Notice))
	}
	if m.StatusMessage != "" {
		lines = append(lines, theme.MutedStyle.Width(width).Render(m.StatusMessage))
	}
	if m.HasContentRows() {
		if hint := m.withheldSearchHint(); hint != "" {
			lines = append(lines, theme.MutedStyle.Width(width).Render("Also matches: "+hint+"."))
		}
	}
	return lines
}

// statusStripSignalRows is what those signals cost once wrapped, which is not
// their count: each is rendered at the strip's width and a long one takes more
// than a row.
func (m *OutlinerModel) statusStripSignalRows(width int) int {
	signals := m.statusStripSignals(width)
	if len(signals) == 0 {
		return 0
	}
	return lipgloss.Height(strings.Join(signals, "\n"))
}

// statusStripHeight is the strip's divider plus its wrapped lines. The Outliner
// reserves it from its own row budget, so the strip takes rows from the list
// rather than pushing the footer off screen.
func (m *OutlinerModel) StatusStripHeight(width int) int {
	body := m.StatusStripBody(width)
	if body == "" {
		return 0
	}
	return 1 + lipgloss.Height(body)
}

// ViewStatusStrip hands the strip to the shell and consumes the one-shot focus
// notice. AppModel.View renders the view before the status bar, so the
// reservation above and this call see the same inputs in the same frame.
func (m *OutlinerModel) ViewStatusStrip(width int) string {
	body := m.StatusStripBody(width)
	m.Focus.clearNotice()
	return body
}

func (m *OutlinerModel) outlinerFocusContext() string {
	if m.Project == nil {
		return m.CrossProjectFocusContext()
	}
	return m.ProjectFocusContext()
}

// buildOutlinerSearchIndicator renders the search chrome, appending a badge only
// when the title row can hold it: at narrow widths the query itself is the more
// useful half of the segment, so the match count sheds before it does. A refused
// sigil wins over the count and the case badge, because a query that cannot run
// explains an empty list better than how it was matched.
func (m *OutlinerModel) buildOutlinerSearchIndicator(width int) string {
	return m.searchIndicator(width, true)
}

// searchIndicator is that chrome with or without the match count. The context
// row draws the counted form in full and the uncounted one compact, so a row
// too narrow for every entry gives up the count before it drops the query.
func (m *OutlinerModel) searchIndicator(width int, counted bool) string {
	if m.IsSearchMode() && m.Search.historyPicker != nil {
		picker := m.Search.historyPicker
		picker.input.SetWidth(max(width-10, 1))
		return theme.MutedStyle.Render("History: ") + picker.input.View()
	}
	var indicator string
	switch {
	case m.HasActiveLens() && m.Lens.Query != "":
		return m.appendLensQueryBadge(theme.MutedStyle.Render(fmt.Sprintf("Lens query: %q", m.Lens.Query)), width)
	case m.IsSearchMode():
		indicator = theme.MutedStyle.Render("Search: ") + m.SearchInputView()
	case m.SearchQuery() != "":
		indicator = theme.MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	default:
		return ""
	}
	if guidance := m.Search.Gate.GuidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if count := m.searchMatchCountLabel(); counted && count != "" {
		indicator = appendSearchBadge(indicator, " · "+count, width)
	}
	if m.Search.caseRelaxed {
		indicator = appendSearchBadge(indicator, " (case ignored)", width)
	}
	if m.IsSearchMode() && !m.Search.Autocomplete.Active() && m.Search.history.notice != "" {
		indicator = appendSearchBadge(indicator, " · "+m.Search.history.notice, width)
	}
	return indicator
}

// searchMatchCountLabel spells how many task and note rows the projection
// draws for the held query, the same in the typing and the applied form. It is
// empty with no projection standing, since a query installed without one has
// drawn nothing of its own to count, and over an empty query, which matched
// nothing and narrowed nothing.
func (m *OutlinerModel) searchMatchCountLabel() string {
	if m.Search.flight != nil && m.Search.flight.result == nil {
		return "Searching…"
	}
	if !m.HasSearchProjection() || strings.TrimSpace(m.SearchQuery()) == "" {
		return ""
	}
	return util.CountPlural(m.Search.drawn, "match", "matches")
}

// appendLensQueryBadge gives the lens query the badges ordinary search already
// uses, read off the lens compilation: a refused sigil wins over the case badge
// there for the same reason it does in search.
func (m *OutlinerModel) appendLensQueryBadge(indicator string, width int) string {
	if guidance := m.LensGuidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if !m.Lens.NarrowRelaxed {
		return indicator
	}
	return appendSearchBadge(indicator, " (case ignored)", width)
}

func appendSearchBadge(indicator, text string, width int) string {
	badge := theme.MutedStyle.Render(text)
	if lipgloss.Width(indicator)+lipgloss.Width(badge) > width {
		return indicator
	}
	return indicator + badge
}
