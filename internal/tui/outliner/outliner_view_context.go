package outliner

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

func (m *OutlinerModel) outlinerScopeLabel() string {
	if m.Project == nil {
		if m.IsProjectScoped() {
			return m.scopedProjectChromeLabel(m.Scope.ProjectID)
		}
		return m.baseScopeChromeLabel()
	}
	return m.Project.Name
}

// effectiveSort is the ordering the rows on screen actually follow. A
// fixed-order mode owns its ordering, so a sort recorded there is not on screen
// and must not be advertised. Under a search projection the mode underneath is
// irrelevant — the sort is live over the ranked rows — so zeroing it there would
// silently hide a live axis.
//
// It is one definition because the context row and the readout both state the
// sort, and a view where the row and the readout disagreed about what the rows
// are ordered by would be worse than either saying nothing.
func (m *OutlinerModel) effectiveSort() (query.SortField, bool) {
	if m.ActiveOutlinerModeEntry().FixedOrder() && !m.ShowsRankedList() {
		return query.SortByDefault, false
	}
	return m.Sort.Field, m.Sort.Ascending
}

// ViewContextEntry is one live axis of interaction context, described once so
// that the compact context row and the expanded readout state the same thing
// without either reading the other's rendered text. Row and RowCompact are the
// two spellings the row may draw, already styled; Label and Value are the plain
// pair the readout lists, whole.
//
// ReadOnly marks an axis the readout reports but no peel digit removes, because
// it narrows nothing: a selection, an ordering, a Focus, and a section zoom are
// all things the view is doing rather than clauses over the population. Those
// four are exactly what the readout adds to its peelable stack, which is why
// they are described here and not beside it.
type ViewContextEntry struct {
	Label      string
	Value      string
	Row        string
	RowCompact string
	ReadOnly   bool
}

// BuildViewContextEntries describes every axis of interaction context that is
// live right now, in the order the context row draws them: the selection, the
// lens naming the context the segments after it describe, the query, the
// filters, the ordering, and last the two narrowings a single Esc leaves. The
// row drops from the right, so that order is also the order it gives them up
// in, and the zoom goes before a narrowing the user cannot leave with one Esc.
//
// Scope and mode are deliberately absent: they are identity rather than
// context, they stay in the title above, and the readout already peels them
// from its own stack.
//
// The set is a question about state alone. Width chooses between a full and a
// compact search badge; it never adds or removes an entry, which is what lets
// contextRowRows answer the height question without one.
func (m *OutlinerModel) BuildViewContextEntries(width int) []ViewContextEntry {
	entries := make([]ViewContextEntry, 0, 7)
	if m.MultiSelectMode {
		entries = append(entries, m.selectionContextEntry())
	}
	if lens := m.ActiveLensLabel(); lens != "" {
		entries = append(entries, ViewContextEntry{
			Label: "lens",
			Value: lens,
			Row:   theme.MutedStyle.Render(lens),
		})
	}
	if search := m.buildOutlinerSearchIndicator(width); search != "" {
		// The plain half is read off the same live state the indicator is built
		// from rather than stripped back out of it, so the readout never depends on
		// how the row happened to style or badge the query.
		label, value := "query", m.SearchQuery()
		if m.HasActiveLens() && m.Lens.Query != "" {
			label, value = "lens query", m.Lens.Query
		}
		entries = append(entries, ViewContextEntry{
			Label:      label,
			Value:      value,
			Row:        search,
			RowCompact: m.searchIndicator(width, false),
		})
	}
	if parts := m.ActiveFilterParts(); len(parts) > 0 {
		row := query.RenderFilterBadge(parts)
		if m.ShowsRankedList() {
			row = theme.MutedStyle.Render(util.Count(len(parts), "filter") + ": " + strings.Join(parts, " "))
		}
		entries = append(entries, ViewContextEntry{
			Label:      "filters",
			Value:      strings.Join(parts, " "),
			Row:        row,
			RowCompact: pulse.OperationalTitleBadge(theme.FilterBadgeStyle, "["+util.Count(len(parts), "filter")+"]"),
		})
	}
	if field, ascending := m.effectiveSort(); field != query.SortByDefault {
		direction := "↓"
		if ascending {
			direction = "↑"
		}
		value := fmt.Sprintf("%s %s", field, direction)
		entries = append(entries, ViewContextEntry{
			Label:    "sort",
			Value:    value,
			Row:      theme.MutedStyle.Render("Sort: " + value),
			ReadOnly: true,
		})
	}
	if focus := m.outlinerFocusContext(); focus != "" {
		entries = append(entries, ViewContextEntry{
			Label:    "focus",
			Value:    strings.TrimPrefix(focus, outlinerFocusContextPrefix),
			Row:      theme.MutedStyle.Render(focus),
			ReadOnly: true,
		})
	}
	if m.Focus.Zoom.Active() && m.Focus.Zoom.Name != "" {
		entries = append(entries, ViewContextEntry{
			Label:    "section",
			Value:    m.Focus.Zoom.Name,
			Row:      theme.MutedStyle.Render("Section: " + m.Focus.Zoom.Name),
			ReadOnly: true,
		})
	}
	if m.ShowsRankedList() {
		population := "Open tasks"
		if m.Search.Gate.AllowsTerminal() {
			population = "Includes completed"
		}
		if m.Search.Gate.AllowsDeferred() {
			population += " · includes deferred"
		} else {
			population += " · not deferred"
		}
		entries = append(entries, ViewContextEntry{
			Label: "search population", Value: population,
			Row:        theme.MutedStyle.Render(population),
			RowCompact: theme.MutedStyle.Render("Q population details"), ReadOnly: true,
		})
	}
	return entries
}

