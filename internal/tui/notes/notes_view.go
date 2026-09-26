package notes

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// NoteRowIdentity is the stable workspace identity of a note row. Filenames
// alone are deliberately insufficient because owners may use the same name.
type NoteRowIdentity struct {
	Location app.NoteLocation
	Filename string
}

// LocatedNoteRow carries the note together with its durable collection and
// resolved owner metadata.
type LocatedNoteRow struct {
	Note       *models.Note
	Location   app.NoteLocation
	OwnerLabel string
}

func (r LocatedNoteRow) Identity() NoteRowIdentity {
	filename := ""
	if r.Note != nil {
		filename = r.Note.Filename
	}
	return NoteRowIdentity{Location: r.Location, Filename: filename}
}

type noteManagerOwner struct {
	Label     string
	Directory string
	Project   *models.Project
}

type noteCollectionFailure struct {
	OwnerLabel string
	Err        error
}

// NotesViewModel displays and manages notes across the complete active
// workspace.
type NotesViewModel struct {
	listsurface.BaseView

	notesDir    string
	Scope       route.NotesScope
	NoteService ProjectNoteService
	NoteLedger  *storage.WorkspaceLedger

	Owners           map[app.NoteLocation]noteManagerOwner
	ownerProjects    []*models.Project
	LoadedRows       []LocatedNoteRow
	VisibleRows      []LocatedNoteRow
	collectionErrors []noteCollectionFailure
	snapshotError    error
	SearchMatches    map[NoteRowIdentity]NoteSearchMatch

	searchEditing    bool
	SearchApplied    string
	searchBeforeEdit string
	searchInput      textinput.Model

	// searchCaseRelaxed records that the current visible rows came from a
	// case-insensitive retry after the query's smart-case reading matched
	// nothing.
	searchCaseRelaxed bool

	Cursor       int
	ScrollOffset int
	cursorFlash  bool

	// preview is the wide-layout body pane's line cache and scroll position. It
	// is keyed on the note, its revision, and the pane width, so cursor moves,
	// reloads, and resizes reopen it at the top by construction.
	preview notesPreviewState

	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the manager keeps its own one-key prefix.
	pendingTopChord bool

	Confirm       chrome.ConfirmDialog
	PendingDelete NoteRowIdentity

	renaming       bool
	renameTarget   NoteRowIdentity
	renameInput    textinput.Model
	confirmDiscard bool
	confirmOverlay chrome.ConfirmDialog

	StatusMessage string
	StatusIsError bool

	LocationPicker *notesLocationPicker
	moveSource     *NoteMutationRef

	registry *operation.OperationRegistry
	dispatch operation.KeyDispatch
}

// SetOperationSystem hands the manager the descriptor authority and the shell's
// key dispatch seam. Without them the browse grammar resolves nothing and `?`
// documents nothing, so a host that mounts this model must supply both. The
// executor stays out because the Space sheet is shell-owned.
func (m *NotesViewModel) SetOperationSystem(
	registry *operation.OperationRegistry,
	dispatch operation.KeyDispatch,
) {
	m.registry = registry
	m.dispatch = dispatch
}

// NewNotesViewModelWithService constructs the workspace-wide Notes manager.
// projects must be the note owners returned by Runtime.AvailableProjects().
func NewNotesViewModelWithService(
	notesDir string,
	scope route.NotesScope,
	service ProjectNoteService,
	ledger *storage.WorkspaceLedger,
	projects ...*models.Project,
) *NotesViewModel {
	m := &NotesViewModel{
		notesDir:    notesDir,
		Scope:       scope,
		NoteService: service,
		NoteLedger:  ledger,
	}
	m.initSearchInput()
	m.setOwnerProjects(projects)
	m.LoadNotes()
	return m
}

func (m *NotesViewModel) initSearchInput() {
	m.searchInput = textinput.New()
	m.searchInput.Placeholder = "Search note titles and bodies..."
	m.searchInput.CharLimit = 200
	m.searchInput.SetWidth(listsurface.SearchInputWidth(m.Width()))
	theme.ApplyThemedTextInput(&m.searchInput)
}

// setOwnerProjects rebuilds the owner catalog.
func (m *NotesViewModel) setOwnerProjects(projects []*models.Project) {
	m.Owners, m.ownerProjects = NoteOwnerCatalog(m.notesDir, projects)
}

// NoteOwnerCatalog resolves the note owners a workspace-wide read covers:
// Global plus every supplied project, in the supplied order. Archival is the
// only reason a project is refused — a parked owner keeps its notes reachable,
// so the parked tag is deliberately not consulted here. The Notes manager and
// the Outliner's search-tier note index both admit owners through this one
// function, so neither can drift from the other's exclusion rule.
func NoteOwnerCatalog(
	notesDir string,
	projects []*models.Project,
) (map[app.NoteLocation]noteManagerOwner, []*models.Project) {
	seen := make(map[string]bool, len(projects))
	owned := make([]*models.Project, 0, len(projects))
	owners := make(map[app.NoteLocation]noteManagerOwner, len(projects)+1)
	owners[app.GlobalNoteLocation()] = noteManagerOwner{
		Label:     "Global",
		Directory: notesDir,
	}
	for _, project := range projects {
		if project == nil || project.Archived ||
			strings.TrimSpace(project.ID) == "" || seen[project.ID] {
			continue
		}
		seen[project.ID] = true
		owned = append(owned, project)
		location := app.ProjectNoteLocation(project.ID)
		label := strings.TrimSpace(project.Name)
		if label == "" {
			label = project.ID
		}
		owners[location] = noteManagerOwner{
			Label:     label,
			Directory: storage.NotesDir(project),
			Project:   project,
		}
	}
	return owners, owned
}

