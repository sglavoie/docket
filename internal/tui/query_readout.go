package tui

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
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
	queryReadoutAxisMode
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

// queryReadoutRow is one peelable narrowing. fields is set only for the clause
// rows, where the peel is a write of that facet's own query fields.
type queryReadoutRow struct {
	axis   queryReadoutAxis
	label  string
	value  string
	note   string
	fields []models.QueryField
}

// toggleQueryReadout opens or closes the expanded stack. It is the whole of the
// operation's handler: the strip reads the flag when it next renders, so there
// is no second copy of the readout to keep in step.
func (m *OutlinerModel) toggleQueryReadout() {
	if m == nil {
		return
	}
	m.queryReadoutExpanded = !m.queryReadoutExpanded
}

// closeQueryReadout puts the stack down without asking what it was showing. The
// resets that replace the whole projection call it, so a readout cannot survive
// into a view whose axes it was never built from.
func (m *OutlinerModel) closeQueryReadout() {
	if m == nil {
		return
	}
	m.queryReadoutExpanded = false
}

// queryReadoutRows lists the active narrowings, widest context first. The order
// is the one the operational title already reads left to right — lens, scope,
// mode, query, then the facets — so a row's position in the stack matches where
// the same narrowing is named in the header above it.
func (m *OutlinerModel) queryReadoutRows() []queryReadoutRow {
	if m == nil {
		return nil
	}
	rows := make([]queryReadoutRow, 0, 8)
	if m.hasActiveLens() {
		value := m.activeLens.name
		if m.lensModified() {
			value += " (modified)"
		}
		rows = append(rows, queryReadoutRow{
			axis:  queryReadoutAxisLens,
			label: "lens",
			value: value,
			note:  "restores the view it opened on",
		})
	}
	if m.isProjectScoped() {
		rows = append(rows, queryReadoutRow{
			axis:  queryReadoutAxisScope,
			label: "scope",
			value: strings.Join(m.scopeChainNames(), " / "),
		})
	}
	// All is the unnarrowed mode, so it is the absence of a row rather than a row
	// saying nothing is narrowed.
	if mode := m.activeOutlinerModeEntry(); mode.ID != ModeAll {
		rows = append(rows, queryReadoutRow{
			axis:  queryReadoutAxisMode,
			label: "mode",
			value: mode.Label,
			note:  "peels to All",
		})
	}
	// A lens's narrowing query and an ordinary search projection are two
	// different axes that can never both be live: slash search under a lens edits
	// that lens's query rather than opening a projection of its own.
	switch {
	case m.hasActiveLens() && m.lensQuery != "":
		rows = append(rows, queryReadoutRow{
			axis:  queryReadoutAxisLensQuery,
			label: "lens query",
			value: m.lensQuery,
		})
	case m.SearchQuery() != "":
		rows = append(rows, queryReadoutRow{
			axis:  queryReadoutAxisQuery,
			label: "query",
			value: m.SearchQuery(),
		})
	}
	rows = append(rows, commonFilterReadoutRows(&m.filter.CommonFilterState)...)
	return append(rows, projectFilterReadoutRows(&m.projectFilter)...)
}

// clauseReadoutRow builds one facet row from the clauses that facet currently
// spells. The value is filterClauseParts output, so a readout row, a header
// chip, and a typed query name the same narrowing with the same characters.
func clauseReadoutRow(
	label string,
	fields []models.QueryField,
	clauses []models.QueryClause,
	note string,
) queryReadoutRow {
	return queryReadoutRow{
		axis:   queryReadoutAxisClauses,
		label:  label,
		value:  strings.Join(filterClauseParts(clauses), " "),
		note:   note,
		fields: fields,
	}
}

