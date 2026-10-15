package outliner

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
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

// hasSearchProjection reports that a slash-search frame is open, which is the
// same thing as saying the view holds a query. It says nothing about how that
// query is presented: by default the query narrows the mode's own lanes, and
// only showsRankedList reports the flat presentation.
func (m *OutlinerModel) HasSearchProjection() bool {
	return m != nil && m.Search.Active()
}

// showsRankedList reports that the held query is being presented as one flat
// relevance-ordered list. It is the single test every consumer that cannot
// survive a bypassed mode must ask: the ranked list has no lanes, no grouping,
// no manual order, and no section the zoom or the folds gate, so the operations
// that reshape those axes refuse here and nowhere else. A query alone never
// freezes an axis.
func (m *OutlinerModel) ShowsRankedList() bool {
	return m != nil && m.Search.showsRanked()
}

// toggleRankedList switches the held query between narrowing the mode and the
// flat ranked list. Entering suspends the section zoom, which gates emitters the
// flat list does not run; leaving restores nothing else, because every other
// axis stayed live under both presentations.
func (m *OutlinerModel) ToggleRankedList() bool {
	if !m.HasSearchProjection() {
		return false
	}
	if m.Search.toggleRanked() {
		m.clearSectionZoom()
	}
	m.Population.Invalidate()
	RebuildOutlinerPreservingCursor(m)
	return true
}

// BeginSearchProjection opens the frame with the query narrowing the mode's own
// lanes. It is the programmatic seam — SetSearchQuery and the navigation and
// tests that reach through it — and it keeps that presentation deliberately: a
// caller installing a query without a keystroke is asking the mode it left
// standing to answer it, not asking for the flat list instead of it.
func (m *OutlinerModel) BeginSearchProjection() {
	m.beginSearchProjection(false)
}

// beginRankedSearchProjection is the slash key's own open. A typed query is a
// question about the workspace rather than about the lane the cursor happened to
// be standing in, so the user's entry path starts the flat relevance-ordered
// list across modes and the ranked toggle is what narrows it back. Entering
// ranked suspends the section zoom for the one reason ToggleRankedList does —
// the flat list runs none of the emitters that zoom gates — and the frame
// captured in the same act is what hands the zoom back when the query is
// dropped.
func (m *OutlinerModel) beginRankedSearchProjection() {
	m.beginSearchProjection(true)
}

func (m *OutlinerModel) beginSearchProjection(ranked bool) {
	if m == nil || m.Search.Active() {
		return
	}
	m.closeMenuBar()
	m.Search.begin(m.capturePresentationSnapshot(), ranked)
	if ranked {
		m.clearSectionZoom()
	}
	m.Population.Invalidate()
	m.RebuildRows()
}

// ActivateSearch starts or resumes the one temporary search projection, ranked.
// A second slash edits the existing query without replacing its restoration
// snapshot and without restating the presentation: the toggle is a deliberate
// act, so reopening the input over a query already narrowed back into its mode
// must not silently undo it. Under an active lens the same key suspends that
// lens rather than refusing or editing it: search asks one question of the
// workspace, the answer is worth the same inside a lens as outside it, and the
// suspension is what makes the visit temporary.
func (m *OutlinerModel) ActivateSearch() tea.Cmd {
	// Taken before the open, because the checkpoint is the view the search is
	// about to replace. A second slash finds no lens to take and so records
	// nothing, leaving the first slash's suspension exactly as it was.
	suspended := m.suspendActiveLensForSearch()
	m.beginRankedSearchProjection()
	m.Search.suspend(suspended)
	// Every open starts a fresh walk: the ring survives the input closing, the
	// position in it does not, so the first Up always names the newest query.
	m.Search.history.resetRecall()
	return m.BaseSearchableView.ActivateSearch()
}

// SetSearchQuery retains the programmatic seam used by tests and navigation. It
// opens the same one temporary projection slash does and answers the same
// compiled query; what it does not do is choose the flat list, because only a
// keystroke asks for that. Called into an open frame it restates nothing, so
// typing into the input slash opened keeps the presentation slash declared.
func (m *OutlinerModel) SetSearchQuery(query string) {
	if strings.TrimSpace(query) != "" {
		m.BeginSearchProjection()
	}
	m.BaseSearchableView.SetSearchQuery(query)
	if m.HasSearchProjection() {
		m.Population.Invalidate()
		RebuildOutlinerPreservingCursor(m)
	}
}

