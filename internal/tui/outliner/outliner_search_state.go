package outliner

import (
	"context"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
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
	queued      tea.Cmd
	snapshot    *backgroundSearchSnapshot
	flight      *backgroundSearch
	worker      *sync.Mutex
	workContext context.Context
	// Scores are content-addressed, not task-addressed, and bounded to 128 MiB.
	// Rebuilds still read current tasks, scope, predicates and ordering.
	scoreCache *fuzzy.ScoreCache
	// The frame. projection is the navigation frame the query was typed in, and
	// its presence is the same thing as saying the view holds a query. rankedList
	// is the other presentation of that same query — one flat relevance-ordered
	// list rather than the mode's own lanes — toggled deliberately, never entered
	// by typing, and it dies with the query that gave it rows.
	Projection *outlinerSearchSnapshot
	rankedList bool

	// suspendedLens is the lens this query is standing in front of, if the slash
	// was pressed under one. It belongs to the frame rather than beside it because
	// the lens and the presentation it owned are put back in the same act the
	// query is dropped in; holding it anywhere else would let a discard leave a
	// lens nothing will ever restore. The pointer is what keeps the type from
	// containing itself: the checkpoint it carries holds this state's own
	// snapshot, which is also what makes a lens survive a relation hop or a second
	// lens taken on top of the search.
	suspendedLens *lensSuspension

	// The compiled query. remainder is the free text left after the sigil gate was
	// lifted out of it and gate is that gate; scoring, smart case, highlighting and
	// the work-mode predicate all read the remainder, so a predicate token is never
	// scored or highlighted as prose. caseRelaxed records that the last rebuild
	// found nothing under the query's smart-case reading and fell back to a
	// case-insensitive pass, so what matched and what is shown can never disagree.
	remainder   string
	Gate        *query.SearchPredicates
	caseRelaxed bool

	// withheld counts the finished and deferred tasks the ranked list's query
	// matches but its population rule kept out, so search can name the sigil
	// that brings them in even when other results are visible.
	withheld         searchWithheldMatches
	hiddenFilterHint string
	// eliminationHint names the sigil clause that emptied a ranked list: one
	// naming a value nothing carries, else the one whose removal restores the
	// most matches. It is decided by the rebuild that found nothing, so the
	// empty state reads an answer rather than recomputing one per frame.
	eliminationHint string

	// drawn counts the task and note rows the last rebuild drew for this query,
	// in either presentation. It is taken when the rows are built rather than
	// recounted at render, so the badge states the projection on screen and a
	// frame never walks the rows to say so.
	drawn int

	// The input's own state. autocomplete is the suggestion pane, holding no cached
	// vocabulary: every refresh reprojects the grammar and rereads the live
	// candidates, so a mutation or a scope change cannot leave a stale value behind.
	// history holds submitted queries, persisted through the shell and walked with
	// Up and Down inside the input. Unlike jump history it is not scoped to
	// a projection: a query is worth recalling in whatever mode or scope the user
	// reaches for it next.
	Autocomplete  outlinerSearchAutocomplete
	history       outlinerSearchHistory
	historyPicker *searchHistoryPicker

	// The notes tier. noteIndex is the shell-injected accessor over the
	// workspace-wide note index the tier ranks, so the Outliner never learns where
	// notes live or when they are read; a nil accessor means the tier is simply
	// absent. Project browsing uses the same index and note-row route.
	// noteMatches holds the current projection's note payloads, keyed the way
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
func (s *searchState) Active() bool { return s.Projection != nil }

// showsRanked reports that the held query is being presented as one flat
// relevance-ordered list. A query alone never freezes an axis; this is what the
// operations that reshape lanes, grouping, manual order and the zoom refuse on.
func (s *searchState) showsRanked() bool { return s.rankedList && s.Projection != nil }

// begin opens the frame over the presentation the caller captured and declares
// which presentation the new query stands in. Taking the snapshot by value is
// what guarantees the retained frame is the caller's capture and not an alias of
// anything still being written; taking the presentation beside it is what keeps
// opening one act, exactly as restore below hands both axes back together.
func (s *searchState) begin(snapshot outlinerSearchSnapshot, ranked bool) {
	s.Projection = &snapshot
	s.rankedList = ranked
}

// lensSuspension is the return frame a temporary search takes when the slash is
// pressed under a live lens. It is the relationship trail's frame in every
// respect but its subject: one presentation checkpoint and one lens frame,
// restored lens-first for the same reason PopRelationReturn gives, because the
// checkpoint owns the narrowing query and writing it second would put an empty
// query over the lens just reinstalled.
//
// modified is read off the live view while the lens still owns it, because the
// marker is a comparison against a fingerprint the suspended view no longer
// produces. Nothing here names a task or a project: the checkpoint's cursor
// anchor is the whole of what a restore aims at, so a target deleted or moved
// meanwhile lands on the nearest surviving row rather than on a stale pointer.
type lensSuspension struct {
	presentation OutlinerPresentationCheckpoint
	lens         *activeLensFrame
	modified     bool
}

// suspend installs the lens frame this query displaced. A nil frame is not a
// suspension to record: a second slash into an open search has no lens to take,
// and letting it write through would drop the one the first slash suspended.
func (s *searchState) suspend(frame *lensSuspension) {
	if s == nil || frame == nil {
		return
	}
	s.suspendedLens = frame
}

// takeSuspendedLens hands the frame back and puts it down in one act, so a
// restoration cannot be replayed onto whatever opens next.
func (s *searchState) takeSuspendedLens() *lensSuspension {
	if s == nil {
		return nil
	}
	frame := s.suspendedLens
	s.suspendedLens = nil
	return frame
}

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
	projection := s.Projection
	s.discard()
	return projection
}

