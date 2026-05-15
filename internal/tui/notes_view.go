package tui

import (
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// ShowNoteTagSelectorMsg opens the tag selector for a note.
type ShowNoteTagSelectorMsg struct {
	Note        *models.Note
	Project     *models.Project
	NotesDir    string
	IsGlobal    bool
	CurrentTags []string
}

// NoteTagSelectorDoneMsg returns the result from the note tag selector.
type NoteTagSelectorDoneMsg struct {
	Note         *models.Note
	Project      *models.Project
	NotesDir     string
	IsGlobal     bool
	SelectedTags []string
	Canceled     bool
}

// NotesViewModel displays and manages project notes.
type NotesViewModel struct {
	BaseView

	project  *models.Project
	notesDir string // when set, use path-based storage (global notes mode)
	isGlobal bool   // true when operating on global notes
	config   interface {
		GetEditor() string
		GetNotesEditor() string
	}

	notes       []*models.Note
	cursor      int
	cursorFlash bool

	// Filter state
	filterTags map[string]bool

	// Delete confirmation state
	confirm          ConfirmDialog
	pendingDeleteIdx int

	// Undo state
	lastDeleted     *models.Note
	lastBulkDeleted []*models.Note
	lastTagUndo     []noteTagUndoEntry
	lastLinkUndo    []noteLinkUndoEntry

	// Details pane state
	detailsPane DetailsPaneState
	listHeight  int

	// Multi-select state
	multiSelectMode bool
	selectedNotes   map[string]bool // keyed by filename
	pendingPrefix   string          // waiting for second key after prefix (e.g. "g")

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

// Tab constants for notes view filtering.
var noteTabNames = []string{"All", "Pinned", "Recent", "Tagged"}

const (
	noteTabPinned = 1
	noteTabRecent = 2
	noteTabTagged = 3
)

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

// noteTagUndoEntry stores a note's previous tags for undo.
type noteTagUndoEntry struct {
	Note    *models.Note
	OldTags []string
}

// noteLinkUndoEntry stores a note's previous links for undo.
type noteLinkUndoEntry struct {
	Note     *models.Note
	OldLinks []string
}

// notesConfig is a minimal interface for config access.
type notesConfig interface {
	GetEditor() string
	GetNotesEditor() string
}

// NewNotesViewModel creates a new notes view.
func NewNotesViewModel(project *models.Project, cfg ...notesConfig) *NotesViewModel {
	m := &NotesViewModel{
		project:       project,
		filterTags:    make(map[string]bool),
		selectedNotes: make(map[string]bool),
		tabs:          TabState{Names: noteTabNames},
	}
	if len(cfg) > 0 && cfg[0] != nil {
		m.config = cfg[0]
	}
	m.loadNotes()
	return m
}

// NewGlobalNotesViewModel creates a notes view for global notes.
func NewGlobalNotesViewModel(notesDir string, cfg ...notesConfig) *NotesViewModel {
	m := &NotesViewModel{
		notesDir:      notesDir,
		isGlobal:      true,
		filterTags:    make(map[string]bool),
		selectedNotes: make(map[string]bool),
		tabs:          TabState{Names: noteTabNames},
	}
	if len(cfg) > 0 && cfg[0] != nil {
		m.config = cfg[0]
	}
	m.loadNotes()
	return m
}

// effectiveDir returns the notes directory path for either global or project notes.
func (m *NotesViewModel) effectiveDir() string {
	if m.isGlobal {
		return m.notesDir
	}
	return storage.NotesDir(m.project)
}

func (m *NotesViewModel) loadNotes() {
	var notes []*models.Note
	var err error
	if m.isGlobal {
		notes, err = storage.ListNotesFrom(m.notesDir)
	} else {
		notes, err = storage.ListNotes(m.project)
	}
	if err != nil {
		m.notes = nil
		return
	}
	// Load project tasks once for displaying linked task titles
	if !m.isGlobal && !m.tasksLoaded {
		if loadErr := storage.LoadProject(m.project); loadErr == nil {
			m.cachedTasks = m.project.Tasks
		}
		m.tasksLoaded = true
	}
	if len(m.filterTags) > 0 {
		var filtered []*models.Note
		for _, n := range notes {
			for _, t := range n.Tags {
				if m.filterTags[t] {
					filtered = append(filtered, n)
					break
				}
			}
		}
		notes = filtered
	}
	// Apply tab filter
	switch m.tabs.Active {
	case noteTabPinned:
		var filtered []*models.Note
		for _, n := range notes {
			if n.Pinned {
				filtered = append(filtered, n)
			}
		}
		notes = filtered
	case noteTabRecent:
		var filtered []*models.Note
		for _, n := range notes {
			if time.Since(n.ModifiedAt) < util.Week {
				filtered = append(filtered, n)
			}
		}
		notes = filtered
	case noteTabTagged:
		var filtered []*models.Note
		for _, n := range notes {
			if len(n.Tags) > 0 {
				filtered = append(filtered, n)
			}
		}
		notes = filtered
	}
	// Apply sort mode (pinned notes always first)
	m.sortNotes(notes)
	m.notes = notes
	m.cursor = ClampCursor(m.cursor, len(m.notes))
}

func (m *NotesViewModel) sortNotes(notes []*models.Note) {
	sort.SliceStable(notes, func(i, j int) bool {
		if notes[i].Pinned != notes[j].Pinned {
			return notes[i].Pinned
		}
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
	// Reserve 4 for header (title + tab bar + count + blank line), 1 for status bar
	usable := max(height-4, 4)
	m.listHeight = usable / 2
	m.detailsPane.Height = usable - m.listHeight - 1 // -1 for divider
}

// ShowGlobalNoteCreatorMsg opens a note creator in global notes mode.
type ShowGlobalNoteCreatorMsg struct {
	NotesDir string
}

// ShowGlobalNoteMoveMsg requests a project picker to move a global note.
type ShowGlobalNoteMoveMsg struct {
	Note      *models.Note
	SourceDir string
}

// GlobalNoteMoveCompleteMsg indicates a global note was moved to a project.
type GlobalNoteMoveCompleteMsg struct {
	NoteTitle   string
	ProjectName string
}

func (m *NotesViewModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
		return &m.confirmOverlay
	}
	m.confirmOverlay.Cancel()
	return &m.confirm
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *NotesViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	km := m.buildKeyMap()
	d := keymap.NewDispatcher(km)
	d.SetPendingPrefix(m.pendingPrefix)
	rawHints := d.PrefixHints()
	hints := make([]PrefixHint, 0, len(rawHints))
	for _, h := range rawHints {
		hints = append(hints, PrefixHint{Key: h.Key, Description: h.Description})
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  PrefixHints{m.pendingPrefix: hints},
	}
}

func (m *NotesViewModel) StatusBarLabel() string {
	if m.isGlobal {
		return "Global Notes"
	}
	return "Notes"
}

// IsFullHeight returns true because the notes view renders its own footer.
func (m *NotesViewModel) IsFullHeight() bool { return true }