// suspendActiveLensForSearch puts the live lens down for the duration of one
// temporary search and hands back the frame that reinstates it. The checkpoint
// is the whole return: mode, facets, sort, folds, focus, zoom, cursor,
// selection, Peek, the frozen Recent ordering, and the lens's own narrowing
// query. Scope is deliberately not disturbed — search never leaves the scope it
// was pressed in, so the lens gets back the one it owned by never losing it.
//
// forgetActiveLensFrame is the seam for the reason the relationship hop gives:
// it drops the frame, the application still reading a population for it, and the
// narrowing it installed, while leaving the machine-local resume pointer exactly
// as it was. A temporary search is not a lens being put down, so the next launch
// still opens the lens whatever the search did.
func (m *OutlinerModel) suspendActiveLensForSearch() *lensSuspension {
	if m == nil || !m.HasActiveLens() || m.HasSearchProjection() {
		return nil
	}
	checkpoint := m.CaptureOutlinerPresentationCheckpoint()
	// Read while the lens still owns the view: the marker is a comparison the
	// suspended projection cannot make for itself.
	modified := m.LensModified()
	frame := m.Lens.takeActive()
	m.forgetActiveLensFrame()
	return &lensSuspension{presentation: checkpoint, lens: frame, modified: modified}
}

// HasSuspendedLens reports that a temporary search is standing in front of a
// lens one Esc puts back. It is not HasActiveLens: no lens owns this projection,
// which is why the mode, the query, and every operation reading them answer for
// the search instead.
func (m *OutlinerModel) HasSuspendedLens() bool {
	return m != nil && m.Search.suspendedLens != nil
}

// SearchExitAction names what Esc does to the held query, in the imperative the
// Esc cascade carries and the footer spells after the key. Over a suspended lens
// the key clears nothing: it puts a projection back, and a footer promising
// "clear search" would send the user looking for the lens it silently restored.
// It is the search step's own Action, so the cascade and every hint below read
// one answer.
func (m *OutlinerModel) SearchExitAction() string {
	if m.HasSuspendedLens() {
		return "restore lens"
	}
	return "clear search"
}

// SearchExitHint is that action as a complete footer entry, and
// searchExitClause says the same thing inside a sentence. The live-input hint,
// the empty-state guidance, and the empty-state message all read these, so one
// key cannot be advertised three ways.
func (m *OutlinerModel) SearchExitHint() string {
	return "Esc " + m.SearchExitAction()
}

func (m *OutlinerModel) searchExitClause() string {
	if m.HasSuspendedLens() {
		return "Esc to restore the lens"
	}
	return "Esc to clear"
}

// ClearSearch exits the temporary projection and restores the exact ordinary
// Outliner state. If the original task vanished while search operations were
// mutating data, focus falls back to the nearest surviving task row.
func (m *OutlinerModel) ClearSearch() {
	if m == nil || !m.Search.Active() {
		m.Search.Autocomplete.clear()
		m.BaseSearchableView.ClearSearch()
		return
	}
	m.closeMenuBar()
	// A suspended lens is restored instead of the bare presentation snapshot, and
	// it is the same restore the relationship trail performs: the lens frame
	// first, because the checkpoint carries the narrowing query and installing it
	// second would write an empty one over the lens just put back. The checkpoint
	// subsumes the snapshot begin took — it holds that snapshot — so this path
	// owes neither a second restore nor a second cursor seat.
	if suspended := m.Search.takeSuspendedLens(); suspended != nil {
		m.BaseSearchableView.ClearSearch()
		m.Lens.setActive(suspended.lens)
		m.RestoreOutlinerPresentationCheckpoint(suspended.presentation)
		return
	}
	snapshot := m.Search.take()
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
	m.restorePresentationSnapshot(*snapshot)
	m.Population.Invalidate()
	m.RebuildRows()
	m.restoreSearchCursor(snapshot.cursor)
}

// exitSearchOntoCursorTask dismisses the flat ranked list and lands the ordinary
// projection on the result the cursor named, so a jump reached through it is not
// a dead end. The restored projection can hide the result behind the mode it was
// searched out of, so a hidden result forces All once before it reports that a
// facet still excludes it. A query that is merely narrowing has nothing to exit:
// its rows are already the mode's own, so the caller keeps the query.
func (m *OutlinerModel) ExitSearchOntoCursorTask() (string, bool) {
	if !m.ShowsRankedList() {
		return "", true
	}
	identity := m.CurrentTaskIdentity()
	m.ClearSearch()
	if !identity.Valid() || m.landOnSearchResult(identity) {
		return "", true
	}
	m.SelectOutlinerMode(chrome.ModeAll)
	if m.landOnSearchResult(identity) {
		return "", true
	}
	return "Task is hidden by the active facets", false
}

