package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var recentActivityTabNames = []string{"All", "Active", "Completed"}

const (
	recentActivityTabAll       = 0
	recentActivityTabActive    = 1
	recentActivityTabCompleted = 2
)

// RecentActivityModel displays recently modified tasks across all projects.
type RecentActivityModel struct {
	BaseSearchableView
	InteractiveViewState
	config *config.Config

	tasks    []DailyReviewTask
	filtered []DailyReviewTask

	activeTab int

	// Filter
	FilterMixin
}

// NewRecentActivityModel creates a new recent activity model.
func NewRecentActivityModel(cfg *config.Config) *RecentActivityModel {
	m := &RecentActivityModel{
		config:      cfg,
		FilterMixin: FilterMixin{filter: newFilterState()},
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

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			m.tasks = append(m.tasks, DailyReviewTask{Task: task, Project: project})
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
	var tabFiltered []DailyReviewTask
	hasFilters := hasActiveCommonFilters(&m.filter)
	for _, dt := range m.tasks {
		switch m.activeTab {
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

// ShouldCaptureEsc returns true when search is active, a query is set, prefix is pending, or filters are active.
func (m *RecentActivityModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || m.SearchQuery() != "" || m.HasActiveFilters()
}

func (m *RecentActivityModel) Init() tea.Cmd {
	return nil
}

func (m *RecentActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadTasks, m.clampCursor, func(s string) { m.statusMessage = s }); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

func (m *RecentActivityModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "esc":
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseRecentActivityMsg{}
		}

	case "q":
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			return m, nil
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

	case "[", "h":
		CycleTab(&m.activeTab, len(recentActivityTabNames), -1)
		m.applyFilter()
		m.clampCursor()
		return m, nil

	case "]", "l":
		CycleTab(&m.activeTab, len(recentActivityTabNames), 1)
		m.applyFilter()
		m.clampCursor()
		return m, nil

	case "r":
		m.loadTasks()
		m.clampCursor()

	case "F":
		m.pendingPrefix = "F"
		return m, nil

	case "/":
		return m, m.ActivateSearch()

	case "?":
		return m, helpCmd(ViewRecentActivity, "Recent Activity", m.GetHelpItems())
	}

	if handleListNavigation(msg.String(), &m.cursor, len(m.filtered), m.Height()) {
		return m, nil
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
	return GetKeybindingRegistry().GetForView(ViewRecentActivity)
}

func (m *RecentActivityModel) View() string {
	var b strings.Builder

	subtitle := fmt.Sprintf("%d tasks", len(m.filtered))
	if m.SearchQuery() != "" || m.IsSearchMode() {
		subtitle = fmt.Sprintf("%d/%d tasks", len(m.filtered), len(m.tasks))
	}
	b.WriteString(renderFullscreenTitle("Latest Activity", subtitle))
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
		return b.String()
	}

	// Tab bar
	b.WriteString(renderWrappingTabBar(recentActivityTabNames, m.activeTab, m.Width()))
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

	return b.String()
}

func (m *RecentActivityModel) renderTaskLine(dt DailyReviewTask, selected bool) string {
	task := dt.Task
	projectName := util.Truncate(dt.Project.Name, 12)
	preBadges := MutedStyle.Render(fmt.Sprintf("[%s]", projectName)) + " " +
		MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)) + " "
	relTime := MutedStyle.Render("  " + storage.FormatRelativeTime(task.UpdatedAt))

	maxTitleLen := m.Width() - 50
	return TaskLine{
		Task:       task,
		IsSelected: selected,
		MaxTitle:   maxTitleLen,
		PreBadges:  preBadges,
		PostTitle:  relTime,
		ViewWidth:  m.Width(),
	}.Render()
}
