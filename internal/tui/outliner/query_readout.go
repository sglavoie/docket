package outliner

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// The query readout answers one question: what is narrowing this view, and how
// do I take one narrowing away.
//
// Collapsed it adds nothing. The operational title already carries the lens
// name, the scope label, the search query, and the filter chips, so a second
// collapsed line would restate them a row lower; the readout is the expansion of
// what is already on screen rather than a rival for it.
//
// Expanded it lists one row per independently removable narrowing, read off live
// state every frame. Nothing here is remembered between frames and nothing is
// inferred: a mode's implied clauses are not spelled, because the mode catalog
// declares none and the readout must not name what it cannot prove.

// queryReadoutAxis names which narrowing one row stands for. It decides the peel
// and nothing else: the label, the value, and the ordering are all built beside
// the row.
type queryReadoutAxis uint8

const (
	queryReadoutAxisLens queryReadoutAxis = iota
	queryReadoutAxisScope
	QueryReadoutAxisMode
	queryReadoutAxisQuery
	queryReadoutAxisLensQuery
	queryReadoutAxisClauses
)

// queryReadoutRowLimit is how many rows carry a peel digit. The digits are the
// peel vocabulary, so the limit is the digit set rather than a display budget,
// and anything past it is named on an overflow line instead of being dropped
// silently.
const queryReadoutRowLimit = 9

// queryReadoutPeelAllLimit bounds the peel-all loop. Every peel removes at least
// the row it was aimed at, so the loop terminates well inside this; the cap is
// there so a future axis that fails to clear itself cannot spin.
const queryReadoutPeelAllLimit = 32

const (
	// queryReadoutChromeRows is what the readout costs before a single document
	// line is drawn: the strip's divider, the readout's own heading, and the one
	// outline row it may never take.
	queryReadoutChromeRows = 3
	// queryReadoutMinBodyRows is the floor under that budget. At a terminal small
	// enough to reach it the frame is already over its own minimum, and a readout
	// showing one line is still the answer to the key that opened it.
	queryReadoutMinBodyRows = 1
)

// queryReadoutRow is one peelable narrowing. fields is set only for the clause
// rows, where the peel is a write of that facet's own query fields.
type queryReadoutRow struct {
	Axis   queryReadoutAxis
	Label  string
	value  string
	note   string
	fields []models.QueryField
}

// toggleQueryReadout opens or closes the expanded stack. It is the whole of the
// operation's handler: the strip reads the sub-state when it next renders, so
// there is no second copy of the readout to keep in step.
func (m *OutlinerModel) ToggleQueryReadout() {
	if m == nil {
		return
	}
	m.QueryReadout.toggle()
}

// closeQueryReadout puts the stack down without asking what it was showing. The
// resets that replace the whole projection call it, so a readout cannot survive
// into a view whose axes it was never built from.
func (m *OutlinerModel) closeQueryReadout() {
	if m == nil {
		return
	}
	m.QueryReadout.close()
}

