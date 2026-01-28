package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// GlobalSearchModel provides global search across all projects.
type GlobalSearchModel struct {
	config   *config.Config
	projects []*models.Project
	allTasks []models.SearchResult  // flattened list of all tasks with project refs
	filtered []filteredSearchResult // tasks matching current query with highlight positions
	cursor   int

	// Input for filtering
	searchInput textinput.Model

	// Parsed filters from current query
	parsedFilters *ParsedSearchQuery

	// Autocomplete state
	acMode       int                // Current autocomplete mode (acSearchMode*)
	suggestions  []searchSuggestion // Current autocomplete suggestions
	acCursor     int                // Selected suggestion index
	acTriggerPos int                // Position of trigger character in input

	// Caches for autocomplete
	tagCache     []string // All tags across projects
	sectionCache []string // All sections across projects

	// Dimensions
	width        int
	height       int
	listWidth    int
	previewWidth int
}

// NewGlobalSearchModel creates a new global search model.
func NewGlobalSearchModel(cfg *config.Config) *GlobalSearchModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search all tasks..."
	searchInput.CharLimit = 100
	searchInput.Width = 40
	searchInput.Focus()

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
		config:      cfg,
		projects:    projects,
		allTasks:    allTasks,
		searchInput: searchInput,
		cursor:      0,
		acMode:      acSearchModeNone,
	}

	// Build caches for autocomplete
	m.buildCaches()

	// Start with no results (empty query shows nothing)
	m.filtered = nil
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

// HasActiveSearch returns true (search input is always active).
func (m *GlobalSearchModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because global search always has input focus.
func (m *GlobalSearchModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns false so the app can close the view on esc.
func (m *GlobalSearchModel) ShouldCaptureEsc() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *GlobalSearchModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate inner content width for split-pane overlay:
	// cardWidth = viewWidth - 4 (margin)
	// lipgloss Width = cardWidth - 2 (border adds 2 to total)
	// usable text area = Width - 4 (padding inside) = viewWidth - 10
	innerWidth := width - 10
	if innerWidth < 60 {
		innerWidth = 60
	}

	// Calculate split pane widths: 50/50 based on inner width
	m.listWidth = innerWidth * 50 / 100
	if m.listWidth < 30 {
		m.listWidth = 30
	}
	m.previewWidth = innerWidth - m.listWidth - 1 // -1 for separator

	m.searchInput.Width = innerWidth - 4
}

func (m *GlobalSearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *GlobalSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Handle autocomplete navigation when active
		if m.acMode != acSearchModeNone && len(m.suggestions) > 0 {
			switch key {
			case "tab", "enter":
				m.acceptAutocompleteSuggestion()
				m.filterTasks()
				return m, nil
			case "esc":
				// Dismiss autocomplete
				m.acMode = acSearchModeNone
				m.suggestions = nil
				m.acCursor = 0
				return m, nil
			case "up", "ctrl+p":
				if m.acCursor > 0 {
					m.acCursor--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.acCursor < len(m.suggestions)-1 {
					m.acCursor++
				}
				return m, nil
			}
		}

		switch key {
		case "esc":
			return m, func() tea.Msg {
				return CloseGlobalSearchMsg{}
			}
		case "enter":
			// Only select if not in autocomplete mode
			if m.acMode == acSearchModeNone && len(m.filtered) > 0 && m.cursor < len(m.filtered) {
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
			if m.acMode == acSearchModeNone && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+j":
			if m.acMode == acSearchModeNone && m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "ctrl+u":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			return m, nil
		case "ctrl+d":
			m.cursor += 10
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			return m, nil
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
type CloseGlobalSearchMsg struct{}

// GlobalSearchSelectMsg is sent when a task is selected from global search.
type GlobalSearchSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowGlobalSearchMsg is sent to open the global search.
type ShowGlobalSearchMsg struct{}
