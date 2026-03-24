package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var recentActivityTabNames = []string{"All", "Active", "Completed"}

const (
	recentActivityTabActive    = 1
	recentActivityTabCompleted = 2
)

// RecentActivityModel displays recently modified tasks across all projects.
type RecentActivityModel struct {
	BaseSearchableView
	InteractiveViewState
	config *config.Config

	tasks    []CrossProjectTask
	filtered []CrossProjectTask

	tabs TabState

	// Filter
	FilterMixin

	// When non-nil, scopes to a single project (omits project badges).
	projectScope *models.Project
}

// NewRecentActivityModel creates a new recent activity model.
func NewRecentActivityModel(cfg *config.Config) *RecentActivityModel {
	m := &RecentActivityModel{
		config:      cfg,
		FilterMixin: FilterMixin{filter: newFilterState()},
		tabs:        TabState{Names: recentActivityTabNames},
	}
	if !cfg.GetRecentActivityShowAll() {
		m.tabs.Active = recentActivityTabActive
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.InitSearchInput("Filter tasks...", 100, 40)
	m.loadTasks()
	return m
}

// NewProjectRecentActivityModel creates a project-scoped recent activity model.
func NewProjectRecentActivityModel(cfg *config.Config, project *models.Project) *RecentActivityModel {
	m := &RecentActivityModel{
		config:       cfg,
		projectScope: project,
		FilterMixin:  FilterMixin{filter: newFilterState()},
		tabs:         TabState{Names: recentActivityTabNames},
	}
	if !cfg.GetRecentActivityShowAll() {
		m.tabs.Active = recentActivityTabActive
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.InitSearchInput("Filter tasks...", 100, 40)
	m.loadTasks()
	return m
}

// loadTasks aggregates tasks from all active projects, sorted by UpdatedAt desc.
func (m *RecentActivityModel) loadTasks() {
	m.tasks = nil
	limit := m.config.GetRecentActivityLimit()

	if m.projectScope != nil {
		for _, task := range m.projectScope.Tasks {
			m.tasks = append(m.tasks, CrossProjectTask{Task: task, Project: m.projectScope})
		}
	} else {
		projects := m.config.GetActiveProjects()
		for _, project := range projects {
			if err := storage.LoadProject(project); err != nil {
				continue
			}
			for _, task := range project.Tasks {
				m.tasks = append(m.tasks, CrossProjectTask{Task: task, Project: project})
			}
		}
	}

	// Sort by UpdatedAt descending
	sort.Slice(m.tasks, func(i, j int) bool {
		return m.tasks[i].Task.UpdatedAt.After(m.tasks[j].Task.UpdatedAt)
	})

	// Cap at limit
	if limit > 0 && len(m.tasks) > limit {
		m.tasks = m.tasks[:limit]
	}

	m.applyFilter()
}

func (m *RecentActivityModel) applyFilter() {
	// First filter by tab and common filters
	var tabFiltered []CrossProjectTask
	hasFilters := hasActiveCommonFilters(&m.filter)
	for _, dt := range m.tasks {
		switch m.tabs.Active {
		case recentActivityTabActive:
			if dt.Task.Status.IsComplete() {
				continue
			}
		case recentActivityTabCompleted:
			if !dt.Task.Status.IsComplete() {
				continue
			}
		}
		if hasFilters && !passesCommonFilters(dt.Task, &m.filter, dt.Project.ID, m.config) {
			continue
		}
		tabFiltered = append(tabFiltered, dt)
	}

	// Then apply search
	query := strings.ToLower(m.SearchQuery())
	if query == "" && !m.IsSearchMode() {
		query = strings.ToLower(m.SearchInputValue())
	}
	if query == "" {
		m.filtered = tabFiltered
		return
	}
	m.filtered = nil
	for _, dt := range tabFiltered {
		title := strings.ToLower(dt.Task.Title)
		project := strings.ToLower(dt.Project.Name)
		if strings.Contains(title, query) || strings.Contains(project, query) {
			m.filtered = append(m.filtered, dt)
		}
	}
}

// HasActiveSearch returns true when search is active or a query is set.
func (m *RecentActivityModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// HasTextInput returns true when search input is active.
func (m *RecentActivityModel) HasTextInput() bool {
	return m.DefaultHasTextInput()
}

// escSteps defines the Esc dismissal cascade: search → filters.
func (m *RecentActivityModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.applyFilter()
			m.clampCursor()
		}},
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when search is active, a query is set, prefix is pending, or filters are active.
func (m *RecentActivityModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || AnyEscStepActive(m.escSteps())
}

func (m *RecentActivityModel) Init() tea.Cmd {
	return nil
}

func (m *RecentActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadTasks, m.clampCursor, m.setStatus); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

func (m *RecentActivityModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		m.applyFilter()
		m.clampCursor()
	}); handled {
		return model, cmd
	}

	// Handle pending filter prefix keys
	if m.pendingPrefix != "" {
		return m.handleRecentActivityPrefixKey(msg.String())
	}

	switch msg.String() {
	case "esc", "q":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}
		if m.projectScope != nil {
			return m, func() tea.Msg {
				return CloseProjectRecentActivityMsg{}
			}
		}
		return m, func() tea.Msg {
			return CloseRecentActivityMsg{}
		}

	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			dt := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return RecentActivitySelectMsg(dt)
			}
		}

	case "ctrl+.":
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			dt := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return OpenQuickNoteMsg{Task: dt.Task, Project: dt.Project}
			}
		}

	case "g":
		m.pendingPrefix = "g"
		return m, nil

	case "p":
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			dt := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return ShowRecentActivityPreviewMsg{Task: dt, Project: dt.Project}
			}
		}

	case "[", "h", "]", "l":
		if handleTabCycleKey(msg.String(), &m.tabs, true, func() {
			m.applyFilter()
			m.clampCursor()
		}) {
			return m, nil
		}

	case "r":
		m.loadTasks()
		m.clampCursor()

	case "F":
		m.pendingPrefix = "F"
		return m, nil

	case "/":
		return m, m.ActivateSearch()

	case "?":
		viewType := ViewRecentActivity
		viewName := "Recent Activity"
		if m.projectScope != nil {
			viewType = ViewProjectRecentActivity
			viewName = "Project Activity"
		}
		return m, helpCmd(viewType, viewName, m.GetHelpItems())
	}

	if handled, cmd := handleListNavigation(msg.String(), &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	return m, nil
}