// queryReadoutRows lists the active narrowings, widest context first. The order
// is the one the operational title already reads left to right — lens, scope,
// mode, query, then the facets — so a row's position in the stack matches where
// the same narrowing is named in the header above it.
func (m *OutlinerModel) QueryReadoutRows() []queryReadoutRow {
	if m == nil {
		return nil
	}
	rows := make([]queryReadoutRow, 0, 8)
	if m.HasActiveLens() {
		value := m.Lens.Active.Name
		if m.LensModified() {
			value += " (modified)"
		}
		rows = append(rows, queryReadoutRow{
			Axis:  queryReadoutAxisLens,
			Label: "lens",
			value: value,
			note:  "restores the view it opened on",
		})
	}
	if m.IsProjectScoped() {
		rows = append(rows, queryReadoutRow{
			Axis:  queryReadoutAxisScope,
			Label: "scope",
			value: strings.Join(m.scopeChainNames(), " / "),
		})
	}
	// All is the unnarrowed mode, so it is the absence of a row rather than a row
	// saying nothing is narrowed.
	if mode := m.ActiveOutlinerModeEntry(); mode.ID != chrome.ModeAll {
		rows = append(rows, queryReadoutRow{
			Axis:  QueryReadoutAxisMode,
			Label: "mode",
			value: mode.Label,
			note:  "peels to All",
		})
	}
	// A lens's narrowing query and an ordinary search projection are two different
	// axes that can never both be live: slash search under a lens suspends the
	// lens outright, so while the projection stands there is no lens to carry a
	// query of its own. That suspension is why the query row names its own peel:
	// removing this query hands the lens back, and a row promising only that a
	// search goes away would be describing half the act.
	switch {
	case m.HasActiveLens() && m.Lens.Query != "":
		rows = append(rows, queryReadoutRow{
			Axis:  queryReadoutAxisLensQuery,
			Label: "lens query",
			value: m.Lens.Query,
		})
	case m.SearchQuery() != "":
		row := queryReadoutRow{
			Axis:  queryReadoutAxisQuery,
			Label: "query",
			value: m.SearchQuery(),
		}
		if m.HasSuspendedLens() {
			row.note = "restores the suspended lens"
		}
		rows = append(rows, row)
	}
	rows = append(rows, commonFilterReadoutRows(&m.Filter.CommonFilterState)...)
	return append(rows, projectFilterReadoutRows(&m.ProjectFilter)...)
}

// clauseReadoutRow builds one facet row from the clauses that facet currently
// spells. The value is FilterClauseParts output, so a readout row, a header
// chip, and a typed query name the same narrowing with the same characters.
func clauseReadoutRow(
	label string,
	fields []models.QueryField,
	clauses []models.QueryClause,
	note string,
) queryReadoutRow {
	return queryReadoutRow{
		Axis:   queryReadoutAxisClauses,
		Label:  label,
		value:  strings.Join(query.FilterClauseParts(clauses), " "),
		note:   note,
		fields: fields,
	}
}

// commonFilterReadoutRows splits the row cache into one row per independently
// clearable facet. The split is CommonFilterFacetCount's, promoted from a count
// into the peel unit rather than invented beside it, which is why the completion
// age facet carries the terminal-state clauses it installed and the state facet
// steps aside when it does.
//
//nolint:gocyclo // One row per facet, kept together so the set stays auditable.
func commonFilterReadoutRows(f *query.CommonFilterState) []queryReadoutRow {
	if f == nil {
		return nil
	}
	rows := make([]queryReadoutRow, 0, 10)
	if f.HasStateFilter() && !query.CompletionAgeOwnsTerminalStates(f) {
		clauses := make([]models.QueryClause, 0, 4)
		for _, state := range f.States.Values() {
			clauses = query.AppendFilterClause(clauses, models.QueryFieldState,
				models.QueryOperatorIs, string(state))
		}
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldState}, clauses, ""))
	}
	if f.HasCompletionAgeFilter() {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldCompleted, models.QueryFieldState},
			query.CompletionAgeClauses(f.CompletionAge), ""))
	}
	if f.Priority > 0 {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldPriority},
			query.FilterClauseList(models.QueryFieldPriority, models.QueryOperatorAtLeast,
				strconv.Itoa(f.Priority)), ""))
	}
	if len(f.Tags) > 0 {
		clauses := make([]models.QueryClause, 0, len(f.Tags))
		for _, tag := range slices.Sorted(maps.Keys(f.Tags)) {
			clauses = query.AppendFilterClause(clauses, models.QueryFieldTag,
				models.QueryOperatorIs, tag)
		}
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldTag}, clauses, ""))
	}
	if len(f.Contexts) > 0 || f.NoContext {
		names := slices.Sorted(maps.Keys(f.Contexts))
		if f.NoContext {
			names = append(names, query.NoContextFilterKey)
		}
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldContext},
			query.ContextFilterClauses(names), ""))
	}
	if f.DueDate != query.DueDateAll {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldDue}, query.DueFilterClauses(f.DueDate), ""))
	}
	if f.Hierarchy != query.HierarchyAll {
		rows = append(rows, clauseReadoutRow("filter",
			query.FilterQueryHierarchyFields, query.HierarchyFilterClauses(f.Hierarchy), ""))
	}
	if f.Dependency != query.DependencyFilterAll {
		rows = append(rows, clauseReadoutRow("filter",
			query.FilterQueryDependencyFields, query.DependencyFilterClauses(f.Dependency), ""))
	}
	if f.Stale {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldStale},
			query.FilterClauseList(models.QueryFieldStale, models.QueryOperatorPresent, ""), ""))
	}
	if f.Deferred {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldDeferred},
			query.FilterClauseList(models.QueryFieldDeferred, models.QueryOperatorPresent, ""), ""))
	}
	if f.Pinned {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldPinned},
			query.FilterClauseList(models.QueryFieldPinned, models.QueryOperatorPresent, ""), ""))
	}
	if f.Review {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldReview},
			query.FilterClauseList(models.QueryFieldReview, models.QueryOperatorPresent, ""), ""))
	}
	return rows
}

