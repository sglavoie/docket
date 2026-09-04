package tui

import (
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/query"
)

// searchState is the whole of the Outliner's slash search: the frame a query was
// typed in, the compiled reading of that query, the input's own suggestion pane
// and recall ring, and the notes tier the query may reach. Four concerns, one
// owner, because the query is what makes the other three mean anything — the
// compilation is that query's reading, the pane describes the token being typed
// into it, and the notes tier exists only for a query carrying no sigil.
//
// resetSearchCompilation is the evidence: dropping a query drops the pane, the
// gate and the retained note payloads in one act, and it has to, or a pane
// outlives the search it was describing and a note payload outlives the row that
// named it.
//
// It is deliberately not the lens's narrowing query. A lens narrows its own
// saved mode, facets and sort and keeps its compiled form in lensGate,
// lensRemainder and lensNarrowRelaxed on the model, while ordinary search
// replaces the whole projection and stays scope-wide. At most one of the two is
// ever live, which is what lets activeNarrowingGate answer for both.
//
// The zero value is inert: no frame, no gate, an empty ring, and a nil index
// accessor that reads as an absent tier rather than a missing dependency. That
// is what keeps a struct-literal test model and the Project Hub's preview
// painter working without a constructor.
type searchState struct {
	// The frame. projection is the navigation frame the query was typed in, and
	// its presence is the same thing as saying the view holds a query. rankedList
	// is the other presentation of that same query — one flat relevance-ordered
	// list rather than the mode's own lanes — toggled deliberately, never entered
	// by typing, and it dies with the query that gave it rows.
	projection *outlinerSearchSnapshot
	rankedList bool

	// The compiled query. remainder is the free text left after the sigil gate was
	// lifted out of it and gate is that gate; scoring, smart case, highlighting and
	// the work-mode predicate all read the remainder, so a predicate token is never
	// scored or highlighted as prose. caseRelaxed records that the last rebuild
	// found nothing under the query's smart-case reading and fell back to a
	// case-insensitive pass, so what matched and what is shown can never disagree.
	remainder   string
	gate        *query.SearchPredicates
	caseRelaxed bool

	// The input's own state. autocomplete is the suggestion pane, holding no cached
	// vocabulary: every refresh reprojects the grammar and rereads the live
	// candidates, so a mutation or a scope change cannot leave a stale value behind.
	// history is the session-only ring of submitted queries, walked with Up and
	// Down inside the input, never persisted, and unlike jump history not scoped to
	// a projection: a query is worth recalling in whatever mode or scope the user
	// reaches for it next.
	autocomplete outlinerSearchAutocomplete
	history      outlinerSearchHistory

	// The notes tier. noteIndex is the shell-injected accessor over the
	// workspace-wide note index the tier ranks, so the Outliner never learns where
	// notes live or when they are read; a nil accessor means the tier is simply
	// absent. noteMatches is what the tier's rows are rendered from, keyed the way
	// the Notes manager keys its own match map and held beside the rows rather than
	// on them because a note row's render payload is wide and outlinerRow is copied
	// for every row of every rebuild. noteCursorReseat is the note the cursor rested
	// on when an accepted index refresh forced a rebuild, captured because that
	// rebuild re-ranks the tier and the numeric cursor alone would silently name a
	// different result.
	noteIndex        func() []notes.LocatedNoteRow
	noteMatches      map[notes.NoteRowIdentity]outlinerSearchNoteMatch
	noteCursorReseat notes.NoteRowIdentity
}

// active reports that a search frame is open, which is the same thing as saying
// the view holds a query. It says nothing about how that query is presented.
func (s *searchState) active() bool { return s.projection != nil }

// showsRanked reports that the held query is being presented as one flat
// relevance-ordered list. A query alone never freezes an axis; this is what the
// operations that reshape lanes, grouping, manual order and the zoom refuse on.
func (s *searchState) showsRanked() bool { return s.rankedList && s.projection != nil }

// begin opens the frame over the presentation the caller captured. Taking the
// snapshot by value is what guarantees the retained frame is the caller's
// capture and not an alias of anything still being written.
func (s *searchState) begin(snapshot outlinerSearchSnapshot) { s.projection = &snapshot }

// toggleRanked switches the held query between narrowing the mode and the flat
// ranked list, and reports which presentation now stands. What else the switch
// costs stays the caller's: the section zoom belongs to the projection rather
// than to the query, so suspending it is a decision made outside here.
func (s *searchState) toggleRanked() bool {
	s.rankedList = !s.rankedList
	return s.rankedList
}

