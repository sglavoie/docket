package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

const noteTitleSearchWeight = 2

type noteSearchMatch struct {
	score          int
	titlePositions []int
	bodyMatched    bool
}

func scoreNoteSearch(note *models.Note, query string) (noteSearchMatch, bool) {
	if note == nil {
		return noteSearchMatch{}, false
	}

	titleScore, titlePositions := fuzzy.ScoreTermsWithPositions(note.Title, query)
	if titleScore > 0 {
		titleScore *= noteTitleSearchWeight
	} else {
		titlePositions = nil
	}
	bodyScore := scoreNoteBodyTerms(note.Body, query)
	score := max(titleScore, bodyScore)
	if score == 0 {
		return noteSearchMatch{}, false
	}
	return noteSearchMatch{
		score:          score,
		titlePositions: titlePositions,
		bodyMatched:    bodyScore > 0,
	}, true
}

// scoreNoteBodyTerms checks each term independently so a multi-term query can
// match terms in different parts of a long note body. Contiguous,
// case-insensitive matches receive a predictable score; the fuzzy scorer is a
// fallback for terms that are present as a sufficiently dense subsequence.
func scoreNoteBodyTerms(body, query string) int {
	query = strings.TrimSpace(query)
	if query == "" {
		return 1
	}

	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return 1
	}

	lowerBody := strings.ToLower(body)
	total := 0
	for _, term := range terms {
		if strings.Contains(lowerBody, term) {
			total += len([]rune(term)) * 4
			continue
		}

		score := fuzzy.Score(body, term)
		if score == 0 {
			return 0
		}
		total += score
	}
	return total
}

func (m *NotesViewModel) beginSearch() tea.Cmd {
	if m.searchInput.CharLimit == 0 {
		m.initSearchInput()
	}
	m.searchBeforeEdit = m.searchApplied
	m.searchInput.SetValue(m.searchApplied)
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
		m.rederiveVisibleRows()
	}
	return cmd
}

func (m *NotesViewModel) acceptSearch() {
	m.searchApplied = strings.TrimSpace(m.searchInput.Value())
	m.searchBeforeEdit = m.searchApplied
	m.searchInput.SetValue(m.searchApplied)
	m.searchInput.Blur()
	m.searchEditing = false
	m.rederiveVisibleRows()
}

func (m *NotesViewModel) cancelSearchEdit() {
	m.searchApplied = m.searchBeforeEdit
	m.searchInput.SetValue(m.searchApplied)
	m.searchInput.Blur()
	m.searchEditing = false
	m.rederiveVisibleRows()
}

func (m *NotesViewModel) clearSearch() {
	m.searchApplied = ""
	m.searchBeforeEdit = ""
	m.searchInput.SetValue("")
	m.searchInput.Blur()
	m.searchEditing = false
	m.rederiveVisibleRows()
}