// projectFilterReadoutRows does the same for the population half. Each row says
// "reload" because peeling it reloads the projects rather than rebuilding the
// rows, which is the one thing a population facet does that a row facet does not.
func projectFilterReadoutRows(f *query.ProjectFilterState) []queryReadoutRow {
	if f == nil {
		return nil
	}
	rows := make([]queryReadoutRow, 0, 3)
	if tags := query.ProjectTagClauses(f); len(tags) > 0 {
		rows = append(rows, clauseReadoutRow("population",
			[]models.QueryField{models.QueryFieldProjectTag}, tags, "reload"))
	}
	if clauses := query.PopulationClauses(models.QueryFieldParked, f.ParkedMode); len(clauses) > 0 {
		rows = append(rows, clauseReadoutRow("population",
			[]models.QueryField{models.QueryFieldParked}, clauses, "reload"))
	}
	if clauses := query.PopulationClauses(models.QueryFieldArchived, f.ArchivedMode); len(clauses) > 0 {
		rows = append(rows, clauseReadoutRow("population",
			[]models.QueryField{models.QueryFieldArchived}, clauses, "reload"))
	}
	return rows
}

// queryReadoutBody renders the expanded stack, or the empty string when it is
// collapsed. It is pure, because statusStripHeight measures exactly this string
// before statusStripBody paints it: the window is computed from the stored
// offset rather than written back, the way a Peek render reads peekOffsetForRow
// without moving it.
func (m *OutlinerModel) QueryReadoutBody(width int) string {
	if m == nil || !m.QueryReadout.Expanded {
		return ""
	}
	width = max(width, 1)
	heading := theme.TitleStyle.Render(textfit.TruncateToVisualWidth("Narrowing this view", width))
	document, window, overflows := m.queryReadoutWindow(width)
	if !overflows {
		return strings.Join(append([]string{heading}, document...), "\n")
	}
	offset := clampQueryReadoutOffset(m.QueryReadout.Offset, len(document), window)
	lines := append([]string{heading}, document[offset:offset+window]...)
	return strings.Join(append(lines,
		queryReadoutPositionLine(offset, len(document)-offset-window, width)), "\n")
}

// queryReadoutWindow is the one measurement of the readout: the whole document,
// how many of its lines the window draws, and whether anything is off it.
// Render, every scroll write, and the footer all read it, so none of them can
// disagree about where the window ends.
func (m *OutlinerModel) queryReadoutWindow(width int) (document []string, window int, overflows bool) {
	document = m.queryReadoutDocument(width)
	window = m.queryReadoutBodyRows(width)
	if len(document) <= window {
		return document, window, false
	}
	// One row goes to the position line, which is what makes a window that hides
	// something say so instead of ending silently.
	return document, max(window-1, 1), true
}