// discard puts the frame down. take above is the same act with the frame handed
// back on the way out; the two clear paths differ in nothing else, because an
// intentional navigation reset replaces the projection rather than dismissing
// it and so leaves nobody to restore the frame onto.
//
// A suspended lens goes down with it for that same reason: the resets that reach
// here are installing a whole projection over this one, and a lens left standing
// would be reinstated by the next Esc over a view it was never suspended under.
// Whatever needs it kept has already captured a checkpoint, which carries it.
func (s *searchState) discard() {
	s.cancelBackground()
	s.snapshot = nil
	s.Projection = nil
	s.rankedList = false
	s.suspendedLens = nil
}

// searchSnapshot is the part of this state a lens restoration point carries: the
// frame ordinary search was itself holding and the presentation it stood under.
// The compilation is absent because it is not state a frame owns but the last
// rebuild's reading of a query, and the rebuild that follows a restore produces
// its own; the suggestion pane and the recall ring are absent for the same
// reason and because the ring is not scoped to a projection at all.
// The lens this frame suspended travels with it, which is what makes the two
// nestings compose: a relation hop or a second lens taken inside a temporary
// search captures a checkpoint holding this snapshot, so unwinding that hop
// hands the search back with the lens it still owes an Esc.
type searchSnapshot struct {
	Projection    *outlinerSearchSnapshot
	rankedList    bool
	suspendedLens *lensSuspension
}

// capture reads the frame out. The pointer is retained rather than copied
// because begin already took the presentation by value, so what is shared here is
// a frame nothing writes to again.
func (s *searchState) capture() searchSnapshot {
	if s == nil {
		return searchSnapshot{}
	}
	return searchSnapshot{
		Projection:    s.Projection,
		rankedList:    s.rankedList,
		suspendedLens: s.suspendedLens,
	}
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
	s.Projection = snapshot.Projection
	s.rankedList = snapshot.rankedList
	s.suspendedLens = snapshot.suspendedLens
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
	s.Autocomplete.clear()
	s.historyPicker = nil
	s.setCompilation("", nil, false)
	s.clearNoteMatches()
	s.hiddenFilterHint = ""
	s.drawn = 0
	s.eliminationHint = ""
}