// commonFilterReadoutRows splits the row cache into one row per independently
// clearable facet. The split is commonFilterFacetCount's, promoted from a count
// into the peel unit rather than invented beside it, which is why the completion
// age facet carries the terminal-state clauses it installed and the state facet
// steps aside when it does.
//
//nolint:gocyclo // One row per facet, kept together so the set stays auditable.
func commonFilterReadoutRows(f *CommonFilterState) []queryReadoutRow {
	if f == nil {
		return nil
	}
	rows := make([]queryReadoutRow, 0, 10)
	if f.hasStateFilter() && !completionAgeOwnsTerminalStates(f) {
		clauses := make([]models.QueryClause, 0, 4)
		for _, state := range f.States.Values() {
			clauses = appendFilterClause(clauses, models.QueryFieldState,
				models.QueryOperatorIs, string(state))
		}
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldState}, clauses, ""))
	}
	if f.hasCompletionAgeFilter() {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldCompleted, models.QueryFieldState},
			completionAgeClauses(f.CompletionAge), ""))
	}
	if f.Priority > 0 {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldPriority},
			filterClauseList(models.QueryFieldPriority, models.QueryOperatorAtLeast,
				strconv.Itoa(f.Priority)), ""))
	}
	if len(f.Tags) > 0 {
		clauses := make([]models.QueryClause, 0, len(f.Tags))
		for _, tag := range slices.Sorted(maps.Keys(f.Tags)) {
			clauses = appendFilterClause(clauses, models.QueryFieldTag,
				models.QueryOperatorIs, tag)
		}
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldTag}, clauses, ""))
	}
	if len(f.Contexts) > 0 || f.NoContext {
		names := slices.Sorted(maps.Keys(f.Contexts))
		if f.NoContext {
			names = append(names, noContextFilterKey)
		}
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldContext},
			contextFilterClauses(names), ""))
	}
	if f.DueDate != DueDateAll {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldDue}, dueFilterClauses(f.DueDate), ""))
	}
	if f.Hierarchy != HierarchyAll {
		rows = append(rows, clauseReadoutRow("filter",
			filterQueryHierarchyFields, hierarchyFilterClauses(f.Hierarchy), ""))
	}
	if f.Dependency != DependencyFilterAll {
		rows = append(rows, clauseReadoutRow("filter",
			filterQueryDependencyFields, dependencyFilterClauses(f.Dependency), ""))
	}
	if f.Stale {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldStale},
			filterClauseList(models.QueryFieldStale, models.QueryOperatorPresent, ""), ""))
	}
	if f.Deferred {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldDeferred},
			filterClauseList(models.QueryFieldDeferred, models.QueryOperatorPresent, ""), ""))
	}
	if f.Pinned {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldPinned},
			filterClauseList(models.QueryFieldPinned, models.QueryOperatorPresent, ""), ""))
	}
	if f.Review {
		rows = append(rows, clauseReadoutRow("filter",
			[]models.QueryField{models.QueryFieldReview},
			filterClauseList(models.QueryFieldReview, models.QueryOperatorPresent, ""), ""))
	}
	return rows
}

// projectFilterReadoutRows does the same for the population half. Each row says
// "reload" because peeling it reloads the projects rather than rebuilding the
// rows, which is the one thing a population facet does that a row facet does not.
func projectFilterReadoutRows(f *ProjectFilterState) []queryReadoutRow {
	if f == nil {
		return nil
	}
	rows := make([]queryReadoutRow, 0, 3)
	if tags := projectTagClauses(f); len(tags) > 0 {
		rows = append(rows, clauseReadoutRow("population",
			[]models.QueryField{models.QueryFieldProjectTag}, tags, "reload"))
	}
	if clauses := populationClauses(models.QueryFieldParked, f.ParkedMode); len(clauses) > 0 {
		rows = append(rows, clauseReadoutRow("population",
			[]models.QueryField{models.QueryFieldParked}, clauses, "reload"))
	}
	if clauses := populationClauses(models.QueryFieldArchived, f.ArchivedMode); len(clauses) > 0 {
		rows = append(rows, clauseReadoutRow("population",
			[]models.QueryField{models.QueryFieldArchived}, clauses, "reload"))
	}
	return rows
}

// queryReadoutBody renders the expanded stack, or the empty string when it is
// collapsed. It is pure, because statusStripHeight measures exactly this string
// before statusStripBody paints it.
func (m *OutlinerModel) queryReadoutBody(width int) string {
	if m == nil || !m.queryReadoutExpanded {
		return ""
	}
	width = max(width, 1)
	rows := m.queryReadoutRows()
	lines := make([]string, 0, queryReadoutRowLimit+2)
	lines = append(lines, TitleStyle.Render(truncateToVisualWidth("Narrowing this view", width)))
	if len(rows) == 0 {
		return strings.Join(append(lines,
			MutedStyle.Render(truncateToVisualWidth("nothing: every axis is at its default", width))), "\n")
	}
	shown := min(len(rows), queryReadoutRowLimit)
	labelWidth := 0
	for _, row := range rows[:shown] {
		labelWidth = max(labelWidth, lipgloss.Width(row.label))
	}
	for index, row := range rows[:shown] {
		text := fmt.Sprintf("%d %-*s %s", index+1, labelWidth, row.label, row.value)
		if row.note != "" {
			text += "  (" + row.note + ")"
		}
		lines = append(lines, MutedStyle.Render(truncateToVisualWidth(text, width)))
	}
	if dropped := len(rows) - shown; dropped > 0 {
		// Named rather than dropped: the digits are the peel vocabulary, so a
		// stack past them still has to say what is not on it.
		lines = append(lines, MutedStyle.Render(truncateToVisualWidth(
			fmt.Sprintf("%d more narrowing(s) below the peel digits; peel one to reach them", dropped),
			width)))
	}
	return strings.Join(lines, "\n")
}