// queryReadoutBodyRows is the readout's budget: the view's own rows, less what
// the footer overran, less the dock, less the strip's signals above it, less
// this readout's chrome, and less the one outline row the readout may never
// take — it is an expansion of the view, not a replacement for it.
//
// It cannot ask statusStripHeight for the strip's cost, because the height that
// call would answer with is the one this function is still deciding. For the
// same reason it charges the dock's frame-preferred height rather than the
// height the dock settles on: the settled height is constrained by this strip,
// so asking for it here would ask this function for its own answer. Preferred is
// never smaller than settled, so the charge is conservative in the one direction
// that cannot overflow the frame.
func (m *OutlinerModel) queryReadoutBodyRows(width int) int {
	dock := m.peekDockPreferredRows()
	reserved := m.ShellFrame.ExcessRows()
	if dock > 0 {
		reserved = m.peekDockFooterReservation()
	}
	budget := m.ContentHeight() - reserved - dock - m.statusStripSignalRows(width) - queryReadoutChromeRows
	return max(budget, queryReadoutMinBodyRows)
}

// queryReadoutDocument is the whole readout below its heading, wrapped to width
// and styled: the peelable stack, then the axes the readout reports but nothing
// peels. It is complete — a value is wrapped rather than cut — because this is
// where a context row that shortened something says what it shortened.
func (m *OutlinerModel) queryReadoutDocument(width int) []string {
	rows := m.QueryReadoutRows()
	lines := make([]string, 0, len(rows)+6)
	if len(rows) == 0 {
		lines = appendQueryReadoutLines(lines, theme.MutedStyle, width, "nothing: every axis is at its default")
	} else {
		labelWidth := 0
		for _, row := range rows {
			labelWidth = max(labelWidth, lipgloss.Width(row.Label))
		}
		for index, row := range rows {
			// The digits are the peel vocabulary rather than a numbering of what is
			// on screen, so they stay attached to the first nine axes wherever the
			// window sits, and a tenth axis is listed without one: scrolling reaches
			// it, and no new binding is minted to press it.
			marker := "  "
			if index < queryReadoutRowLimit {
				marker = strconv.Itoa(index+1) + " "
			}
			text := fmt.Sprintf("%s%-*s %s", marker, labelWidth, row.Label, row.value)
			if row.note != "" {
				text += "  (" + row.note + ")"
			}
			lines = appendQueryReadoutLines(lines, theme.MutedStyle, width, text)
		}
	}
	return append(lines, m.queryReadoutDetailLines(width)...)
}

// queryReadoutDetailLines lists the axes that are live but peel to nothing,
// read off the same structured description the context row draws from rather
// than out of the row's rendered text. They are last and they carry no digit,
// because a digit is a promise that pressing it takes the axis away.
func (m *OutlinerModel) queryReadoutDetailLines(width int) []string {
	entries := make([]ViewContextEntry, 0, 4)
	labelWidth := 0
	for _, entry := range m.BuildViewContextEntries(width) {
		if !entry.ReadOnly {
			continue
		}
		entries = append(entries, entry)
		labelWidth = max(labelWidth, lipgloss.Width(entry.Label))
	}
	if len(entries) == 0 {
		return nil
	}
	lines := appendQueryReadoutLines(nil, theme.TitleStyle, width, "Also in view")
	for _, entry := range entries {
		lines = appendQueryReadoutLines(lines, theme.MutedStyle, width,
			fmt.Sprintf("  %-*s %s", labelWidth, entry.Label, entry.Value))
	}
	return lines
}

// appendQueryReadoutLines wraps one entry to the readout's width and styles
// every line it became. Wrapping rather than truncating is the whole point of
// the bounded viewport: the readout is where a value the header shortened is
// readable in full.
func appendQueryReadoutLines(lines []string, style lipgloss.Style, width int, text string) []string {
	for _, wrapped := range textfit.WrapText(text, width) {
		lines = append(lines, style.Render(wrapped))
	}
	return lines
}

// queryReadoutPositionLine spends the reserved row on exact counts for the same
// reason Peek's does: what a reader needs is how much is still unread in each
// direction, and a page number only answers that after arithmetic.
func queryReadoutPositionLine(above, below, width int) string {
	return theme.MutedStyle.Render(textfit.TruncateToVisualWidth(
		fmt.Sprintf("%d above · %d below", above, below), width))
}