func (m *OutlinerModel) landOnSearchResult(identity taskref.TaskIdentity) bool {
	task, project := m.TaskAndProjectForIdentity(identity)
	if task == nil {
		return false
	}
	if m.Project != nil {
		m.NavigateToTask(task)
	} else {
		m.NavigateToTaskInProject(task, project)
	}
	if !m.MoveCursorToIdentity(identity) {
		return false
	}
	// The navigation kernels report a hidden target themselves, and that sentence
	// would outlive a landing the forced mode went on to make.
	m.ClearStatus()
	return true
}

func (m *OutlinerModel) restoreSearchCursor(anchor modeCursorAnchor) {
	m.ViewScrollOffset = anchor.scrollOffset
	if anchor.hasTask {
		for index, row := range m.Rows {
			if !row.IsTaskRow() || row.Task == nil || row.Task.ID != anchor.identity.TaskID {
				continue
			}
			if m.Project == nil && row.ProjectID != anchor.identity.ProjectID {
				continue
			}
			m.Cursor = index
			return
		}
		m.moveCursorToNearestTask(anchor.prevCursor)
		return
	}
	m.Cursor = anchor.prevCursor
	m.ClampCursor()
}

// clearSearchProjectionWithoutRestore is reserved for intentional navigation
// resets that replace the current projection rather than dismissing search.
func (m *OutlinerModel) clearSearchProjectionWithoutRestore() {
	if m == nil {
		return
	}
	m.Search.discard()
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
}

// resetSearchCompilation is the model's name for dropping everything derived
// from the held query. Both clear paths route here, so neither can leave a pane
// open over no search; the restoration path reaches the same act through
// searchState.restore, which owes it for the query it is replacing.
func (m *OutlinerModel) resetSearchCompilation() {
	m.Search.resetCompilation()
}

// searchTextQuery is the free-text remainder every consumer of the query must
// read: scoring, smart case, highlighting, and the work-mode predicate. Reading
// the raw query instead would score and highlight the sigils as prose.
func (m *OutlinerModel) searchTextQuery() string {
	if m == nil {
		return ""
	}
	if m.HasSearchProjection() {
		return m.Search.remainder
	}
	return strings.TrimSpace(m.SearchQuery())
}

// searchCaseSensitive reports the case mode the visible result set was actually
// produced under, so highlighting and predicates never disagree with the rows.
func (m *OutlinerModel) searchCaseSensitive() bool {
	if m == nil || m.Search.caseRelaxed {
		return false
	}
	return fuzzy.CaseSensitiveQuery(m.searchTextQuery())
}

func (m *OutlinerModel) buildSearchProjectionRows() {
	projects := m.ProjectsInCatalogOrder(m.WorkModeScopeProjects())
	candidates := m.SearchCandidateBoundary(projects)

	query, predicates := m.compileSearchQuery(m.SearchQuery(), projects)
	// The pass opens unrelaxed: the retry below is the only thing that can raise
	// the case reading, and it has not run yet.
	m.Search.setCompilation(query, predicates, false)

	// The one place the notes tier is admitted or refused. A sigil compiles to an
	// AND gate applied before scoring and a note satisfies no conjunct of it, so
	// the tier is absent whole rather than exempted per conjunct. Deciding once
	// here rather than inside the scorer is what makes the case-relaxed retry
	// below behave identically for free: both passes read the same decision.
	notesAdmitted := !predicates.Invalid() && !predicates.HasAnySigil()

	matcher := fuzzy.SmartCase(query)
	matches := searchProjectionMatchesWithin(projects, query, candidates, matcher, predicates)
	notes := m.scoreNoteSearchTier(query, matcher, notesAdmitted)
	// One query, one case reading: relaxation is decided over the whole result
	// set, so a query that relaxes relaxes for tasks and notes together.
	if len(matches) == 0 && len(notes) == 0 && matcher.CaseSensitive() {
		relaxed := fuzzy.NewMatcher(false)
		matches = searchProjectionMatchesWithin(projects, query, candidates, relaxed, predicates)
		notes = m.scoreNoteSearchTier(query, relaxed, notesAdmitted)
		m.Search.setCaseRelaxed(len(matches) > 0 || len(notes) > 0)
		matcher = relaxed
	}
	m.Search.withheld = searchWithheldMatches{}
	if !predicates.Invalid() {
		m.Search.withheld = countWithheldSearchMatches(
			searchProjectionMatchesWithin(projects, query, candidates, matcher, predicates.WithEveryPopulation()),
			predicates,
		)
	}
	m.Search.hiddenFilterHint = ""
	m.Search.eliminationHint = ""
	if len(matches) == 0 && len(notes) == 0 && !predicates.Invalid() {
		m.Search.hiddenFilterHint = m.removableFilterSearchHint(projects)
		m.Search.eliminationHint = m.searchEliminationHint(projects, candidates)
	}
	m.sortSearchMatchesWithinTier(matches)

	// The task matches arrive ordered by rank, so the notes tier is seated at the
	// first task match its own rank no longer outranks. Reading
	// searchFieldRankNote here rather than appending blind keeps that constant the
	// one statement of where the tier sits, so moving it up the ladder later moves
	// the rows with it.
	boundary := len(matches)
	for index, match := range matches {
		if match.fieldRank >= searchFieldRankNote {
			boundary = index
			break
		}
	}
	m.appendSearchTaskRows(matches[:boundary])
	m.appendSearchNoteRows(notes)
	m.appendSearchTaskRows(matches[boundary:])
}

