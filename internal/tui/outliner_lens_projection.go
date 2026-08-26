package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// A lens query narrows the projection its own mode, facets, scope, and sort
// already produced. That is the whole difference from ordinary slash search,
// which replaces the projection with a scope-wide relevance-ordered result set:
// here the saved order stands, the rows are a subset of what the mode was
// already showing, and the query only decides which of them survive.

// lensNarrowing is one compiled lens query. retained is what the projection may
// still admit — every match plus the ancestors that give a match its hierarchy
// context — and matched is the attribution source for the actual hits alone, so
// a retained ancestor carries no cue it did not earn.
type lensNarrowing struct {
	active   bool
	retained map[TaskIdentity]bool
	matched  map[TaskIdentity]outlinerMatchSource
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

// compileLensNarrowing parses the lens query through the shared search grammar
// and resolves the identities the projection may keep. It records the compiled
// gate, remainder, and case mode on the model so chrome reads exactly what the
// rows were built from.
func (m *OutlinerModel) compileLensNarrowing(projects []*models.Project) lensNarrowing {
	if m == nil {
		return lensNarrowing{}
	}
	m.lensGate = nil
	m.lensRemainder = ""
	m.lensNarrowRelaxed = false
	if m.lensQuery == "" {
		return lensNarrowing{}
	}

	remainder, predicates := m.compileSearchQuery(m.lensQuery, projects)
	// The saved mode and facets already decide whether terminal work is on
	// screen, so the gate must not exclude it a second time: a lens saved over
	// completed work would otherwise narrow to nothing.
	predicates.admitsTerminal = true
	m.lensRemainder = remainder
	m.lensGate = predicates

	narrowing := lensNarrowing{
		active:   true,
		retained: make(map[TaskIdentity]bool),
		matched:  make(map[TaskIdentity]outlinerMatchSource),
	}
	if predicates.invalid() {
		// The guidance stands and nothing is admitted, which is what ordinary
		// search does with the same query.
		return narrowing
	}

	matcher := fuzzy.SmartCase(remainder)
	matches := searchProjectionMatchesWithin(projects, remainder, nil, matcher, predicates)
	if len(matches) == 0 && matcher.CaseSensitive() {
		matches = searchProjectionMatchesWithin(projects, remainder, nil, fuzzy.NewMatcher(false), predicates)
		m.lensNarrowRelaxed = len(matches) > 0
	}

	for _, match := range matches {
		identity := taskIdentityFor(match.task, match.project)
		if !identity.Valid() {
			continue
		}
		narrowing.matched[identity] = match.source
		narrowing.retained[identity] = true
		retainLensAncestors(narrowing.retained, match.task, match.project)
	}
	return narrowing
}

// retainLensAncestors admits the canonical ancestry of one match. It follows
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
