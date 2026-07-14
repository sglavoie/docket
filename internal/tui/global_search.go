package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workspace"
)

var globalSearchTabNames = []string{"All", "Tasks", "Notes"}

const (
	globalSearchTabTasks = 1
	globalSearchTabNotes = 2
)

// GlobalSearchModel provides global search across all projects.
type GlobalSearchModel struct {
	BaseView

	runtime         *config.RuntimeState
	workspaceLoader *workspace.Loader
	projects        []*models.Project
	allTasks        []models.SearchResult  // flattened list of all tasks with project refs
	filtered        []filteredSearchResult // tasks matching current query with highlight positions
	cursor          int
	scrollOffset    int

	// Input for filtering
	searchInput textinput.Model

	// Parsed filters from current query
	parsedFilters *ParsedSearchQuery

	// Tabs
	tabs TabState

	// Autocomplete state
	ac AutocompleteState

	// Caches for autocomplete
	tagCache     []string // All tags across projects
	sectionCache []string // All sections across projects

	// Pane dimensions
	listWidth    int
	previewWidth int

	// Toggle to hide completed/canceled tasks
	hideCompleted bool

	// Toggle to show help panel
	showHelp bool

	// Search history navigation
	searchHistory      []string
	searchHistoryIndex int // -1 = not browsing history, 0+ = index in history
}

// NewGlobalSearchModel creates a new global search model.
func NewGlobalSearchModel(runtime *config.RuntimeState) *GlobalSearchModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search tasks… ($status #tag @due ^type) · Alt+↑↓ history"
	searchInput.CharLimit = 100
	searchInput.SetWidth(40)
	searchInput.Focus()
	applyThemedTextInput(&searchInput)

	loader := workspace.NewLoader()
	projects, allTasks := loadGlobalSearchIndex(runtime, loader)

	m := &GlobalSearchModel{
		runtime:            runtime,
		workspaceLoader:    loader,
		projects:           projects,
		allTasks:           allTasks,
		searchInput:        searchInput,
		cursor:             0,
		tabs:               TabState{Names: globalSearchTabNames},
		hideCompleted:      true,
		searchHistory:      runtime.Machine().GetGlobalSearchHistory(),
		searchHistoryIndex: -1,
	}

	// Build caches for autocomplete
	m.buildCaches()

	// Apply initial filter (hideCompleted shows non-completed tasks by default)
	m.filterTasks()
	return m
}

// HasActiveSearch returns true (search input is always active).
func (m *GlobalSearchModel) buildCaches() {
	tagSet := make(map[string]bool)
	sectionSet := make(map[string]bool)

	for _, project := range m.projects {
		// Collect project catalog and task-used tags
		for _, tag := range CollectTagNamesForProject(project) {
			tagSet[tag] = true
		}
		// Collect tags from notes
		notes, err := storage.ListNotes(project)
		if err == nil {
			for _, note := range notes {
				for _, tag := range note.Tags {
					tagSet[tag] = true
				}
			}
		}
		// Collect sections
		for _, section := range gtdSections(project.GetSections()) {
			sectionSet[section] = true
		}
	}

	m.tagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.tagCache = append(m.tagCache, tag)
	}

	m.sectionCache = make([]string, 0, len(sectionSet))
	for section := range sectionSet {
		m.sectionCache = append(m.sectionCache, section)
	}
}

// Rebuild reloads notes for all projects to refresh the search index.
func (m *GlobalSearchModel) Rebuild() {
	projects, allTasks := loadGlobalSearchIndex(m.runtime, m.workspaceLoader)
	m.projects = projects
	m.allTasks = allTasks
	m.buildCaches()
	m.filterTasks()
}

// loadGlobalSearchIndex loads every active project once through an immutable
// workspace snapshot and flattens tasks and notes into search results. Snapshot
// diagnostics (skipped/corrupt projects) are logged rather than silently
// dropped so the index visibly reflects any project that could not be read.
func loadGlobalSearchIndex(runtime *config.RuntimeState, loader *workspace.Loader) ([]*models.Project, []models.SearchResult) {
	if loader == nil {
		loader = workspace.NewLoader()
	}
	snap := loader.Build(runtime.ActiveProjects())
	for _, diag := range snap.Diagnostics() {
		logging.Warn("global search: %s", diag.String())
	}

	projects := snap.Projects()
	var allTasks []models.SearchResult
	for _, project := range projects {
		for _, task := range project.Tasks {
			allTasks = append(allTasks, models.SearchResult{
				Task:    task,
				Project: project,
			})
		}
		// Load notes for search.
		notes, err := storage.ListNotes(project)
		if err == nil {
			for _, note := range notes {
				allTasks = append(allTasks, models.SearchResult{
					Note:    note,
					Project: project,
				})
			}
		}
	}
	return projects, allTasks
}

// HasActiveSearch returns true (search input is always active).
func (m *GlobalSearchModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because global search always has input focus.
func (m *GlobalSearchModel) HasTextInput() bool {
	return true
}

func (m *GlobalSearchModel) SupportsHelpShortcut() bool { return false }

// SetSize sets the view dimensions and recalculates pane widths.
func (m *GlobalSearchModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.resetScrollOffset()

	// Calculate inner content width for split-pane overlay:
	// cardWidth = viewWidth - 4 (margin)
	// lipgloss Width = cardWidth - 2 (border adds 2 to total)
	// usable text area = Width - 4 (padding inside) = viewWidth - 10
	innerWidth := max(width-10, 1)

	// Calculate split pane widths: 50/50 based on inner width
	m.listWidth = max(innerWidth*50/100, 1)
	m.previewWidth = max(
		// -1 for separator
		innerWidth-m.listWidth-1, 1)

	m.searchInput.SetWidth(max(innerWidth-4, 1))
}

func (m *GlobalSearchModel) resetScrollOffset() {
	m.scrollOffset = 0
}

func (m *GlobalSearchModel) clampScrollOffsetToResults() {
	if len(m.filtered) == 0 {
		m.scrollOffset = 0
		return
	}
	m.scrollOffset = min(max(m.scrollOffset, 0), len(m.filtered)-1)
}

func (m *GlobalSearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *GlobalSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		return m.updateKeyPress(msg)
	}

	return m, nil
}