// clampQueryReadoutOffset is the one bound on the window's position. It is pure
// and it is applied at render as well as at every write, so a peel that
// shortened the document or a resize that shortened the window re-seats the
// window without anything having to notice that either happened.
func clampQueryReadoutOffset(offset, total, window int) int {
	return min(max(offset, 0), max(total-window, 0))
}

// scrollQueryReadout moves the window and nothing else. The outline's cursor,
// its own scroll offset, and the selection are all untouched by every key the
// readout claims, which is what lets it borrow j/k and the paging keys while it
// is open without the list moving underneath it.
//
// A relative move is measured from the line the reader can see, not from the
// stored offset, and those two part company whenever the window or the document
// grew since the last write: the render clamps without writing back, on purpose,
// so a taller terminal leaves an offset past the new maximum standing. Adding
// the delta to that stale number would spend the first k normalizing it and the
// screen would not move, and would shorten a PgUp by however far the offset
// overran. Clamping first is what makes every relative key travel its full
// distance from what is drawn.
func (m *OutlinerModel) scrollQueryReadout(delta int) {
	document, window, _ := m.queryReadoutWindow(max(m.Width(), 1))
	visible := clampQueryReadoutOffset(m.QueryReadout.Offset, len(document), window)
	m.QueryReadout.Offset = clampQueryReadoutOffset(visible+delta, len(document), window)
}

// scrollQueryReadoutTo seats the window at an absolute line, clamped. The two
// document bounds are spelled as offsets rather than as their own writes, so
// Home and End answer through the same clamp every other scroll takes.
func (m *OutlinerModel) scrollQueryReadoutTo(offset int) {
	document, window, _ := m.queryReadoutWindow(max(m.Width(), 1))
	m.QueryReadout.Offset = clampQueryReadoutOffset(offset, len(document), window)
}

// queryReadoutPageRows is one page of the readout: whatever the window is
// currently showing, so a page is always exactly what is on screen.
func (m *OutlinerModel) queryReadoutPageRows() int {
	_, window, _ := m.queryReadoutWindow(max(m.Width(), 1))
	return max(window, 1)
}

// queryReadoutOverflows is whether the footer should advertise the scroll keys.
// They are claimed whether or not the document moves — the readout owns input
// outright — but naming them over a document that fits would promise a scroll
// there is nothing to do.
func (m *OutlinerModel) queryReadoutOverflows() bool {
	_, _, overflows := m.queryReadoutWindow(max(m.Width(), 1))
	return overflows
}

// queryReadoutGuidance is the footer while the stack is open. It is the gated
// keymap a modal sub-state supplies, so the readout takes no footer rank of its
// own and the ranked projection stays untouched.
func (m *OutlinerModel) queryReadoutGuidance() chrome.GuidanceSpec {
	closeKey := m.queryReadoutKey()
	rows := m.QueryReadoutRows()
	overflows := m.queryReadoutOverflows()

	// The peel keys are named off the stack alone. An empty stack under a
	// document long enough to scroll is still an empty stack: the rows it is
	// showing are the read-only ones, and a digit over those would promise a peel
	// that cannot happen.
	var hints []chrome.GuidanceHint
	if len(rows) == 0 {
		if !overflows {
			// Two keys, one act, one entry — the same shape the populated readout
			// below already uses, so the empty stack does not spend a wider footer
			// than the full one saying less.
			return chrome.GuidanceSpec{
				Hints: chrome.RequiredGuidanceHints(closeKey + "/Esc close readout"),
				Rows:  1,
			}
		}
		hints = chrome.RequiredGuidanceHints(closeKey + "/Esc close readout")
	} else {
		peel := "1 peel one"
		if shown := min(len(rows), queryReadoutRowLimit); shown > 1 {
			peel = fmt.Sprintf("1-%d peel one", shown)
		}
		hints = append(chrome.RequiredGuidanceHints(peel, "x peel all", closeKey+"/Esc close"),
			chrome.OptionalGuidanceHints(10, "s save as lens")...)
	}
	if overflows {
		// Advertised only over a document that can move, for the same reason the
		// dock's paging hints are: the keys are claimed either way, and naming them
		// over a readout that fits would promise a scroll with nowhere to go.
		hints = append(hints, chrome.OptionalGuidanceHints(20,
			"j/k scroll", "PgUp/PgDn page", "Home/End ends")...)
	}
	return chrome.GuidanceSpec{Hints: hints, Rows: 2}
}