// countWithheldSearchMatches sorts the matches a widened gate found into the two
// populations the live gate kept out, by the rule AdmitsPopulation reads:
// terminal first, then a pending deferral.
func countWithheldSearchMatches(matches []outlinerSearchMatch, gate *query.SearchPredicates) searchWithheldMatches {
	var withheld searchWithheldMatches
	for _, match := range matches {
		if gate.AdmitsPopulation(match.task) {
			continue
		}
		if match.task.IsTerminalState() {
			// ?completed reads the completion instant, so a finished task
			// carrying none is not one that sigil would bring back.
			if match.task.CompletedAt != nil && !match.task.CompletedAt.IsZero() {
				withheld.completed++
			}
		} else {
			withheld.deferred++
		}
	}
	return withheld
}

func (m *OutlinerModel) appendSearchTaskRows(matches []outlinerSearchMatch) {
	for _, match := range matches {
		row := m.outlinerRowFromModeRow(OutlinerModeRow{
			Task:       match.task,
			Project:    match.project,
			Breadcrumb: taskAncestorBreadcrumb(match.task),
		})
		// Attribution is recomputed by this rebuild alongside membership, so a
		// mutation that moves a hit from one field to another moves its cue too.
		row.MatchSource = match.source
		m.Rows = append(m.Rows, row)
	}
}

// appendSearchNoteRows emits the notes tier and retains what rendering it needs.
// The row carries the note's workspace identity alone, exactly as the Notes
// manager keys its own match map, so the wide payload a render reads — the note,
// its owner label, and the title positions that ranked it — stays off a struct
// every projection copies per row. The map is replaced on every rebuild, so it
// can never describe a result set that is no longer on screen.
func (m *OutlinerModel) appendSearchNoteRows(matches []outlinerSearchNoteMatch) {
	m.Search.clearNoteMatches()
	if len(matches) == 0 {
		return
	}
	payloads := make(map[notes.NoteRowIdentity]outlinerSearchNoteMatch, len(matches))
	for _, note := range matches {
		identity := note.row.Identity()
		payloads[identity] = note
		m.Rows = append(m.Rows, OutlinerRow{
			IsNoteRow:   true,
			NoteRow:     identity,
			MatchSource: MatchSourceNote,
		})
	}
	m.Search.setNoteMatches(payloads)
}

// sortSearchMatchesWithinTier reorders a resolved result set by the active sort
// axis without ever crossing a relevance tier. In a search projection the tier
// is the group, and a sort reorders tasks within their group only, so no
// `· description` hit can float above a title hit. It runs here rather than
// inside searchProjectionMatchesWithin so the lens call site keeps the saved
// order the lens itself installs.
func (m *OutlinerModel) sortSearchMatchesWithinTier(matches []outlinerSearchMatch) {
	if m == nil || m.Sort.Field == query.SortByDefault || len(matches) < 2 {
		return
	}
	field, ascending := m.Sort.Field, m.Sort.Ascending
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].fieldRank != matches[j].fieldRank {
			return matches[i].fieldRank < matches[j].fieldRank
		}
		return query.ApplySortDirection(query.CompareTasks(matches[i].task, matches[j].task, field), ascending)
	})
}

