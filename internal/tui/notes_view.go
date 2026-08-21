package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	applyThemedTextInput(&m.searchInput)
}

// setOwnerProjects rebuilds the owner catalog. Archival is the only reason a
// supplied project is refused: a parked owner keeps its notes reachable, so the
// parked tag is deliberately not consulted here.
func (m *NotesViewModel) setOwnerProjects(projects []*models.Project) {
	seen := make(map[string]bool, len(projects))
	m.ownerProjects = make([]*models.Project, 0, len(projects))
	m.owners = make(map[app.NoteLocation]noteManagerOwner, len(projects)+1)
	global := app.GlobalNoteLocation()
	m.owners[global] = noteManagerOwner{
		Label:     "Global",
		Directory: m.notesDir,
	}
	for _, project := range projects {
		if project == nil || project.Archived ||
			strings.TrimSpace(project.ID) == "" || seen[project.ID] {
			continue
		}
		seen[project.ID] = true
		m.ownerProjects = append(m.ownerProjects, project)
		location := app.ProjectNoteLocation(project.ID)
		label := strings.TrimSpace(project.Name)
		if label == "" {
			label = project.ID
		}
		m.owners[location] = noteManagerOwner{
			Label:     label,
			Directory: storage.NotesDir(project),
			Project:   project,
		}
	}
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
		left, right := m.visibleRows[i], m.visibleRows[j]
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
	})
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