// queryReadoutKey is the key that opens and closes the readout, read off the
// registry so the footer, the context row's overflow hint, and the operation
// itself all name the same one.
func (m *OutlinerModel) queryReadoutKey() string {
	if m.OperationRegistry != nil {
		if key := m.OperationRegistry.InputShortcutForOperation(
			operation.InputStrategySimplified, operation.OperationCoreQueryReadout); key != "" {
			return key
		}
	}
	return "Q"
}

// queryReadoutDetailsHint is what the context row says where it had to shorten
// or drop a value. It is one phrase because the row has one place to send a
// reader, and that place is this readout.
func (m *OutlinerModel) queryReadoutDetailsHint() string {
	return m.queryReadoutKey() + " details"
}

// handleQueryReadoutKey is the readout's own gated keymap. The stack owns input
// while it is open, so an unlisted key closes it rather than falling through to
// a grammar whose footer is not on screen.
//
// The six scrolling keys move the readout's window and nothing else. They are
// consumed whether or not the document overflows, because a key that scrolled a
// short readout by falling through to the outline would move a cursor the
// reader cannot see.
func (m *OutlinerModel) HandleQueryReadoutKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.closeQueryReadout()
		return m, nil
	case "x":
		return m, m.peelAllQueryReadoutRows()
	case "s":
		// Saving is a shell act: the name prompt and the lens transaction both
		// live above this model, so the readout asks for it and closes.
		m.closeQueryReadout()
		return m, func() tea.Msg { return SaveViewAsLensRequestMsg{} }
	case "j", "down":
		m.scrollQueryReadout(1)
		return m, nil
	case "k", "up":
		m.scrollQueryReadout(-1)
		return m, nil
	case "pgdown":
		m.scrollQueryReadout(m.queryReadoutPageRows())
		return m, nil
	case "pgup":
		m.scrollQueryReadout(-m.queryReadoutPageRows())
		return m, nil
	case "home":
		m.scrollQueryReadoutTo(0)
		return m, nil
	case "end":
		m.scrollQueryReadoutTo(math.MaxInt32)
		return m, nil
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return m, m.PeelQueryReadoutRow(int(key[0] - '1'))
	}
	m.closeQueryReadout()
	return m, nil
}

// queryReadoutRowRequiresReload reports whether peeling a row changes which
// projects load rather than which rows survive. It reads the row's own fields
// through the same predicate the f chords apply, so the readout and the chord
// cannot disagree about which facet owes a reload.
func queryReadoutRowRequiresReload(row queryReadoutRow) bool {
	for _, field := range row.fields {
		if models.QueryFieldRequiresReload(field) {
			return true
		}
	}
	return false
}

// peelQueryReadoutRow removes exactly the narrowing at index. An index past the
// live stack is the idempotent case the contract names: nothing is written and
// the readout stays open reporting that there was nothing there.
func (m *OutlinerModel) PeelQueryReadoutRow(index int) tea.Cmd {
	rows := m.QueryReadoutRows()
	if index < 0 || index >= len(rows) || index >= queryReadoutRowLimit {
		m.SetTransientStatus("Nothing to peel there")
		return status.DefaultStatusTimeout()
	}
	row := rows[index]
	// A population peel reloads, and a ranked projection is scored over the
	// snapshot that reload replaces, so it refuses here in the same words the f
	// chord refuses in rather than reloading out from under the results.
	if queryReadoutRowRequiresReload(row) && m.ShowsRankedList() {
		m.SetTransientStatus("Exit search before changing filters")
		return status.DefaultStatusTimeout()
	}
	peeled := "Peeled " + row.Label + ": " + row.value
	reload := m.peelQueryReadoutAxis(row)
	if reload {
		m.reseatQueryReadoutAfterPeel()
		return func() tea.Msg { return QueryReadoutReloadRequestMsg{Status: peeled} }
	}
	m.SetTransientStatus(peeled)
	m.reseatQueryReadoutAfterPeel()
	return status.DefaultStatusTimeout()
}

