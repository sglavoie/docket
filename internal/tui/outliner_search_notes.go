package tui

import (
	"slices"
	"sort"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/notes"
)

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
func (m *OutlinerModel) setNoteSearchIndex(accessor func() []notes.LocatedNoteRow) {
	m.noteSearchIndex = accessor
}

// noteSearchRowsInScope applies the Outliner's own project scope to the
// workspace-wide index. A single-project scope yields that project's notes
// alone — not Global — matching the way a scoped Outliner shows only that
// project's work; unscoped yields Global plus every project the index carries.
// Archived owners were already refused when the index was built, so there is
// no archive test here.
func (m *OutlinerModel) noteSearchRowsInScope() []notes.LocatedNoteRow {
	if m == nil || m.noteSearchIndex == nil {
		return nil
	}
	rows := m.noteSearchIndex()
	scopeID := m.scopedProjectID
	if m.project != nil {
		scopeID = m.project.ID
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
	// the one order sortNoteSearchIndexRows names, not its slice's.
	rows = slices.Clone(rows)
	sortNoteSearchIndexRows(rows)
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