// searchCandidateBoundary returns an authoritative per-project candidate set
// whenever something other than the query itself bounds the result. Two things
// do: task Focus, whose descendants-only branch search must never escape, and
// the active common facets, which are an AND gate applied before scoring
// exactly like the sigils. Search still ignores the current mode, sort, and
// folds. With neither in force the nil return keeps the scope-wide behavior.
//
// The facets compose through this channel rather than inside
// searchProjectionMatchesWithin because the lens path calls that function with
// its own facets already applied, and checking them there would apply them twice.
func (m *OutlinerModel) SearchCandidateBoundary(projects []*models.Project) map[string][]*models.Task {
	if m == nil {
		return nil
	}
	if !m.Focus.Active() && !query.HasActiveCommonFilters(&m.Filter.CommonFilterState) {
		return nil
	}
	return m.searchCandidateBoundary(projects, m.FacetPredicateAt(time.Now()))
}

func (m *OutlinerModel) searchCandidateBoundary(projects []*models.Project, facets func(*models.Task, *models.Project) bool) map[string][]*models.Task {
	focused := m.Focus.Active()
	if !focused && facets == nil {
		return nil
	}
	// buildCanonicalModeRows populates the open-child census above the search
	// branch, so the hierarchy half of the facet gate reads a live count here.
	candidates := make(map[string][]*models.Task, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		tasks := project.Tasks
		if focused {
			tasks = flattenTaskPointers(m.primaryModeRoots(project))
		}
		admitted := make([]*models.Task, 0, len(tasks))
		for _, task := range tasks {
			if task != nil && (facets == nil || facets(task, project)) {
				admitted = append(admitted, task)
			}
		}
		candidates[project.ID] = admitted
	}
	return candidates
}

// compileSearchQuery hands the Outliner's own carriers to the shared gate. The
// gate reads two things this model owns and nothing else does: the durable plan
// that decides Today and Next membership, and the canonical filter query whose
// facets open the terminal and deferred populations beside the sigils.
//
// A nil model still compiles: the sigils answer for themselves, and a view with
// no plan and no facets neither names a planned task nor opens a population.
func (m *OutlinerModel) compileSearchQuery(text string, projects []*models.Project) (string, *query.SearchPredicates) {
	if m == nil {
		return query.CompileSearchQuery(text, projects, nil, models.Query{})
	}
	return query.CompileSearchQuery(text, projects, outlinerPlanningMembership{outliner: m}, m.FilterQuery)
}

// outlinerPlanningMembership is the Outliner's answer to the gate's planning
// question. It is a wrapper rather than a set of methods on the model itself so
// the gate's contract stays the two memberships it names, not whatever else the
// Outliner happens to expose.
type outlinerPlanningMembership struct {
	outliner *OutlinerModel
}

func (p outlinerPlanningMembership) TodayIdentities(projects []*models.Project) map[taskref.TaskIdentity]bool {
	return p.outliner.resolveTodayIDs(projects)
}

func (p outlinerPlanningMembership) NextIdentities(projects []*models.Project) map[taskref.TaskIdentity]bool {
	return resolveSearchNextIDs(p.outliner, projects)
}

// resolveSearchNextIDs reads Next membership from the durable plan, because Next
// and Today both store Backlog tasks and a state compare cannot tell them apart.
func resolveSearchNextIDs(m *OutlinerModel, projects []*models.Project) map[taskref.TaskIdentity]bool {
	ids := make(map[taskref.TaskIdentity]bool)
	if m == nil || m.Runtime == nil {
		return ids
	}
	for _, item := range planningNextOrder(m.Runtime) {
		for _, project := range projects {
			if project == nil || project.ID != item.ProjectID {
				continue
			}
			if task := project.GetTaskByID(item.TaskID); task != nil {
				ids[taskref.For(task, project)] = true
			}
			break
		}
	}
	return ids
}

// outlinerSearchNoteMatch is one scored note from the shell's index. It is the
// note counterpart of outlinerSearchMatch and carries the same two things a
// task match does: the subject and the evidence that ranked it.
type outlinerSearchNoteMatch struct {
	row   notes.LocatedNoteRow
	match notes.NoteSearchMatch
	// sourceIndex is the note's position in the in-scope index under the index's
	// own order — location, then filename. It is the tier's tiebreak exactly as
	// it is for tasks, and it is carried rather than recompared so that ordering
	// is stated once, in sortNoteSearchIndexRows.
	sourceIndex int
}