// reseatQueryReadoutAfterPeel re-seats the window a shorter document left behind,
// here rather than at the next render: the stored offset is what the next scroll
// key adds to, and an offset past the end would make that first press look like
// it did nothing.
//
// It runs after the status the peel publishes, never before it. The readout's
// window is what the strip leaves, and the strip is a row taller while it holds
// a message — so re-seating against a momentarily empty channel would clamp the
// offset against a window one row larger than the one this frame goes on to
// draw, and the render's own clamp can only reduce an offset further. A peel
// that is about to say something measures the strip it is about to have.
func (m *OutlinerModel) reseatQueryReadoutAfterPeel() {
	m.scrollQueryReadout(0)
}

// peelAllQueryReadoutRows peels from the top until the stack is empty. It walks
// one row at a time rather than resetting the view wholesale, because a reset
// would also drop the sort and the folds, which narrow nothing.
//
// It needs no search guard of its own: the query row sits above every facet row,
// so any ranked projection is already gone by the time a population row is
// reached.
func (m *OutlinerModel) peelAllQueryReadoutRows() tea.Cmd {
	peeled := 0
	reload := false
	for range queryReadoutPeelAllLimit {
		rows := m.QueryReadoutRows()
		if len(rows) == 0 {
			break
		}
		if m.peelQueryReadoutAxis(rows[0]) {
			reload = true
		}
		peeled++
	}
	if peeled == 0 {
		m.SetTransientStatus("Nothing to peel")
		return status.DefaultStatusTimeout()
	}
	summary := fmt.Sprintf("Peeled %d narrowing(s)", peeled)
	if reload {
		m.reseatQueryReadoutAfterPeel()
		return func() tea.Msg { return QueryReadoutReloadRequestMsg{Status: summary} }
	}
	m.SetTransientStatus(summary)
	m.reseatQueryReadoutAfterPeel()
	return status.DefaultStatusTimeout()
}

// peelQueryReadoutAxis is the one write. Each axis is taken away through the
// same call the key that set it would use, so peeling can never reach a state
// the ordinary grammar cannot. It reports whether the peel owes a population
// reload, which is the one thing this model cannot do for itself.
func (m *OutlinerModel) peelQueryReadoutAxis(row queryReadoutRow) bool {
	switch row.Axis {
	case queryReadoutAxisLens:
		// The same act Esc performs: a lens is one atomic context, so leaving it
		// returns the view it was opened over rather than stripping a clause out
		// of the middle of it.
		m.RestoreLensBaseline()
	case queryReadoutAxisScope:
		m.ClearProjectScope()
	case QueryReadoutAxisMode:
		_, _ = m.SelectOutlinerMode(chrome.ModeAll)
	case queryReadoutAxisQuery:
		m.ClearSearch()
	case queryReadoutAxisLensQuery:
		m.SetLensQuery("")
		m.Population.Invalidate()
		RebuildOutlinerPreservingCursor(m)
	case queryReadoutAxisClauses:
		m.ReplaceFilterClauses(row.fields, nil)
		if queryReadoutRowRequiresReload(row) {
			return true
		}
		m.Population.Invalidate()
		RebuildOutlinerPreservingCursor(m)
	}
	return false
}

// SaveViewAsLensRequestMsg asks the shell to save the live view as a named lens.
// The readout raises it rather than executing the operation itself: the lens
// operations answer only to a context the shell grants, and the name is typed
// into an overlay the shell owns.
type SaveViewAsLensRequestMsg struct{}

// QueryReadoutReloadRequestMsg asks the shell to reload the population after a
// peel wrote a facet that decides which projects load. The clauses are already
// written when this is raised, so the shell only reloads and reports.
type QueryReadoutReloadRequestMsg struct{ Status string }
