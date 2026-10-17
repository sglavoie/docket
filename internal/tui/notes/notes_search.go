package notes

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

const noteTitleSearchWeight = 2

// NoteSearchMatch is one note's ranking against a query. Score and
// TitlePositions are exported because the Outliner's notes tier ranks and
// highlights with the same numbers this manager does; whether the body matched
// is this package's own rendering cue.
type NoteSearchMatch struct {
	Score          int
	TitlePositions []int
	BodyMatched    bool
}

func ScoreNoteSearch(note *models.Note, query string, matcher fuzzy.Matcher, caches ...*fuzzy.ScoreCache) (NoteSearchMatch, bool) {
	if note == nil {
		return NoteSearchMatch{}, false
	}

	titleScore, titlePositions := matcher.ScoreTermsWithPositions(note.Title, query)
	if titleScore > 0 {
		titleScore *= noteTitleSearchWeight
	} else {
		titlePositions = nil
	}
	bodyScore := scoreNoteBodyTerms(note.Body, query, matcher, caches...)
	score := max(titleScore, bodyScore)
	if score == 0 {
		return NoteSearchMatch{}, false
	}
	return NoteSearchMatch{
		Score:          score,
		TitlePositions: titlePositions,
		BodyMatched:    bodyScore > 0,
	}, true
}

// scoreNoteBodyTerms checks each term independently so a multi-term query can
// match terms in different parts of a long note body. Contiguous matches under
// the matcher's case sensitivity receive a predictable score; the fuzzy scorer
// is a fallback for terms that are present as a sufficiently dense subsequence.
func scoreNoteBodyTerms(body, query string, matcher fuzzy.Matcher, caches ...*fuzzy.ScoreCache) int {
	query = strings.TrimSpace(query)
	if query == "" {
		return 1
	}

	haystack, terms := strings.ToLower(body), strings.Fields(strings.ToLower(query))
	if matcher.CaseSensitive() {
		haystack, terms = body, strings.Fields(query)
	}
	if len(terms) == 0 {
		return 1
	}

	total := 0
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			total += len([]rune(term)) * 4
			continue
		}

		compiled := matcher.CompileTerms(term)
		if len(caches) > 0 {
			compiled = compiled.WithCache(caches[0])
		}
		score := compiled.Score(body)
		if score == 0 {
			return 0
		}
		total += score
	}
	return total
}

func (m *NotesViewModel) BeginSearch() tea.Cmd {
	if m.searchInput.CharLimit == 0 {
		m.initSearchInput()
	}
	m.searchBeforeEdit = m.SearchApplied
	m.searchInput.SetValue(m.SearchApplied)
	m.searchInput.CursorEnd()
	m.searchInput.Focus()
	m.searchEditing = true
	return textinput.Blink
}

func (m *NotesViewModel) updateSearchDraft(msg tea.Msg) tea.Cmd {
	oldValue := m.searchInput.Value()
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	if m.searchInput.Value() != oldValue {
		m.RederiveVisibleRows()
	}
	return cmd
}

func (m *NotesViewModel) acceptSearch() {
	m.SearchApplied = strings.TrimSpace(m.searchInput.Value())
	m.searchBeforeEdit = m.SearchApplied
	m.searchInput.SetValue(m.SearchApplied)
	m.searchInput.Blur()
	m.searchEditing = false
	m.RederiveVisibleRows()
}

func (m *NotesViewModel) cancelSearchEdit() {
	m.SearchApplied = m.searchBeforeEdit
	m.searchInput.SetValue(m.SearchApplied)
	m.searchInput.Blur()
	m.searchEditing = false
	m.RederiveVisibleRows()
}

func (m *NotesViewModel) clearSearch() {
	m.SearchApplied = ""
	m.searchBeforeEdit = ""
	m.searchInput.SetValue("")
	m.searchInput.Blur()
	m.searchEditing = false
	m.RederiveVisibleRows()
}