// selectionContextEntry states the whole selection, hidden members included. It
// is the only persistent statement of a live selection: the status channel says
// nothing about one, so whatever this entry sheds is shed everywhere.
//
// That is why the hidden count is not what the compact form gives up. A count
// that silently covered rows off screen is the one thing a bulk gesture must not
// be surprised by, so both spellings carry it whenever it is nonzero. The two
// spellings are therefore identical: the entry has nothing it may shorten, so
// what a narrow row gives up is decided by the row's own fitting alone.
//
// `Q` keeps the whole breakdown on the same terms, and on those terms only: with
// nothing hidden the visible half is the total restated, and a readout row
// costing two wrapped lines to say a number twice is a row the document cannot
// spend. The breakdown appears exactly where there is a difference to read.
func (m *OutlinerModel) selectionContextEntry() ViewContextEntry {
	counts := m.SelectionCounts()
	value := fmt.Sprintf("%d selected", counts.Total)
	full := fmt.Sprintf("Multi-select: %d selected", counts.Total)
	compact := full
	if counts.Hidden > 0 {
		value = fmt.Sprintf("%d total · %d visible · %d hidden", counts.Total, counts.Visible, counts.Hidden)
		hidden := fmt.Sprintf(" (%d hidden)", counts.Hidden)
		full += hidden
		compact += hidden
	}
	return ViewContextEntry{
		Label:      "selection",
		Value:      value,
		Row:        theme.SelectionCountStyle.Render(full),
		RowCompact: theme.SelectionCountStyle.Render(compact),
		ReadOnly:   true,
	}
}

// BuildOutlinerContextRow draws the optional second header row, or nothing at
// all. It exists only while some axis is live: a row standing over a default
// view would spend an outline row every frame to say that nothing is narrowed,
// which is the row the readout already answers on demand.
func (m *OutlinerModel) BuildOutlinerContextRow(width int) string {
	width = max(width, 1)
	entries := m.BuildViewContextEntries(width)
	if len(entries) == 0 {
		return ""
	}
	segments := make([]pulse.ContextRowSegment, 0, len(entries))
	query := -1
	for index, entry := range entries {
		if entry.Label == "query" && entry.RowCompact != entry.Row {
			query = index
		}
		segments = append(segments, pulse.ContextRowSegment{Full: entry.Row, Compact: entry.RowCompact})
	}
	row := pulse.RenderContextRow(segments, width, m.queryReadoutDetailsHint())
	if query < 0 || strings.Contains(row, segments[query].Full) {
		return row
	}
	// The match count is the first thing the row gives up: the search entry
	// steps down to its uncounted form before any other entry compacts or drops,
	// so a count never costs the row an axis. Q still states it.
	segments[query].Full = segments[query].Compact
	return pulse.RenderContextRow(segments, width, m.queryReadoutDetailsHint())
}

// contextRowRows is the row the header spends on context, counted in one place
// so every path that measures the header charges it exactly once. It reads the
// model's own width because entry presence is a question about state and the
// count does not move with the terminal.
func (m *OutlinerModel) contextRowRows() int {
	if len(m.BuildViewContextEntries(max(m.Width(), 1))) == 0 {
		return 0
	}
	return 1
}

// buildOutlinerTitleLine assembles the Outliner's one-row operational title. It
// carries identity and passive state alone — scope, mode, Today pace, and the
// workspace pulse — because interaction context moved to the row below, where a
// value has room to keep its label.
func (m *OutlinerModel) BuildOutlinerTitleLine(width int) string {
	// Read once into a local: the spec takes the pulse by pointer, and the reader
	// is the promoted accessor rather than a field, so there is no address to
	// take without naming the value first.
	workspacePulse := m.CurrentWorkspacePulse()
	var pace *pulse.TodayPaceReadout
	if m.PrimaryMode == chrome.ModeToday && m.CompletionActivity.HasHistory() {
		pace = &pulse.TodayPaceReadout{
			OpenCount:    workspacePulse.Today,
			MedianPerDay: m.CompletionActivity.MedianPerDay(),
		}
	}
	spec := pulse.OperationalTitleSpec{
		Scope: theme.TitleStyle.Render(m.outlinerScopeLabel()),
		Mode:  chrome.OperationalTitleModeSegment(m.ActiveOutlinerModeEntry()),
		Pace:  pace,
		Pulse: &workspacePulse,
	}
	if m.ShowsRankedList() {
		// The mode underneath search is a return destination, not a limit on
		// these results. Say so even before the first character is typed.
		spec.Mode = theme.MutedStyle.Render("Search · all modes")
		spec.Pace = nil
		return pulse.RenderOperationalTitle(spec, max(width, 1))
	}
	// A wide title spends its blank middle on the active mode's Tab neighbors,
	// so the reader sees where Tab and Shift+Tab go. It is offered only where the whole title still
	// fits unshortened; anywhere narrower the lone active badge stands.
	strip := spec
	strip.Mode = chrome.OperationalTitleModeStrip(m.ActiveOutlinerMode())
	if pulse.OperationalTitleFitsWhole(strip, width) {
		spec = strip
	}
	return pulse.RenderOperationalTitle(spec, max(width, 1))
}

// writeOutlinerHeader is the one place the header's rows are written, so both
// projections draw the same one or two lines and contextRowRows charges for the
// second exactly once.
func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int) {
	b.WriteString(m.BuildOutlinerTitleLine(width))
	b.WriteString("\n")
	if row := m.BuildOutlinerContextRow(width); row != "" {
		b.WriteString(row)
		b.WriteString("\n")
	}
}