// RefreshWorkspace replaces the owner catalog from the latest availability
// snapshot, removes stale owners from any open picker, and reloads the canonical
// note rows. It reports whether a project scope became unavailable and was
// normalized to All Notes so the shell can replace its typed route immediately.
func (m *NotesViewModel) RefreshWorkspace(projects ...*models.Project) bool {
	m.setOwnerProjects(projects)
	scopeInvalid := m.Scope.Kind == route.NotesScopeProject &&
		!m.LocationAvailable(app.ProjectNoteLocation(m.Scope.ProjectID))
	if scopeInvalid {
		m.Scope = route.AllNotesScope()
	}
	m.refreshLocationPicker()
	m.LoadNotes()
	return scopeInvalid
}

func (m *NotesViewModel) LocationAvailable(location app.NoteLocation) bool {
	_, ok := m.Owners[location]
	return ok
}

func (m *NotesViewModel) refreshLocationPicker() {
	if m.LocationPicker == nil {
		return
	}

	purpose := m.LocationPicker.Purpose
	query := m.LocationPicker.SearchQuery()
	selectedID := m.LocationPicker.SelectedItem().ItemID()
	excluded := app.NoteLocation{}
	if purpose == NotesLocationPickerMove {
		if m.moveSource == nil || !m.moveSource.Valid() ||
			!m.LocationAvailable(m.moveSource.Location) {
			m.LocationPicker = nil
			m.moveSource = nil
			m.SetErrorStatus("Move canceled: the source location is no longer available")
			return
		}
		excluded = m.moveSource.Location
	}

	picker := newNotesLocationPicker(
		purpose,
		m.Scope,
		excluded,
		m.ownerProjects,
	)
	if purpose == NotesLocationPickerMove && len(picker.SourceItems()) == 0 {
		m.LocationPicker = nil
		m.moveSource = nil
		m.SetStatus("No other note locations are available; note was not moved")
		return
	}
	picker.SetSize(m.Width(), m.Height())
	picker.SetAccentColor(m.AccentColor())
	if query != "" {
		picker.SetSearchInputValue(query)
		picker.SetSearchQuery(query)
		picker.alwaysSearch.FuzzyFilter(query)
	}
	if selectedID != "" {
		picker.SetCursorToID(selectedID)
	}
	m.LocationPicker = picker
}

func (m *NotesViewModel) OwnerForLocation(location app.NoteLocation) noteManagerOwner {
	if owner, ok := m.Owners[location]; ok {
		return owner
	}
	label := location.ProjectID
	if location.Scope == app.NoteScopeGlobal {
		label = "Global"
	}
	if label == "" {
		label = "Unknown"
	}
	return noteManagerOwner{Label: label}
}

// ProjectByID answers the owner catalog this view already built. The shell
// reaches for it while restoring a ledger entry, because a project the runtime
// catalog no longer lists can still own the notes on screen.
func (m *NotesViewModel) ProjectByID(projectID string) *models.Project {
	owner, ok := m.Owners[app.ProjectNoteLocation(projectID)]
	if !ok {
		return nil
	}
	return owner.Project
}

func (m *NotesViewModel) effectiveDir(row *LocatedNoteRow) string {
	if row == nil {
		return ""
	}
	return m.OwnerForLocation(row.Location).Directory
}

func (m *NotesViewModel) noteLedgerScope() storage.LedgerScope {
	return storage.GlobalScope()
}

func (m *NotesViewModel) SetStatus(msg string) {
	m.StatusMessage = msg
	m.StatusIsError = false
}

func (m *NotesViewModel) SetErrorStatus(msg string) {
	m.StatusMessage = msg
	m.StatusIsError = true
}

func (m *NotesViewModel) clearStatus() {
	m.StatusMessage = ""
	m.StatusIsError = false
}

func (m *NotesViewModel) resetScrollOffset() {
	m.ScrollOffset = 0
}

func (m *NotesViewModel) handleConfirmKey(key string) (confirmed, handled bool) {
	if !m.Confirm.IsActive() {
		return false, false
	}
	return m.Confirm.HandleKey(key)
}

func (m *NotesViewModel) Init() tea.Cmd {
	if m.LocationPicker != nil {
		return m.LocationPicker.Init()
	}
	if m.searchEditing {
		m.searchInput.Focus()
		return textinput.Blink
	}
	return nil
}

func (m *NotesViewModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.searchInput.SetWidth(listsurface.SearchInputWidth(width))
	if m.LocationPicker != nil {
		m.LocationPicker.SetSize(width, height)
	}
	m.resetScrollOffset()
	// A resize can change the pane width, the row budget, or whether the preview
	// exists at all, and every one of those makes a manual offset describe lines
	// the reader can no longer see. Dropping the cache reopens at the top, which
	// is the only position that is correct under all three.
	m.preview.invalidate()
}

func (m *NotesViewModel) GetConfirmDialog() *chrome.ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show(chrome.DiscardConfirmAction, "Discard unsaved changes?")
		return &m.confirmOverlay
	}
	m.confirmOverlay.Cancel()
	return &m.Confirm
}

func (m *NotesViewModel) StatusBarLabel() string {
	return "Notes"
}

// IsFullHeight returns true because the notes view renders its own footer.
func (m *NotesViewModel) IsFullHeight() bool { return true }