// queryReadoutGuidance is the footer while the stack is open. It is the gated
// keymap a modal sub-state supplies, so the readout takes no footer rank of its
// own and the ranked projection stays untouched.
func (m *OutlinerModel) queryReadoutGuidance() GuidanceSpec {
	closeKey := "Q"
	if m.operationRegistry != nil {
		if key := m.operationRegistry.InputShortcutForOperation(
			InputStrategySimplified, operationCoreQueryReadout); key != "" {
			closeKey = key
		}
	}
	rows := m.queryReadoutRows()
	if len(rows) == 0 {
		return GuidanceSpec{
			Hints: requiredGuidanceHints(closeKey+" close readout", "Esc close readout"),
			Rows:  1,
		}
	}
	peel := "1 peel one"
	if shown := min(len(rows), queryReadoutRowLimit); shown > 1 {
		peel = fmt.Sprintf("1-%d peel one", shown)
	}
	return GuidanceSpec{
		Hints: append(requiredGuidanceHints(peel, "x peel all", closeKey+"/Esc close"),
			optionalGuidanceHints(10, "s save as lens")...),
		Rows: 2,
	}
}

// handleQueryReadoutKey is the readout's own gated keymap. The stack owns input
// while it is open, so an unlisted key closes it rather than falling through to
// a grammar whose footer is not on screen.
func (m *OutlinerModel) handleQueryReadoutKey(key string) (tea.Model, tea.Cmd) {
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
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return m, m.peelQueryReadoutRow(int(key[0] - '1'))
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
func (m *OutlinerModel) peelQueryReadoutRow(index int) tea.Cmd {
	rows := m.queryReadoutRows()
	if index < 0 || index >= len(rows) || index >= queryReadoutRowLimit {
		m.setTransientStatus("Nothing to peel there")
		return defaultStatusTimeout()
	}
	row := rows[index]
	// A population peel reloads, and a ranked projection is scored over the
	// snapshot that reload replaces, so it refuses here in the same words the f
	// chord refuses in rather than reloading out from under the results.
	if queryReadoutRowRequiresReload(row) && m.showsRankedList() {
		m.setTransientStatus("Exit search before changing filters")
		return defaultStatusTimeout()
	}
	status := "Peeled " + row.label + ": " + row.value
	if m.peelQueryReadoutAxis(row) {
		return func() tea.Msg { return QueryReadoutReloadRequestMsg{Status: status} }
	}
	m.setTransientStatus(status)
	return defaultStatusTimeout()
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
		rows := m.queryReadoutRows()
		if len(rows) == 0 {
			break
		}
		if m.peelQueryReadoutAxis(rows[0]) {
			reload = true
		}
		peeled++
	}
	if peeled == 0 {
		m.setTransientStatus("Nothing to peel")
		return defaultStatusTimeout()
	}
	status := fmt.Sprintf("Peeled %d narrowing(s)", peeled)
	if reload {
		return func() tea.Msg { return QueryReadoutReloadRequestMsg{Status: status} }
	}
	m.setTransientStatus(status)
	return defaultStatusTimeout()
}

// peelQueryReadoutAxis is the one write. Each axis is taken away through the
// same call the key that set it would use, so peeling can never reach a state
// the ordinary grammar cannot. It reports whether the peel owes a population
// reload, which is the one thing this model cannot do for itself.
func (m *OutlinerModel) peelQueryReadoutAxis(row queryReadoutRow) bool {
	switch row.axis {
	case queryReadoutAxisLens:
		// The same act Esc performs: a lens is one atomic context, so leaving it
		// returns the view it was opened over rather than stripping a clause out
		// of the middle of it.
		m.restoreLensBaseline()
	case queryReadoutAxisScope:
		m.clearProjectScope()
	case queryReadoutAxisMode:
		_, _ = m.selectOutlinerMode(ModeAll)
	case queryReadoutAxisQuery:
		m.ClearSearch()
	case queryReadoutAxisLensQuery:
		m.setLensQuery("")
		m.crossForceRebuild = true
		rebuildOutlinerPreservingCursor(m)
	case queryReadoutAxisClauses:
		m.replaceFilterClauses(row.fields, nil)
		if queryReadoutRowRequiresReload(row) {
			return true
		}
		m.crossForceRebuild = true
		rebuildOutlinerPreservingCursor(m)
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