// setNoteSearchIndex injects the shell's note accessor. It is deliberately a
// setter rather than a constructor parameter: NewOutlinerModel builds the Hub
// preview painter too, and that surface must keep the nil accessor.
func (m *OutlinerModel) SetNoteSearchIndex(accessor func() []notes.LocatedNoteRow) {
	m.Search.setNoteIndex(accessor)
}

// noteSearchRowsInScope applies the Outliner's own project scope to the
// workspace-wide index. A single-project scope yields that project's notes
// alone — not Global — matching the way a scoped Outliner shows only that
// project's work; unscoped yields Global plus every project the index carries.
// Archived owners were already refused when the index was built, so there is
// no archive test here.
func (m *OutlinerModel) noteSearchRowsInScope() []notes.LocatedNoteRow {
	if m == nil || m.Search.noteIndex == nil {
		return nil
	}
	rows := m.Search.noteIndex()
	scopeID := m.Scope.ProjectID
	if m.Project != nil {
		scopeID = m.Project.ID
	}
	if scopeID == "" {
		return rows
	}
	scoped := make([]notes.LocatedNoteRow, 0, len(rows))
	for _, row := range rows {
		if row.Location.Scope == app.NoteScopeProject && row.Location.ProjectID == scopeID {
			scoped = append(scoped, row)
		}
	}
	return scoped
}