// take hands back the frame to restore and puts the search frame down in one
// act, so the restoration cannot be replayed by whatever opens next. The
// compilation is not the frame's and stays for the caller's own reset.
func (s *searchState) take() *outlinerSearchSnapshot {
	projection := s.projection
	s.discard()
	return projection
}

// discard puts the frame down. take above is the same act with the frame handed
// back on the way out; the two clear paths differ in nothing else, because an
// intentional navigation reset replaces the projection rather than dismissing
// it and so leaves nobody to restore the frame onto.
func (s *searchState) discard() {
	s.projection = nil
	s.rankedList = false
}

// searchSnapshot is the part of this state a lens restoration point carries: the
// frame ordinary search was itself holding and the presentation it stood under.
// The compilation is absent because it is not state a frame owns but the last
// rebuild's reading of a query, and the rebuild that follows a restore produces
// its own; the suggestion pane and the recall ring are absent for the same
// reason and because the ring is not scoped to a projection at all.
type searchSnapshot struct {
	projection *outlinerSearchSnapshot
	rankedList bool
}

// capture reads the frame out. The pointer is retained rather than copied
// because begin already took the presentation by value, so what is shared here is
// a frame nothing writes to again.
func (s *searchState) capture() searchSnapshot {
	if s == nil {
		return searchSnapshot{}
	}
	return searchSnapshot{projection: s.projection, rankedList: s.rankedList}
}

// restore installs a captured frame and the presentation it stood under, then
// drops the compilation the replaced query left behind. It is discard's inverse
// on both counts: a lens restoration point carries search's own restoration
// point, so handing it back has to hand back both axes together or the flat list
// returns over a query that is no longer held, and a reading of the lens query
// that is going away would badge and highlight rows the restored query never
// matched.
func (s *searchState) restore(snapshot searchSnapshot) {
	if s == nil {
		return
	}
	s.projection = snapshot.projection
	s.rankedList = snapshot.rankedList
	s.resetCompilation()
}

// resetCompilation drops everything derived from the query and nothing else.
// Suggestions describe the query being compiled, so they die with it; the
// retained note payloads hold every matched note's body and would otherwise
// outlive the rows that named them.
func (s *searchState) resetCompilation() {
	if s == nil {
		return
	}
	s.autocomplete.clear()
	s.setCompilation("", nil, false)
	s.clearNoteMatches()
}

// setCompilation installs the reading of the query the visible result set was
// produced under. The three are one act because they are one pass's answer: a
// remainder without the gate it was lifted from would score the sigils as prose,
// and a case reading without the pass that took it would highlight rows that
// pass never matched.
func (s *searchState) setCompilation(remainder string, gate *query.SearchPredicates, relaxed bool) {
	s.remainder = remainder
	s.gate = gate
	s.caseRelaxed = relaxed
}

// setCaseRelaxed records the case reading alone, for the pass that lifts the
// gate out first and only learns whether it had to relax after scoring under it.
func (s *searchState) setCaseRelaxed(relaxed bool) { s.caseRelaxed = relaxed }

// setNoteIndex injects the shell's accessor over the workspace-wide note index.
func (s *searchState) setNoteIndex(accessor func() []notes.LocatedNoteRow) { s.noteIndex = accessor }

// setNoteMatches retains the payloads the tier's rows render from. The map is
// replaced whole on every rebuild, so it can never describe a result set that is
// no longer on screen.
func (s *searchState) setNoteMatches(matches map[notes.NoteRowIdentity]outlinerSearchNoteMatch) {
	s.noteMatches = matches
}

// clearNoteMatches drops those payloads for the passes that emit no notes tier
// at all: only the flat list has one.
func (s *searchState) clearNoteMatches() { s.noteMatches = nil }

// captureNoteCursor records the note the cursor holds ahead of a rebuild that
// will re-rank the tier underneath it.
func (s *searchState) captureNoteCursor(identity notes.NoteRowIdentity) {
	s.noteCursorReseat = identity
}

// takeNoteCursor hands that note back and spends the capture in one call, so
// exactly one rebuild honors it and a later one reseats nothing.
func (s *searchState) takeNoteCursor() notes.NoteRowIdentity {
	identity := s.noteCursorReseat
	s.noteCursorReseat = notes.NoteRowIdentity{}
	return identity
}
