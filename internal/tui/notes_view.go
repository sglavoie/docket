package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func (s NotesScope) valid() bool {
	switch s.Kind {
	case NotesScopeAll, NotesScopeGlobal:
		return s.ProjectID == ""
	case NotesScopeProject:
		return strings.TrimSpace(s.ProjectID) != ""
	default:
		return false
	}
}

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
	BaseView

	notesDir    string
	scope       NotesScope
	noteService projectNoteService
	noteLedger  *storage.WorkspaceLedger

	owners           map[app.NoteLocation]noteManagerOwner
	ownerProjects    []*models.Project
	loadedRows       []LocatedNoteRow
	visibleRows      []LocatedNoteRow
	collectionErrors []noteCollectionFailure
	snapshotError    error
	searchMatches    map[NoteRowIdentity]noteSearchMatch

	searchEditing    bool
	searchApplied    string
	searchBeforeEdit string
	searchInput      textinput.Model

	// searchCaseRelaxed records that the current visible rows came from a
	// case-insensitive retry after the query's smart-case reading matched
	// nothing.
	searchCaseRelaxed bool

	cursor       int
	scrollOffset int
	cursorFlash  bool

	// preview is the wide-layout body pane's line cache and scroll position. It
	// is keyed on the note, its revision, and the pane width, so cursor moves,
	// reloads, and resizes reopen it at the top by construction.
	preview notesPreviewState

	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the manager keeps its own one-key prefix.
	pendingTopChord bool

	confirm       ConfirmDialog
	pendingDelete NoteRowIdentity

	renaming       bool
	renameTarget   NoteRowIdentity
	renameInput    textinput.Model
	confirmDiscard bool
	confirmOverlay ConfirmDialog

	statusMessage string
	statusIsError bool

	locationPicker *notesLocationPicker
	moveSource     *noteMutationRef

	registry *OperationRegistry
	dispatch operationKeyDispatch
}

// SetOperationSystem hands the manager the descriptor authority and the shell's
// key dispatch seam. Without them the browse grammar resolves nothing and `?`
// documents nothing, so a host that mounts this model must supply both. The
// executor stays out because the Space sheet is shell-owned.
func (m *NotesViewModel) SetOperationSystem(
	registry *OperationRegistry,
	dispatch operationKeyDispatch,
) {
	m.registry = registry
	m.dispatch = dispatch
}

// NewNotesViewModelWithService constructs the workspace-wide Notes manager.
// projects must be the note owners returned by Runtime.AvailableProjects().
func NewNotesViewModelWithService(
	notesDir string,
	scope NotesScope,
	service projectNoteService,
	ledger *storage.WorkspaceLedger,
	projects ...*models.Project,
) *NotesViewModel {
	m := &NotesViewModel{
		notesDir:    notesDir,
		scope:       scope,
		noteService: service,
		noteLedger:  ledger,
	}
	m.initSearchInput()
	m.setOwnerProjects(projects)
	m.loadNotes()
	return m
}

func (m *NotesViewModel) initSearchInput() {
	m.searchInput = textinput.New()
	m.searchInput.Placeholder = "Search note titles and bodies..."
	m.searchInput.CharLimit = 200
	m.searchInput.SetWidth(SearchInputWidth(m.Width()))
	theme.ApplyThemedTextInput(&m.searchInput)
}

// setOwnerProjects rebuilds the owner catalog.
func (m *NotesViewModel) setOwnerProjects(projects []*models.Project) {
	m.owners, m.ownerProjects = noteOwnerCatalog(m.notesDir, projects)
}

// noteOwnerCatalog resolves the note owners a workspace-wide read covers:
// Global plus every supplied project, in the supplied order. Archival is the
// only reason a project is refused — a parked owner keeps its notes reachable,
// so the parked tag is deliberately not consulted here. The Notes manager and
// the Outliner's search-tier note index both admit owners through this one
// function, so neither can drift from the other's exclusion rule.
func noteOwnerCatalog(
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
	scopeInvalid := m.scope.Kind == NotesScopeProject &&
		!m.locationAvailable(app.ProjectNoteLocation(m.scope.ProjectID))
	if scopeInvalid {
		m.scope = AllNotesScope()
	}
	m.refreshLocationPicker()
	m.loadNotes()
	return scopeInvalid
}