// scoreNoteSearchTier ranks the in-scope notes against the free-text remainder
// under the matcher the task pass is using, so one query never relaxes its case
// for tasks and not for notes. admitted is the caller's single decision about
// whether the tier exists for this query at all; the scorer does not re-derive
// it, which is what keeps the sigil rule stated in exactly one place.
//
// ScoreNoteSearch is reused as-is: the Notes manager and this tier must agree
// about what a note matches and how strongly, or the same query would rank the
// same note two ways on two surfaces.
func (m *OutlinerModel) scoreNoteSearchTier(
	query string,
	matcher fuzzy.Matcher,
	admitted bool,
) []outlinerSearchNoteMatch {
	if !admitted {
		return nil
	}
	rows := m.noteSearchRowsInScope()
	if len(rows) == 0 {
		return nil
	}
	// The shell's index arrives in this order already and scoping filters it
	// without reordering, so this is a no-op in production. It is applied here
	// anyway because sourceIndex is what the projection's rows are tiebroken and
	// seated by: a caller that hands the tier an unsorted snapshot must still get
	// the one order notes.SortLocatedNoteRows names, not its slice's.
	rows = slices.Clone(rows)
	notes.SortLocatedNoteRows(rows)
	matches := make([]outlinerSearchNoteMatch, 0, len(rows))
	for sourceIndex, row := range rows {
		match, ok := notes.ScoreNoteSearch(row.Note, query, matcher)
		if !ok {
			continue
		}
		matches = append(matches, outlinerSearchNoteMatch{
			row: row, match: match, sourceIndex: sourceIndex,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		left, right := matches[i], matches[j]
		if left.match.Score != right.match.Score {
			return left.match.Score > right.match.Score
		}
		return left.sourceIndex < right.sourceIndex
	})
	return matches
}

// Explain only removals proven to reveal tasks. Each pass removes one ordinary
// facet and retains every other facet, scope, focus, and typed query clause.
func (m *OutlinerModel) removableFilterSearchHint(projects []*models.Project) string {
	rows := m.rowPredicateQuery()
	names := map[models.QueryField]string{
		models.QueryFieldPriority: "priority", models.QueryFieldTag: "tag",
		models.QueryFieldContext: "context", models.QueryFieldProject: "project",
		models.QueryFieldDue: "due date", models.QueryFieldState: "state",
		models.QueryFieldPinned: "pin", models.QueryFieldReview: "review",
		models.QueryFieldStale: "staleness", models.QueryFieldDeferred: "deferral",
	}
	var hints []string
	seen := make(map[models.QueryField]bool)
	now := time.Now()
	for _, clause := range rows.Clauses {
		field := clause.Field
		name := names[field]
		if name == "" || seen[field] || (field == models.QueryFieldState && query.CompletionAgeOwnsTerminalStates(&m.Filter.CommonFilterState)) {
			continue
		}
		seen[field] = true
		cleared := query.WithoutFields(m.FilterQuery, field)
		text, gate := query.CompileSearchQuery(m.SearchQuery(), projects, outlinerPlanningMembership{outliner: m}, cleared)
		if gate.Invalid() {
			continue
		}
		predicate := m.facetPredicateForRowsAt(query.WithoutFields(rows, field), now)
		candidates := m.searchCandidateBoundary(projects, predicate)
		matcher := fuzzy.SmartCase(text)
		matches := searchProjectionMatchesWithin(projects, text, candidates, matcher, gate)
		if len(matches) == 0 && matcher.CaseSensitive() {
			matches = searchProjectionMatchesWithin(projects, text, candidates, fuzzy.NewMatcher(false), gate)
		}
		if len(matches) == 0 {
			continue
		}
		count := util.CountPlural(len(matches), "match", "matches")
		if query.CommonFilterFacetCount(&m.Filter.CommonFilterState) == 1 {
			return fmt.Sprintf("%s hidden by your %s filter.", count, name)
		}
		hints = append(hints, fmt.Sprintf("Clearing your %s filter reveals %s.", name, count))
	}
	return strings.Join(hints, " ")
}

// maxEliminationClauses bounds the questions an empty result asks. Each clause
// costs at most two passes of the existing gate over the loaded population —
// once alone when it names a stored value, once left out — so a long query
// cannot turn one keystroke into an unbounded walk. A query past the bound
// keeps the plain sentence.
const maxEliminationClauses = 8

// searchEliminationHint explains a ranked list that found nothing by naming the
// sigil clause responsible. A positive tag, context, or project clause that no
// task in the searched population carries at all is named first, since that is
// a value to correct rather than a combination to loosen. Otherwise it reruns
// the gate once per clause with that clause left out and names the one whose
// removal restores the most matches, the first typed winning a tie. It answers
// nothing when no single clause is responsible, when removing the clause would
// leave no query at all, or when active filters own the empty state, since that
// sentence already names its own exits.
func (m *OutlinerModel) searchEliminationHint(projects []*models.Project, candidates map[string][]*models.Task) string {
	if m == nil || !m.ShowsRankedList() || query.HasActiveCommonFilters(&m.Filter.CommonFilterState) {
		return ""
	}
	text := m.SearchQuery()
	remainder, clauses := query.SearchClauses(text)
	if len(clauses) == 0 || len(clauses) > maxEliminationClauses {
		return ""
	}
	plan := outlinerPlanningMembership{outliner: m}

	for _, clause := range clauses {
		if !clause.Vocabulary() {
			continue
		}
		gate := query.CompileSearchClauseAlone(text, projects, plan, clause)
		if gate.Invalid() || len(searchProjectionMatchesWithin(projects, "", candidates, fuzzy.NewMatcher(false), gate)) > 0 {
			continue
		}
		if clause.NamesProject() {
			return fmt.Sprintf("No project matches %s.", clause.Spelling)
		}
		return fmt.Sprintf("No task carries %s.", clause.Spelling)
	}

	// Removing the only clause of a sigil-only query leaves an empty query, which
	// matches everything and explains nothing.
	if remainder == "" && len(clauses) == 1 {
		return ""
	}
	best, bestCount := query.SearchClause{}, 0
	for _, clause := range clauses {
		rest, gate := query.CompileSearchQueryWithout(text, projects, plan, m.FilterQuery, clause)
		if gate.Invalid() {
			continue
		}
		matcher := fuzzy.SmartCase(rest)
		matches := searchProjectionMatchesWithin(projects, rest, candidates, matcher, gate)
		if len(matches) == 0 && matcher.CaseSensitive() {
			matches = searchProjectionMatchesWithin(projects, rest, candidates, fuzzy.NewMatcher(false), gate)
		}
		if len(matches) > bestCount {
			best, bestCount = clause, len(matches)
		}
	}
	if bestCount == 0 {
		return ""
	}
	count := util.Count(bestCount, "task")
	verb, object := "matches", "it"
	if bestCount != 1 {
		verb, object = "match", "them"
	}
	if len(clauses) == 1 {
		return fmt.Sprintf("%s %s %q; %s removes %s.", count, verb, remainder, best.Spelling, object)
	}
	return fmt.Sprintf("%s %s without %s.", count, verb, best.Spelling)
}