// handleRecentActivityPrefixKey handles prefix key sequences for recent activity.
func (m *RecentActivityModel) handleRecentActivityPrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	if key == "esc" {
		return m, nil
	}

	switch prefix {
	case "g":
		if key == "g" {
			m.cursor = 0
		}
		return m, nil
	case "F":
		return m.handleRecentActivityFilterPrefix(key)
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		return m.handleRecentActivityFilterSubPrefix(prefix, key)
	}

	return m, nil
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *RecentActivityModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	// All filter categories enabled for recent activity
	hints := buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true)
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

func (m *RecentActivityModel) clampCursor() {
	m.ClampCursorTo(len(m.filtered))
}

// GetHelpItems returns help items for this view.
func (m *RecentActivityModel) GetHelpItems() []HelpItem {
	if m.projectScope != nil {
		return GetKeybindingRegistry().GetForView(ViewProjectRecentActivity)
	}
	return GetKeybindingRegistry().GetForView(ViewRecentActivity)
}

func (m *RecentActivityModel) View() tea.View {
	var b strings.Builder

	title := "Latest Activity"
	if m.projectScope != nil {
		title = fmt.Sprintf("Project Activity: %s", m.projectScope.Name)
	}
	subtitle := fmt.Sprintf("%d tasks", len(m.filtered))
	if m.SearchQuery() != "" || m.IsSearchMode() {
		subtitle = fmt.Sprintf("%d/%d tasks", len(m.filtered), len(m.tasks))
	}
	b.WriteString(renderFullscreenTitle(title, subtitle))
	b.WriteString("\n")

	if len(m.filtered) == 0 && !m.IsSearchMode() && m.SearchQuery() == "" {
		b.WriteString("\n")
		if m.HasActiveFilters() {
			b.WriteString(m.renderActiveFilterIndicator())
			b.WriteString("\n\n")
		}
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No recent activity found.", "Press q or Esc to close.",
			"Press q or Esc to clear filters."))
		return tea.NewView(b.String())
	}

	// Tab bar
	b.WriteString(m.tabs.RenderBar(m.Width()))
	b.WriteString("\n")

	// Filter badge
	filterBadge := m.renderActiveFilterIndicator()
	if filterBadge != "" {
		b.WriteString(filterBadge)
		b.WriteString("\n")
	}

	// Search input
	if m.IsSearchMode() {
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d/%d)", m.SearchQuery(), len(m.filtered), len(m.tasks))))
		b.WriteString("\n")
	}

	contentHeight := fullscreenContentHeight(m.Height(), 2) // tab bar(2)
	if filterBadge != "" {
		contentHeight--
	}
	if m.IsSearchMode() || m.SearchQuery() != "" {
		contentHeight--
	}

	// Build lines
	var lines []string
	for i, dt := range m.filtered {
		selected := i == m.cursor
		line := m.renderTaskLine(dt, selected)
		lines = append(lines, line)
	}

	if len(m.filtered) == 0 {
		lines = append(lines, MutedStyle.Render(EmptyStateFiltered))
	}

	b.WriteString(strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n"))
	b.WriteString("\n\n")

	left := HelpStyle.Render("j/k nav · [/] tabs · Enter open · p preview · / filter · r refresh · ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.Width()))

	return tea.NewView(b.String())
}

func (m *RecentActivityModel) renderTaskLine(dt CrossProjectTask, selected bool) string {
	task := dt.Task
	var preBadges string
	maxTitleLen := m.Width() - 50
	if m.projectScope == nil {
		projectName := truncateToVisualWidth(dt.Project.Name, 12)
		preBadges = MutedStyle.Render(fmt.Sprintf("[%s]", projectName)) + " " +
			MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)) + " "
	} else {
		preBadges = MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)) + " "
		maxTitleLen += 14
	}
	relTime := MutedStyle.Render("  " + storage.FormatRelativeTime(task.UpdatedAt))
	return TaskLine{
		Task:        task,
		IsSelected:  selected,
		MaxTitle:    maxTitleLen,
		PreBadges:   preBadges,
		StageBadge:  GetStageBadge(task.Stage),
		PostTitle:   relTime,
		ViewWidth:   m.Width(),
		Flash:       m.cursorFlash,
		SearchQuery: m.SearchQuery(),
	}.Render()
}

func (m *RecentActivityModel) StatusBarLabel() string { return "Recent Activity" }