func (m *GlobalSearchModel) updateKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.handleAutocompleteKey(key) {
		return m, nil
	}
	if cmd, handled := m.handleShortcutKey(key); handled {
		return m, cmd
	}

	return m.updateSearchInput(msg)
}

func (m *GlobalSearchModel) handleAutocompleteKey(key string) bool {
	return m.ac.HandleKey(key, func() {
		m.acceptAutocompleteSuggestion()
		m.filterTasks()
	})
}

func (m *GlobalSearchModel) handleShortcutKey(key string) (tea.Cmd, bool) {
	switch key {
	case "esc":
		return func() tea.Msg {
			return CloseGlobalSearchMsg{}
		}, true
	case "enter":
		return m.selectCurrentResult(), true
	case "alt+up":
		m.moveSearchHistoryUp()
		return nil, true
	case "alt+down":
		m.moveSearchHistoryDown()
		return nil, true
	case "up", "ctrl+k":
		m.moveCursorUp()
		return nil, true
	case "down", "ctrl+j":
		m.moveCursorDown()
		return nil, true
	case "shift+left", "shift+right":
		handleTabCycleKey(key, &m.tabs, m.filterTasks)
		return nil, true
	case "ctrl+o":
		m.hideCompleted = !m.hideCompleted
		m.filterTasks()
		return nil, true
	case "ctrl+u":
		jumpCursor(&m.cursor, len(m.filtered), -10)
		return nil, true
	case "ctrl+d":
		jumpCursor(&m.cursor, len(m.filtered), 10)
		return nil, true
	case "?":
		if !m.ac.IsActive() {
			m.showHelp = !m.showHelp
			return nil, true
		}
	}
	return nil, false
}

func (m *GlobalSearchModel) selectCurrentResult() tea.Cmd {
	if m.ac.IsActive() || len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}

	result := m.filtered[m.cursor].result
	m.recordGlobalSearchHistory()
	if result.IsNote() {
		return func() tea.Msg {
			return ShowNotesMsg{Project: result.Project}
		}
	}

	return func() tea.Msg {
		return GlobalSearchSelectMsg{
			Task:    result.Task,
			Project: result.Project,
		}
	}
}

func (m *GlobalSearchModel) moveSearchHistoryUp() {
	if len(m.searchHistory) == 0 {
		return
	}
	if m.searchHistoryIndex < 0 {
		m.searchHistoryIndex = 0
	} else if m.searchHistoryIndex < len(m.searchHistory)-1 {
		m.searchHistoryIndex++
	}
	if m.searchHistoryIndex < len(m.searchHistory) {
		m.setGlobalSearchQuery(m.searchHistory[m.searchHistoryIndex])
	}
}

func (m *GlobalSearchModel) moveSearchHistoryDown() {
	if m.searchHistoryIndex > 0 {
		m.searchHistoryIndex--
		m.setGlobalSearchQuery(m.searchHistory[m.searchHistoryIndex])
	} else if m.searchHistoryIndex == 0 {
		m.searchHistoryIndex = -1
		m.setGlobalSearchQuery("")
	}
}

func (m *GlobalSearchModel) moveCursorUp() {
	if !m.ac.IsActive() && m.cursor > 0 {
		m.cursor--
	}
}

func (m *GlobalSearchModel) moveCursorDown() {
	if !m.ac.IsActive() && m.cursor < len(m.filtered)-1 {
		m.cursor++
	}
}

func (m *GlobalSearchModel) updateSearchInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.filterTasks()
	m.detectAutocomplete()
	return m, cmd
}

func (m *GlobalSearchModel) setGlobalSearchQuery(query string) {
	m.searchInput.SetValue(query)
	m.searchInput.SetCursor(len(query))
	m.ac.Clear()
	m.filterTasks()
}

func (m *GlobalSearchModel) recordGlobalSearchHistory() {
	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		return
	}

	newHistory := []string{query}
	for _, h := range m.searchHistory {
		if h != query {
			newHistory = append(newHistory, h)
		}
	}

	const maxHistorySize = 10
	if len(newHistory) > maxHistorySize {
		newHistory = newHistory[:maxHistorySize]
	}

	m.searchHistory = newHistory
	m.searchHistoryIndex = -1
	if err := commitMachineToRuntime(m.runtime, func(machine *config.MachineState) {
		machine.SetGlobalSearchHistory(newHistory)
	}); err != nil {
		logging.Warn("failed to save machine state: %v", err)
	}
}

// CloseGlobalSearchMsg is sent when the global search should close.
type CloseGlobalSearchMsg struct{ PopViewMsg }

// GlobalSearchSelectMsg is sent when a task is selected from global search.
type GlobalSearchSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowGlobalSearchMsg is sent to open the global search.
type ShowGlobalSearchMsg struct{}

func (m *GlobalSearchModel) StatusBarLabel() string { return "Global Search" }
