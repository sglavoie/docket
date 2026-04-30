package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var globalSearchTabNames = []string{"All", "Tasks", "Notes"}

const (
	globalSearchTabTasks = 1
	globalSearchTabNotes = 2
)

// GlobalSearchModel provides global search across all projects.
type GlobalSearchModel struct {
	BaseView

	config   *config.Config
	projects []*models.Project
	allTasks []models.SearchResult  // flattened list of all tasks with project refs
	filtered []filteredSearchResult // tasks matching current query with highlight positions
	cursor   int

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
}

// NewGlobalSearchModel creates a new global search model.
func NewGlobalSearchModel(cfg *config.Config) *GlobalSearchModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search all tasks..."
	searchInput.CharLimit = 100
	searchInput.SetWidth(40)
	searchInput.Focus()
	applyThemedTextInput(&searchInput)

	projects := cfg.GetActiveProjects()

	// Load all tasks and notes from all projects
	var allTasks []models.SearchResult
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			allTasks = append(allTasks, models.SearchResult{
				Task:    task,
				Project: project,
			})
		}
		// Load notes for search
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

	m := &GlobalSearchModel{
		config:        cfg,
		projects:      projects,
		allTasks:      allTasks,
		searchInput:   searchInput,
		cursor:        0,
		tabs:          TabState{Names: globalSearchTabNames},
		hideCompleted: true,
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
		// Collect tags from all tasks
		for _, task := range project.Tasks {
			for _, tag := range task.Tags {
				tagSet[tag] = true
			}
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
		for _, section := range project.GetSections() {
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
	projects := m.config.GetActiveProjects()
	var allTasks []models.SearchResult
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			allTasks = append(allTasks, models.SearchResult{
				Task:    task,
				Project: project,
			})
		}
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
	m.projects = projects
	m.allTasks = allTasks
	m.buildCaches()
	m.filterTasks()
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

func (m *GlobalSearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *GlobalSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		key := msg.String()

		// Handle autocomplete navigation when active
		if m.ac.HandleKey(key, func() {
			m.acceptAutocompleteSuggestion()
			m.filterTasks()
		}) {
			return m, nil
		}

		switch key {
		case "esc":
			return m, func() tea.Msg {
				return CloseGlobalSearchMsg{}
			}
		case "enter":
			// Only select if not in autocomplete mode
			if !m.ac.IsActive() && len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				result := m.filtered[m.cursor].result
				if result.IsNote() {
					// Open notes view for the project containing the note
					return m, func() tea.Msg {
						return ShowNotesMsg{Project: result.Project}
					}
				}
				return m, func() tea.Msg {
					return GlobalSearchSelectMsg{
						Task:    result.Task,
						Project: result.Project,
					}
				}
			}
			return m, nil
		case "up", "ctrl+k":
			if !m.ac.IsActive() && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+j":
			if !m.ac.IsActive() && m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "shift+left", "shift+right":
			if handleTabCycleKey(key, &m.tabs, m.filterTasks) {
				return m, nil
			}
		case "ctrl+o":
			m.hideCompleted = !m.hideCompleted
			m.filterTasks()
			return m, nil
		case "ctrl+u":
			jumpCursor(&m.cursor, len(m.filtered), -10)
			return m, nil
		case "ctrl+d":
			jumpCursor(&m.cursor, len(m.filtered), 10)
			return m, nil
		case "?":
			if !m.ac.IsActive() {
				m.showHelp = !m.showHelp
				return m, nil
			}
			fallthrough
		default:
			// Update search input
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.filterTasks()
			m.detectAutocomplete()
			return m, cmd
		}
	}

	return m, nil
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