func (m *NotesViewModel) locationAvailable(location app.NoteLocation) bool {
	_, ok := m.owners[location]
	return ok
}

func (m *NotesViewModel) refreshLocationPicker() {
	if m.locationPicker == nil {
		return
	}

	purpose := m.locationPicker.purpose
	query := m.locationPicker.SearchQuery()
	selectedID := m.locationPicker.SelectedItem().ItemID()
	excluded := app.NoteLocation{}
	if purpose == notesLocationPickerMove {
		if m.moveSource == nil || !m.moveSource.valid() ||
			!m.locationAvailable(m.moveSource.Location) {
			m.locationPicker = nil
			m.moveSource = nil
			m.setErrorStatus("Move canceled: the source location is no longer available")
			return
		}
		excluded = m.moveSource.Location
	}

	picker := newNotesLocationPicker(
		purpose,
		m.scope,
		excluded,
		m.ownerProjects,
	)
	if purpose == notesLocationPickerMove && len(picker.SourceItems()) == 0 {
		m.locationPicker = nil
		m.moveSource = nil
		m.setStatus("No other note locations are available; note was not moved")
		return
	}
	picker.SetSize(m.Width(), m.Height())
	picker.SetAccentColor(m.AccentColor())
	if query != "" {
		picker.SetSearchInputValue(query)
		picker.SetSearchQuery(query)
		picker.alwaysSearch.fuzzyFilter(query)
	}
	if selectedID != "" {
		picker.SetCursorToID(selectedID)
	}
	m.locationPicker = picker
}

func (m *NotesViewModel) ownerForLocation(location app.NoteLocation) noteManagerOwner {
	if owner, ok := m.owners[location]; ok {
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

func (m *NotesViewModel) projectByID(projectID string) *models.Project {
	owner, ok := m.owners[app.ProjectNoteLocation(projectID)]
	if !ok {
		return nil
	}
	return owner.Project
}

func (m *NotesViewModel) effectiveDir(row *LocatedNoteRow) string {
	if row == nil {
		return ""
	}
	return m.ownerForLocation(row.Location).Directory
}

func (m *NotesViewModel) noteLedgerScope() storage.LedgerScope {
	return storage.GlobalScope()
}

func (m *NotesViewModel) setStatus(msg string) {
	m.statusMessage = msg
	m.statusIsError = false
}

func (m *NotesViewModel) setErrorStatus(msg string) {
	m.statusMessage = msg
	m.statusIsError = true
}

func (m *NotesViewModel) clearStatus() {
	m.statusMessage = ""
	m.statusIsError = false
}

func (m *NotesViewModel) resetScrollOffset() {
	m.scrollOffset = 0
}

func (m *NotesViewModel) handleConfirmKey(key string) (confirmed, handled bool) {
	if !m.confirm.IsActive() {
		return false, false
	}
	return m.confirm.HandleKey(key)
}

func (m *NotesViewModel) Init() tea.Cmd {
	if m.locationPicker != nil {
		return m.locationPicker.Init()
	}
	if m.searchEditing {
		m.searchInput.Focus()
		return textinput.Blink
	}
	return nil
}

func (m *NotesViewModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.searchInput.SetWidth(SearchInputWidth(width))
	if m.locationPicker != nil {
		m.locationPicker.SetSize(width, height)
	}
	m.resetScrollOffset()
	// A resize can change the pane width, the row budget, or whether the preview
	// exists at all, and every one of those makes a manual offset describe lines
	// the reader can no longer see. Dropping the cache reopens at the top, which
	// is the only position that is correct under all three.
	m.preview.invalidate()
}

func (m *NotesViewModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
		return &m.confirmOverlay
	}
	m.confirmOverlay.Cancel()
	return &m.confirm
}

func (m *NotesViewModel) StatusBarLabel() string {
	return "Notes"
}

// IsFullHeight returns true because the notes view renders its own footer.
func (m *NotesViewModel) IsFullHeight() bool { return true }
