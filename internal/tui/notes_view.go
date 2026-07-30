package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// NotesViewModel displays and manages project notes.
type NotesViewModel struct {
	BaseView

	project      *models.Project
	notesDir     string // when set, use path-based storage (global notes mode)
	isGlobal     bool   // true when operating on global notes
	combined     bool
	preferred    app.NoteLocation
	locations    map[*models.Note]app.NoteLocation
	noteService  projectNoteService
	noteLedger   *storage.WorkspaceLedger
	notes        []*models.Note
	cursor       int
	scrollOffset int
	cursorFlash  bool

	// Delete confirmation state
	confirm          ConfirmDialog
	pendingDeleteIdx int

	// Details pane state
	detailsPane DetailsPaneState
	listHeight  int

	// Multi-select state
	multiSelectMode bool
	selectedNotes   map[string]bool // keyed by filename

	// Tab state
	tabs TabState

	// Sort state
	sortMode noteSortMode

	// Rename state
	renaming       bool
	renameInput    textinput.Model
	confirmDiscard bool
	confirmOverlay ConfirmDialog

	// Fullscreen pane toggle
	fullscreenPane bool

	// Cached tasks for displaying linked task titles
	tasksLoaded bool
	cachedTasks []*models.Task

	statusMessage string
	statusIsError bool

	// New-note creation always pauses at this explicit destination selector.
	destinationPicker bool
	destinationCursor int
	destinations      []app.NoteLocation
}

// setStatus sets a status message and clears the error flag.
func (m *NotesViewModel) setStatus(msg string) {
	m.statusMessage = msg
	m.statusIsError = false
}

// setErrorStatus sets a status message with error styling.
func (m *NotesViewModel) setErrorStatus(msg string) {
	m.statusMessage = msg
	m.statusIsError = true
}

// clearStatus clears the status message and error flag.
func (m *NotesViewModel) clearStatus() {
	m.statusMessage = ""
	m.statusIsError = false
}

func (m *NotesViewModel) resetScrollOffset() {
	m.scrollOffset = 0
}

// IsMultiSelectActive reports whether multi-select mode is on. NotesViewModel
// has its own multiSelectMode field rather than embedding MultiSelectState, so
// this method exposes it to the global multi-select interface dispatch.
func (m *NotesViewModel) IsMultiSelectActive() bool {
	return m.multiSelectMode
}

// handleConfirmKey processes a key when the confirm dialog is active.
// Returns (confirmed, handled). On cancel, clears the status message.
// Mirrors InteractiveViewState.HandleConfirmKey for views that don't embed it.
func (m *NotesViewModel) handleConfirmKey(key string) (confirmed, handled bool) {
	if !m.confirm.IsActive() {
		return false, false
	}
	c, h := m.confirm.HandleKey(key)
	return c, h
}

// noteSortMode defines how notes are sorted.
type noteSortMode int

const (
	noteSortModified noteSortMode = iota
	noteSortCreated
	noteSortTitleAZ
	noteSortTitleZA
)

func (s noteSortMode) String() string {
	switch s {
	case noteSortModified:
		return "modified"
	case noteSortCreated:
		return "created"
	case noteSortTitleAZ:
		return "title A-Z"
	case noteSortTitleZA:
		return "title Z-A"
	default:
		return "modified"
	}
}

// NewCombinedNotesViewModelWithService creates the one location-aware Notes
// tool. It lists global notes together with notes from the active project (when
// one exists). preferred controls only the initial row in the explicit
// creation destination picker; it never bypasses confirmation.
func NewCombinedNotesViewModelWithService(
	notesDir string,
	project *models.Project,
	preferred app.NoteLocation,
	service projectNoteService,
	ledger *storage.WorkspaceLedger,
) *NotesViewModel {
	m := &NotesViewModel{
		project:       project,
		notesDir:      notesDir,
		combined:      true,
		preferred:     preferred,
		noteService:   service,
		noteLedger:    ledger,
		locations:     make(map[*models.Note]app.NoteLocation),
		selectedNotes: make(map[string]bool),
		tabs:          TabState{Names: []string{"All"}},
	}
	if preferred.Scope == "" {
		m.preferred = app.GlobalNoteLocation()
	}
	m.loadNotes()
	return m
}

func (m *NotesViewModel) noteLocation() app.NoteLocation {
	if m.combined {
		if len(m.notes) > 0 && m.cursor >= 0 && m.cursor < len(m.notes) {
			if location, ok := m.locations[m.notes[m.cursor]]; ok {
				return location
			}
		}
		return m.preferred
	}
	if m.isGlobal {
		return app.GlobalNoteLocation()
	}
	if m.project == nil {
		return app.NoteLocation{}
	}
	return app.ProjectNoteLocation(m.project.ID)
}

