package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// A narrowing query narrows the projection its own mode, facets, scope, and sort
// already produced: the order stands, the rows are a subset of what the mode was
// already showing, and the query only decides which of them survive. Both
// carriers that hold free text compile through here — a lens's saved query and
// the slash-search input — because a query is a clause, not a mode. The flat
// ranked list is a presentation of the same rows and is reached by toggling it,
// never by typing.

// queryNarrowing is one compiled narrowing query. retained is what the
// projection may still admit — every match plus the ancestors that give a match
// its hierarchy context — and matched is the attribution source for the actual
// hits alone, so a retained ancestor carries no cue it did not earn.
type queryNarrowing struct {
	active   bool
	retained map[TaskIdentity]bool
	matched  map[TaskIdentity]outlinerMatchSource
}

// narrowingCompilation is everything compiling one narrowing query produced. The
// carrier that asked for it records the remainder, gate, and case mode where its
// own chrome reads them; the narrowing itself is what the projection consumes.
type narrowingCompilation struct {
	narrowing queryNarrowing
	remainder string
	gate      *searchPredicates
	relaxed   bool
}

// setLensQuery installs the narrowing query and discards the compilation of the
// previous one. Nothing is rebuilt here: the next projection compiles it.
func (m *OutlinerModel) setLensQuery(query string) {
	if m == nil {
		return
	}
	m.lensQuery = strings.TrimSpace(query)
	m.lensGate = nil
	m.lensRemainder = ""
	m.lensNarrowRelaxed = false
}

// lensGuidanceText returns the refusal a recognized-but-unhonorable lens query
// produced, or the empty string. It is the same guidance ordinary search shows,
// because both queries compile through the same grammar.
func (m *OutlinerModel) lensGuidanceText() string {
	if m == nil {
		return ""
	}
	return m.lensGate.guidanceText()
}

// lensTextQuery is the free-text remainder of the lens query, which is what
// scoring and highlighting must read: the raw query would score its own sigils
// as prose.
func (m *OutlinerModel) lensTextQuery() string {
	if m == nil {
		return ""
	}
	return m.lensRemainder
}

// lensCaseSensitive reports the case mode the visible rows were actually
// produced under, so a highlight can never disagree with the narrowing.
func (m *OutlinerModel) lensCaseSensitive() bool {
	if m == nil || m.lensNarrowRelaxed {
		return false
	}
	return fuzzy.CaseSensitiveQuery(m.lensTextQuery())
}

// compileActiveNarrowing compiles whichever narrowing query the view holds and
// records the compilation where that carrier's chrome reads it. A lens owns the
// query while one is active, because slash search under a lens edits the lens's
// query rather than opening a projection of its own; otherwise the search input
// owns it. At most one is ever live, which is why one gate answers for both.
func (m *OutlinerModel) compileActiveNarrowing(projects []*models.Project) queryNarrowing {
	if m == nil {
		return queryNarrowing{}
	}
	if m.hasActiveLens() {
		compiled := m.compileNarrowingQuery(m.lensQuery, projects)
		m.lensRemainder, m.lensGate, m.lensNarrowRelaxed = compiled.remainder, compiled.gate, compiled.relaxed
		return compiled.narrowing
	}
	query := ""
	if m.hasSearchProjection() {
		query = m.SearchQuery()
	}
	compiled := m.compileNarrowingQuery(query, projects)
	m.searchRemainder, m.searchGate, m.searchCaseRelaxed = compiled.remainder, compiled.gate, compiled.relaxed
	return compiled.narrowing
}

// compileNarrowingQuery parses one query through the shared search grammar and
// resolves the identities the projection may keep. An empty query compiles to an
// inactive narrowing rather than to a gate that admits everything, so a view with
// no query does no scoring work at all.
func (m *OutlinerModel) compileNarrowingQuery(query string, projects []*models.Project) narrowingCompilation {
	if m == nil || strings.TrimSpace(query) == "" {
		return narrowingCompilation{}
	}

	remainder, predicates := m.compileSearchQuery(query, projects)
	compiled := narrowingCompilation{
		remainder: remainder,
		gate:      predicates,
		narrowing: queryNarrowing{
			active:   true,
			retained: make(map[TaskIdentity]bool),
			matched:  make(map[TaskIdentity]outlinerMatchSource),
		},
	}
	if predicates.invalid() {
		// The guidance stands and nothing is admitted.
		return compiled
	}

	matcher := fuzzy.SmartCase(remainder)
	matches := searchProjectionMatchesWithin(projects, remainder, nil, matcher, predicates)
	if len(matches) == 0 && matcher.CaseSensitive() {
		matches = searchProjectionMatchesWithin(projects, remainder, nil, fuzzy.NewMatcher(false), predicates)
		compiled.relaxed = len(matches) > 0
	}

	for _, match := range matches {
		identity := taskIdentityFor(match.task, match.project)
		if !identity.Valid() {
			continue
		}
		compiled.narrowing.matched[identity] = match.source
		compiled.narrowing.retained[identity] = true
		retainNarrowingAncestors(compiled.narrowing.retained, match.task, match.project)
	}
	return compiled
}

// retainNarrowingAncestors admits the canonical ancestry of one match. It follows
// stored ParentID links through the authoritative project rather than a row's
// own Children, which the filter pipeline may have rewritten.
func retainLensAncestors(retained map[TaskIdentity]bool, task *models.Task, project *models.Project) {
	if project == nil {
		return
	}
	seen := make(map[string]bool)
	for current := task; current != nil && current.ParentID != nil; {
		parentID := *current.ParentID
		if seen[parentID] {
			return
		}
		seen[parentID] = true
		parent := project.GetTaskByID(parentID)
		if parent == nil {
			return
		}
		retained[taskIdentityFor(parent, project)] = true
		current = parent
	}
}

// constrainModeFilterToLens composes the narrowing over the mode input's own
// task predicate. Narrowing is applied as one more facet rather than as a
// separate pass, so mode membership, the saved facets, the section emitters,
// the project grouping, and the saved order all keep deciding the rows: an
// emptied section or project group simply stops being emitted.
func (m *OutlinerModel) constrainModeFilterToLens(
	base func(*models.Task, *models.Project) bool,
	narrowing lensNarrowing,
) func(*models.Task, *models.Project) bool {
	if !narrowing.active {
		return base
	}
	return func(task *models.Task, project *models.Project) bool {
		if base != nil && !base(task, project) {
			return false
		}
		if project == nil {
			project = m.project
		}
		return narrowing.retained[taskIdentityFor(task, project)]
	}
}

// attributeLensMatches stamps the match cue on the rows that actually matched.
// Attribution is recomputed with membership on every rebuild, so a mutation that
// moves a hit from one field to another moves its cue too, and an ancestor
// retained only for context keeps matchSourceNone.
func (m *OutlinerModel) attributeLensMatches(narrowing lensNarrowing) {
	if !narrowing.active || len(narrowing.matched) == 0 {
		return
	}
	for index := range m.rows {
		row := m.rows[index]
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		if source, ok := narrowing.matched[m.sectionRowIdentity(row)]; ok {
			m.rows[index].MatchSource = source
		}
	}
}
