package notes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/route"
)

func (m *NotesViewModel) requestedLocations() []app.NoteLocation {
	locations := make([]app.NoteLocation, 0, len(m.ownerProjects)+1)
	locations = append(locations, app.GlobalNoteLocation())
	for _, project := range m.ownerProjects {
		locations = append(locations, app.ProjectNoteLocation(project.ID))
	}
	return locations
}

// Reload rebuilds the row set from the note collections on disk. The shell
// reaches for it after a note mutation or a ledger restore, which are the two
// events that change the files under this view without passing through it.
func (m *NotesViewModel) Reload() { m.LoadNotes() }

// FocusNote moves the cursor onto one note identity when the current row set
// still holds it, and otherwise leaves the cursor alone. An empty identity is a
// no-op, so a caller that has no particular note to select need not ask first.
func (m *NotesViewModel) FocusNote(identity NoteRowIdentity) {
	if identity.Filename == "" {
		return
	}
	for index, row := range m.VisibleRows {
		if row.Identity() == identity {
			m.Cursor = index
			return
		}
	}
}

func (m *NotesViewModel) LoadNotes() {
	selectedID, preserveSelection := m.selectedIdentity()
	m.LoadedRows = nil
	m.collectionErrors = nil
	m.snapshotError = nil

	if !m.Scope.Valid() {
		m.snapshotError = fmt.Errorf("invalid notes scope %q", m.Scope.Kind)
		m.VisibleRows = nil
		m.Cursor = 0
		m.resetScrollOffset()
		return
	}
	if m.NoteService == nil {
		m.snapshotError = fmt.Errorf("note service is unavailable")
		m.VisibleRows = nil
		m.Cursor = 0
		m.resetScrollOffset()
		return
	}

	results, err := m.NoteService.NotesAtLocations(m.requestedLocations())
	if err != nil {
		m.snapshotError = err
		m.VisibleRows = nil
		m.Cursor = 0
		m.resetScrollOffset()
		return
	}
	for _, result := range results {
		owner := m.OwnerForLocation(result.Location)
		if result.Err != nil {
			m.collectionErrors = append(m.collectionErrors, noteCollectionFailure{
				OwnerLabel: owner.Label,
				Err:        result.Err,
			})
			continue
		}
		for _, note := range result.Notes {
			if note == nil {
				continue
			}
			m.LoadedRows = append(m.LoadedRows, LocatedNoteRow{
				Note:       note,
				Location:   result.Location,
				OwnerLabel: owner.Label,
			})
		}
	}

	m.DeriveVisibleRows()
	m.restoreVisibleSelection(selectedID, preserveSelection)
}

// SetScope derives a new visible list from the canonical loaded rows without
// rebuilding the workspace snapshot.
func (m *NotesViewModel) SetScope(scope route.NotesScope) {
	selectedID, preserveSelection := m.selectedIdentity()
	m.LocationPicker = nil
	m.moveSource = nil
	m.Scope = scope
	m.DeriveVisibleRows()
	m.restoreVisibleSelection(selectedID, preserveSelection)
}

func (m *NotesViewModel) DeriveVisibleRows() {
	m.VisibleRows = m.VisibleRows[:0]
	m.SearchMatches = nil
	m.searchCaseRelaxed = false
	if !m.Scope.Valid() {
		return
	}
	query := strings.TrimSpace(m.activeSearchQuery())

	collect := func(matcher fuzzy.Matcher) {
		m.VisibleRows = m.VisibleRows[:0]
		if query != "" {
			m.SearchMatches = make(map[NoteRowIdentity]NoteSearchMatch)
		}
		for _, row := range m.LoadedRows {
			if !notesScopeMatches(m.Scope, row.Location) {
				continue
			}
			if query != "" {
				match, ok := ScoreNoteSearch(row.Note, query, matcher)
				if !ok {
					continue
				}
				m.SearchMatches[row.Identity()] = match
			}
			m.VisibleRows = append(m.VisibleRows, row)
		}
	}

	matcher := fuzzy.SmartCase(query)
	collect(matcher)
	if len(m.VisibleRows) == 0 && matcher.CaseSensitive() {
		collect(fuzzy.NewMatcher(false))
		m.searchCaseRelaxed = len(m.VisibleRows) > 0
	}

	sort.Slice(m.VisibleRows, func(i, j int) bool {
		return m.visibleRowLess(m.VisibleRows[i], m.VisibleRows[j], query)
	})
}

