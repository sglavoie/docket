package outliner

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/util"
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
		note := "restores the view it opened on"
		if m.LensModified() {
			value += " (modified)"
			note = "Changed: " + strings.Join(m.LensChangedAxes(), ", ") + " · " + note
		}
		rows = append(rows, queryReadoutRow{
			Axis:  queryReadoutAxisLens,
			Label: "lens",
			value: value,
			note:  note,
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
		note := "peels to All"
		if m.ShowsRankedList() {
			note = "return mode; search spans all modes · peels to All"
		}
		rows = append(rows, queryReadoutRow{
			Axis:  QueryReadoutAxisMode,
			Label: "mode",
			value: mode.Label,
			note:  note,
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
		// The count rides the note so the readout states what the query drew
		// beside what peeling it does.
		row.note = m.searchMatchCountLabel()
		if m.HasSuspendedLens() {
			row.note += " · restores the suspended lens"
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
	if len(f.Projects) > 0 {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldProject},
			query.ProjectFilterClauses(f.Projects), ""))
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
	// Hidden is a row facet that owes a reload, since hidden work only enters a
	// projection once a query opens its population; the note says so, as the
	// population rows below do.
	if clauses := query.PopulationClauses(models.QueryFieldHidden, f.HiddenMode); len(clauses) > 0 {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldHidden}, clauses, "reload"))
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
		m.SetTransientStatus("Filters are locked in the ranked list · or returns to the narrowed search")
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
	summary := "Peeled " + util.Count(peeled, "narrowing")
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