// setCompilation installs the reading of the query the visible result set was
// produced under. The three are one act because they are one pass's answer: a
// remainder without the gate it was lifted from would score the sigils as prose,
// and a case reading without the pass that took it would highlight rows that
// pass never matched.
func (s *searchState) setCompilation(remainder string, gate *query.SearchPredicates, relaxed bool) {
	s.remainder = remainder
	s.Gate = gate
	s.caseRelaxed = relaxed
}

// searchWithheldMatches is what a ranked list would also have shown had its
// query opened the terminal or the deferred population.
type searchWithheldMatches struct {
	completed int
	deferred  int
}

func (w searchWithheldMatches) any() bool { return w.completed > 0 || w.deferred > 0 }

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
func (s *searchState) CaptureNoteCursor(identity notes.NoteRowIdentity) {
	s.noteCursorReseat = identity
}

// takeNoteCursor hands that note back and spends the capture in one call, so
// exactly one rebuild honors it and a later one reseats nothing.
func (s *searchState) takeNoteCursor() notes.NoteRowIdentity {
	identity := s.noteCursorReseat
	s.noteCursorReseat = notes.NoteRowIdentity{}
	return identity
}

// outlinerSearchSnapshot is the navigation frame a slash search was typed in.
// Scope is deliberately not captured, because scope is a navigation frame rather
// than a clause of the query: a search runs inside the frame it was typed in, a
// lens moves the frame, and the frames stack. Changing scope under a held query
// therefore re-answers that query in the new frame instead of editing it, while
// mode, facets, folds, zoom, ordering, and viewport are the frame's own axes and
// are restored when the query is dropped.
//
// The narrowing is held as the canonical query itself and never as the carrier
// caches derived from it, because those caches cannot spell a clause's polarity
// or hold both dependency directions at once: a frame captured under -#tag or
// under ?blocked and ?blocking would come back narrowed by something else. The
// query is the authority everywhere else, and it is the authority here.
type outlinerSearchSnapshot struct {
	Mode               chrome.OutlinerModeID
	filterQuery        models.Query
	Sort               query.SortState
	treeSort           query.SortState
	ExpandedNodes      map[taskref.TaskIdentity]bool
	ExpandedChecklists map[taskref.TaskIdentity]bool
	CollapsedSections  map[string]bool
	focusStack         []FocusEntry
	crossFocusStack    []CrossProjectFocusEntry
	zoom               sectionZoom
	cursor             modeCursorAnchor
}

// outlinerMatchSource names the field that produced a search hit. It is
// transient presentation attribution carried on the projected row only: no
// persisted task or project field records it, and an ordinary (non-search) row
// keeps the zero value, which means "no cue".
type MatchSource int

const (
	MatchSourceNone MatchSource = iota
	MatchSourceTitle
	MatchSourceDescription
	MatchSourceResume
	MatchSourceChecklist
	MatchSourceComment
	MatchSourceTag
	MatchSourceContext
	MatchSourceAttachment
	MatchSourceProject
	MatchSourceDetails
	// matchSourceNote attributes a row to the notes tier rather than to a field
	// of a task. It is the one source whose row carries no task at all.
	MatchSourceNote
)

// The deterministic free-text tiers. Rank is the primary sort key, so a lower
// tier can never be outranked by a higher one regardless of fuzzy score, and
// searchFieldRankDetails is evaluated only after every single-field tier fails.
const (
	searchFieldRankTitle = iota
	searchFieldRankDescription
	// The resume note sits immediately under the description because it is the
	// task's other body prose: it says where the work stopped, so a note the
	// query names is nearly as strong an answer as the description is, and
	// stronger than any of the collections below it.
	searchFieldRankResume
	searchFieldRankChecklist
	searchFieldRankComment
	// Tags and context share one rank; the source retains which axis won.
	searchFieldRankTagContext
	searchFieldRankAttachment
	searchFieldRankProject
	searchFieldRankDetails
	// Notes are the last tier: a task the query names is what the Outliner is
	// for, so no note may outrank one however strongly it scored. It is also the
	// only tier whose rows carry no task.
	searchFieldRankNote
)

type outlinerSearchMatch struct {
	task        *models.Task
	project     *models.Project
	fieldRank   int
	fuzzyScore  int
	source      MatchSource
	sourceIndex int
}