func (m *NotesViewModel) noteLocationFor(note *models.Note) app.NoteLocation {
	if m.combined {
		if location, ok := m.locations[note]; ok {
			return location
		}
		return app.NoteLocation{}
	}
	return m.noteLocation()
}

// noteLedgerScope preserves project Notes' project-wide restore behavior while
// restricting Global Notes to transactions that touch its own collection.
func (m *NotesViewModel) noteLedgerScope() storage.LedgerScope {
	if m.combined {
		return storage.GlobalScope()
	}
	if m.isGlobal {
		return storage.ArtifactRootScope(m.notesDir)
	}
	if m.project == nil {
		return storage.GlobalScope()
	}
	return storage.ProjectScope(m.project.ID)
}

// effectiveDir returns the notes directory path for either global or project notes.
func (m *NotesViewModel) effectiveDir() string {
	if m.combined {
		location := m.noteLocation()
		if location.Scope == app.NoteScopeGlobal {
			return m.notesDir
		}
		if location.Scope == app.NoteScopeProject && m.project != nil && m.project.ID == location.ProjectID {
			return storage.NotesDir(m.project)
		}
		return ""
	}
	if m.isGlobal {
		return m.notesDir
	}
	return storage.NotesDir(m.project)
}

func (m *NotesViewModel) loadNotes() {
	var notes []*models.Note
	var err error
	if m.combined {
		m.locations = make(map[*models.Note]app.NoteLocation)
		if m.noteService == nil {
			m.notes = nil
			m.resetScrollOffset()
			return
		}
		global, globalErr := m.noteService.NotesAt(app.GlobalNoteLocation())
		if globalErr != nil {
			m.notes = nil
			m.setErrorStatus(globalErr.Error())
			m.resetScrollOffset()
			return
		}
		for _, note := range global {
			m.locations[note] = app.GlobalNoteLocation()
		}
		notes = append(notes, global...)
		if m.project != nil {
			location := app.ProjectNoteLocation(m.project.ID)
			projectNotes, projectErr := m.noteService.NotesAt(location)
			if projectErr != nil {
				m.notes = nil
				m.setErrorStatus(projectErr.Error())
				m.resetScrollOffset()
				return
			}
			for _, note := range projectNotes {
				m.locations[note] = location
			}
			notes = append(notes, projectNotes...)
		}
	} else if m.noteService != nil && (m.isGlobal || m.project != nil) {
		notes, err = m.noteService.NotesAt(m.noteLocation())
	} else if m.isGlobal {
		notes, err = storage.ListNotesFrom(m.notesDir)
	} else {
		notes, err = storage.ListNotes(m.project)
	}
	if err != nil {
		m.notes = nil
		m.resetScrollOffset()
		return
	}
	// Load project tasks once for displaying linked task titles
	if !m.combined && !m.isGlobal && !m.tasksLoaded {
		if loadErr := storage.LoadProject(m.project); loadErr == nil {
			m.cachedTasks = m.project.Tasks
		}
		m.tasksLoaded = true
	}
	if m.combined {
		sort.SliceStable(notes, func(i, j int) bool {
			return notes[i].ModifiedAt.After(notes[j].ModifiedAt)
		})
	} else {
		m.sortNotes(notes)
	}
	m.notes = notes
	m.cursor = ClampCursor(m.cursor, len(m.notes))
	m.resetScrollOffset()
}

func (m *NotesViewModel) sortNotes(notes []*models.Note) {
	sort.SliceStable(notes, func(i, j int) bool {
		switch m.sortMode {
		case noteSortCreated:
			return notes[i].CreatedAt.After(notes[j].CreatedAt)
		case noteSortTitleAZ:
			return strings.ToLower(notes[i].Title) < strings.ToLower(notes[j].Title)
		case noteSortTitleZA:
			return strings.ToLower(notes[i].Title) > strings.ToLower(notes[j].Title)
		default: // noteSortModified
			return notes[i].ModifiedAt.After(notes[j].ModifiedAt)
		}
	})
}

func (m *NotesViewModel) Init() tea.Cmd {
	return nil
}

func (m *NotesViewModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.resetScrollOffset()
	// Reserve 4 for header (title + tab bar + count + blank line), 1 for status bar
	usable := max(height-4, 4)
	m.listHeight = usable / 2
	m.detailsPane.Height = usable - m.listHeight - 1 // -1 for divider
}

// ShowGlobalNoteCreatorMsg opens a note creator in global notes mode.
type ShowGlobalNoteCreatorMsg struct{}

func (m *NotesViewModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
		return &m.confirmOverlay
	}
	m.confirmOverlay.Cancel()
	return &m.confirm
}

func (m *NotesViewModel) StatusBarLabel() string {
	if m.isGlobal {
		return "Global Notes"
	}
	return "Notes"
}

// IsFullHeight returns true because the notes view renders its own footer.
func (m *NotesViewModel) IsFullHeight() bool { return true }
