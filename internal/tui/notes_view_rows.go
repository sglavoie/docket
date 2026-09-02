package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
)

func (m *NotesViewModel) requestedLocations() []app.NoteLocation {
	locations := make([]app.NoteLocation, 0, len(m.ownerProjects)+1)
	locations = append(locations, app.GlobalNoteLocation())
	for _, project := range m.ownerProjects {
		locations = append(locations, app.ProjectNoteLocation(project.ID))
	}
	return locations
}

func (m *NotesViewModel) loadNotes() {
	selectedID, preserveSelection := m.selectedIdentity()
	m.loadedRows = nil
	m.collectionErrors = nil
	m.snapshotError = nil

	if !m.scope.valid() {
		m.snapshotError = fmt.Errorf("invalid notes scope %q", m.scope.Kind)
		m.visibleRows = nil
		m.cursor = 0
		m.resetScrollOffset()
		return
	}
	if m.noteService == nil {
		m.snapshotError = fmt.Errorf("note service is unavailable")
		m.visibleRows = nil
		m.cursor = 0
		m.resetScrollOffset()
		return
	}

	results, err := m.noteService.NotesAtLocations(m.requestedLocations())
	if err != nil {
		m.snapshotError = err
		m.visibleRows = nil
		m.cursor = 0
		m.resetScrollOffset()
		return
	}
	for _, result := range results {
		owner := m.ownerForLocation(result.Location)
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
			m.loadedRows = append(m.loadedRows, LocatedNoteRow{
				Note:       note,
				Location:   result.Location,
				OwnerLabel: owner.Label,
			})
		}
	}

	m.deriveVisibleRows()
	m.restoreVisibleSelection(selectedID, preserveSelection)
}

// SetScope derives a new visible list from the canonical loaded rows without
// rebuilding the workspace snapshot.
func (m *NotesViewModel) SetScope(scope NotesScope) {
	selectedID, preserveSelection := m.selectedIdentity()
	m.locationPicker = nil
	m.moveSource = nil
	m.scope = scope
	m.deriveVisibleRows()
	m.restoreVisibleSelection(selectedID, preserveSelection)
}

func (m *NotesViewModel) deriveVisibleRows() {
	m.visibleRows = m.visibleRows[:0]
	m.searchMatches = nil
	m.searchCaseRelaxed = false
	if !m.scope.valid() {
		return
	}
	query := strings.TrimSpace(m.activeSearchQuery())

	collect := func(matcher fuzzy.Matcher) {
		m.visibleRows = m.visibleRows[:0]
		if query != "" {
			m.searchMatches = make(map[NoteRowIdentity]noteSearchMatch)
		}
		for _, row := range m.loadedRows {
			if !m.scope.matches(row.Location) {
				continue
			}
			if query != "" {
				match, ok := scoreNoteSearch(row.Note, query, matcher)
				if !ok {
					continue
				}
				m.searchMatches[row.Identity()] = match
			}
			m.visibleRows = append(m.visibleRows, row)
		}
	}

	matcher := fuzzy.SmartCase(query)
	collect(matcher)
	if len(m.visibleRows) == 0 && matcher.CaseSensitive() {
		collect(fuzzy.NewMatcher(false))
		m.searchCaseRelaxed = len(m.visibleRows) > 0
	}

	sort.Slice(m.visibleRows, func(i, j int) bool {
		return m.visibleRowLess(m.visibleRows[i], m.visibleRows[j], query)
	})
}

// visibleRowLess is the visible-row order: search score first when a query is
// active, then recency, then owner and note identity.
func (m *NotesViewModel) visibleRowLess(left, right LocatedNoteRow, query string) bool {
	if query != "" {
		leftMatch := m.searchMatches[left.Identity()]
		rightMatch := m.searchMatches[right.Identity()]
		if leftMatch.score != rightMatch.score {
			return leftMatch.score > rightMatch.score
		}
	}
	if !left.Note.ModifiedAt.Equal(right.Note.ModifiedAt) {
		return left.Note.ModifiedAt.After(right.Note.ModifiedAt)
	}
	if order := strings.Compare(strings.ToLower(left.OwnerLabel), strings.ToLower(right.OwnerLabel)); order != 0 {
		return order < 0
	}
	if query == "" {
		if order := compareNoteLocations(left.Location, right.Location); order != 0 {
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
	return compareNoteLocations(left.Location, right.Location) < 0
}

func (m *NotesViewModel) restoreVisibleSelection(identity NoteRowIdentity, preserve bool) {
	if preserve {
		for i := range m.visibleRows {
			if m.visibleRows[i].Identity() == identity {
				m.cursor = i
				m.resetScrollOffset()
				return
			}
		}
	}
	m.cursor = ClampCursor(m.cursor, len(m.visibleRows))
	m.resetScrollOffset()
}

func (m *NotesViewModel) rederiveVisibleRows() {
	selectedID, preserveSelection := m.selectedIdentity()
	m.deriveVisibleRows()
	m.restoreVisibleSelection(selectedID, preserveSelection)
}

func (m *NotesViewModel) activeSearchQuery() string {
	if m.searchEditing {
		return m.searchInput.Value()
	}
	return m.searchApplied
}

// SearchQuery returns the accepted query. A draft remains private until Enter
// accepts it, even though its result set is rendered live.
func (m *NotesViewModel) SearchQuery() string {
	return m.searchApplied
}

func (m *NotesViewModel) scopedRowCount() int {
	count := 0
	for _, row := range m.loadedRows {
		if m.scope.matches(row.Location) {
			count++
		}
	}
	return count
}

func (s NotesScope) matches(location app.NoteLocation) bool {
	switch s.Kind {
	case NotesScopeAll:
		return true
	case NotesScopeGlobal:
		return location.Scope == app.NoteScopeGlobal
	case NotesScopeProject:
		return location.Scope == app.NoteScopeProject && location.ProjectID == s.ProjectID
	default:
		return false
	}
}

func compareNoteLocations(left, right app.NoteLocation) int {
	if left.Scope != right.Scope {
		return strings.Compare(string(left.Scope), string(right.Scope))
	}
	return strings.Compare(left.ProjectID, right.ProjectID)
}

func (m *NotesViewModel) selectedIdentity() (NoteRowIdentity, bool) {
	row := m.currentRow()
	if row == nil {
		return NoteRowIdentity{}, false
	}
	return row.Identity(), true
}

func (m *NotesViewModel) currentRow() *LocatedNoteRow {
	if m.cursor < 0 || m.cursor >= len(m.visibleRows) {
		return nil
	}
	return &m.visibleRows[m.cursor]
}

func (m *NotesViewModel) rowByIdentity(identity NoteRowIdentity) *LocatedNoteRow {
	for i := range m.visibleRows {
		if m.visibleRows[i].Identity() == identity {
			return &m.visibleRows[i]
		}
	}
	return nil
}