// visibleRowLess is the visible-row order: search score first when a query is
// active, then recency, then owner and note identity.
func (m *NotesViewModel) visibleRowLess(left, right LocatedNoteRow, query string) bool {
	if query != "" {
		leftMatch := m.SearchMatches[left.Identity()]
		rightMatch := m.SearchMatches[right.Identity()]
		if leftMatch.Score != rightMatch.Score {
			return leftMatch.Score > rightMatch.Score
		}
	}
	if !left.Note.ModifiedAt.Equal(right.Note.ModifiedAt) {
		return left.Note.ModifiedAt.After(right.Note.ModifiedAt)
	}
	if order := strings.Compare(strings.ToLower(left.OwnerLabel), strings.ToLower(right.OwnerLabel)); order != 0 {
		return order < 0
	}
	if query == "" {
		if order := CompareNoteLocations(left.Location, right.Location); order != 0 {
			return order < 0
		}
		if order := strings.Compare(strings.ToLower(left.Note.Title), strings.ToLower(right.Note.Title)); order != 0 {
			return order < 0
		}
		return strings.Compare(strings.ToLower(left.Note.Filename), strings.ToLower(right.Note.Filename)) < 0
	}
	if order := strings.Compare(left.OwnerLabel, right.OwnerLabel); order != 0 {
		return order < 0
	}
	if order := strings.Compare(strings.ToLower(left.Note.Title), strings.ToLower(right.Note.Title)); order != 0 {
		return order < 0
	}
	if order := strings.Compare(left.Note.Title, right.Note.Title); order != 0 {
		return order < 0
	}
	if order := strings.Compare(strings.ToLower(left.Note.Filename), strings.ToLower(right.Note.Filename)); order != 0 {
		return order < 0
	}
	if order := strings.Compare(left.Note.Filename, right.Note.Filename); order != 0 {
		return order < 0
	}
	return CompareNoteLocations(left.Location, right.Location) < 0
}

func (m *NotesViewModel) restoreVisibleSelection(identity NoteRowIdentity, preserve bool) {
	if preserve {
		for i := range m.VisibleRows {
			if m.VisibleRows[i].Identity() == identity {
				m.Cursor = i
				m.resetScrollOffset()
				return
			}
		}
	}
	m.Cursor = chrome.ClampCursor(m.Cursor, len(m.VisibleRows))
	m.resetScrollOffset()
}

func (m *NotesViewModel) RederiveVisibleRows() {
	selectedID, preserveSelection := m.selectedIdentity()
	m.DeriveVisibleRows()
	m.restoreVisibleSelection(selectedID, preserveSelection)
}

func (m *NotesViewModel) activeSearchQuery() string {
	if m.searchEditing {
		return m.searchInput.Value()
	}
	return m.SearchApplied
}

// SearchQuery returns the accepted query. A draft remains private until Enter
// accepts it, even though its result set is rendered live.
func (m *NotesViewModel) SearchQuery() string {
	return m.SearchApplied
}

func (m *NotesViewModel) scopedRowCount() int {
	count := 0
	for _, row := range m.LoadedRows {
		if notesScopeMatches(m.Scope, row.Location) {
			count++
		}
	}
	return count
}

// notesScopeMatches is a function rather than a method on route.NotesScope
// because the scope is navigation identity and app.NoteLocation is a durable
// collection locator; pinning the two together would put internal/app inside
// the route layer for this one predicate.
func notesScopeMatches(scope route.NotesScope, location app.NoteLocation) bool {
	switch scope.Kind {
	case route.NotesScopeAll:
		return true
	case route.NotesScopeGlobal:
		return location.Scope == app.NoteScopeGlobal
	case route.NotesScopeProject:
		return location.Scope == app.NoteScopeProject && location.ProjectID == scope.ProjectID
	default:
		return false
	}
}

func CompareNoteLocations(left, right app.NoteLocation) int {
	if left.Scope != right.Scope {
		return strings.Compare(string(left.Scope), string(right.Scope))
	}
	return strings.Compare(left.ProjectID, right.ProjectID)
}

// SortLocatedNoteRows gives a note collection one deterministic order —
// location, then filename — so a ranking tie resolves the same way on every
// rebuild. It lives beside CompareNoteLocations because the order is the row
// type's own, not a property of whichever surface happens to be listing it.
func SortLocatedNoteRows(rows []LocatedNoteRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if order := CompareNoteLocations(rows[i].Location, rows[j].Location); order != 0 {
			return order < 0
		}
		return rows[i].Note.Filename < rows[j].Note.Filename
	})
}

func (m *NotesViewModel) selectedIdentity() (NoteRowIdentity, bool) {
	row := m.CurrentRow()
	if row == nil {
		return NoteRowIdentity{}, false
	}
	return row.Identity(), true
}

func (m *NotesViewModel) CurrentRow() *LocatedNoteRow {
	if m.Cursor < 0 || m.Cursor >= len(m.VisibleRows) {
		return nil
	}
	return &m.VisibleRows[m.Cursor]
}

func (m *NotesViewModel) rowByIdentity(identity NoteRowIdentity) *LocatedNoteRow {
	for i := range m.VisibleRows {
		if m.VisibleRows[i].Identity() == identity {
			return &m.VisibleRows[i]
		}
	}
	return nil
}
