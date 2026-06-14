package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ShowNotesSearchMsg opens the notes search view.
type ShowNotesSearchMsg struct {
	Project *models.Project
}

// CloseNotesSearchMsg closes the notes search view.
type CloseNotesSearchMsg struct{ PopViewMsg }

// NotesSearchSelectMsg selects a note from the search results.
type NotesSearchSelectMsg struct {
	Note    *models.Note
	Project *models.Project
}

// notesSearchResult holds a note with match positions for highlighting.
type notesSearchResult struct {
	note           *models.Note
	score          int
	titlePositions []int
	bodySnippet    string // snippet of matched body content
	bodyPositions  []int  // match positions within bodySnippet for highlighting
}

// NotesSearchModel provides search across notes in a project.
type NotesSearchModel struct {
	project     *models.Project
	tasksLoaded bool
	notes       []*models.Note

	filtered     []notesSearchResult
	cursor       int
	scrollOffset int

	searchInput textinput.Model

	width        int
	height       int
	listWidth    int
	previewWidth int

	stale bool // set true when notes are mutated; reloads on next activation
}

// NewNotesSearchModel creates a new notes search model.
func NewNotesSearchModel(project *models.Project) *NotesSearchModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search notes..."
	searchInput.CharLimit = 100
	searchInput.SetWidth(40)
	searchInput.Focus()
	applyThemedTextInput(&searchInput)

	notes, _ := storage.ListNotes(project)

	m := &NotesSearchModel{
		project:     project,
		notes:       notes,
		searchInput: searchInput,
	}
	// Load project tasks for displaying linked task titles
	if loadErr := storage.LoadProject(project); loadErr == nil {
		m.tasksLoaded = true
	}
	m.filterNotes()
	return m
}

// MarkStale flags the search index for reload on next activation.
func (m *NotesSearchModel) MarkStale() {
	m.stale = true
}

// RefreshIfStale reloads notes from storage if the index is stale.
func (m *NotesSearchModel) RefreshIfStale() {
	if !m.stale {
		return
	}
	m.stale = false
	m.notes, _ = storage.ListNotes(m.project)
	m.filterNotes()
}

// HasTextInput returns true because search always has input focus.
func (m *NotesSearchModel) HasTextInput() bool {
	return true
}

// SetSize sets the view dimensions.
func (m *NotesSearchModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.resetScrollOffset()

	innerWidth := max(width-10, 1)

	m.listWidth = max(innerWidth*50/100, 1)
	m.previewWidth = max(innerWidth-m.listWidth-1, 1)

	m.searchInput.SetWidth(max(innerWidth-4, 1))
}

func (m *NotesSearchModel) resetScrollOffset() {
	m.scrollOffset = 0
}

// Init initializes the model.
func (m *NotesSearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *NotesSearchModel) StatusBarLabel() string { return "Notes Search" }
