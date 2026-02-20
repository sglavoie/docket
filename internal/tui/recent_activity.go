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
	config *config.Config

	tasks    []DailyReviewTask
	filtered []DailyReviewTask
	cursor   int

	activeTab int

	// Filter
	filter        CommonFilterState
	pendingPrefix string
}

// NewRecentActivityModel creates a new recent activity model.
func NewRecentActivityModel(cfg *config.Config) *RecentActivityModel {
	m := &RecentActivityModel{
		config: cfg,
		filter: CommonFilterState{
			Tags:  make(map[string]bool),
			Attrs: make(map[string]string),
		},
	}
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
	return m.IsSearchMode()
}

// ShouldCaptureEsc returns true when search is active, a query is set, prefix is pending, or filters are active.
func (m *RecentActivityModel) ShouldCaptureEsc() bool {
	return m.IsSearchMode() || m.SearchQuery() != "" || m.HasPendingPrefix() || m.HasActiveFilters()
}

func (m *RecentActivityModel) Init() tea.Cmd {
	return nil
}

func (m *RecentActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

func (m *RecentActivityModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if m.IsSearchMode() {
		handled, cmd, valueChanged := m.HandleSearchInput(msg)
		if valueChanged {
			m.applyFilter()
			m.clampCursor()
		}
		if handled {
			return m, cmd
		}
		return m, cmd
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

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}

	case "g":
		m.cursor = 0

	case "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		}

	case "ctrl+u":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}

	case "ctrl+d":
		m.cursor += 10
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}

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
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewRecentActivity,
				ViewName:   "Recent Activity",
				Items:      m.GetHelpItems(),
			}
		}
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
	case "F":
		return m.handleRecentActivityFilterPrefix(key)
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		return m.handleRecentActivityFilterSubPrefix(prefix, key)
	}

	return m, nil
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *RecentActivityModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
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
	m.cursor = ClampCursor(m.cursor, len(m.filtered))
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
			b.WriteString(MutedStyle.Render("No tasks match current filters."))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("Press q or Esc to clear filters."))
		} else {
			b.WriteString(MutedStyle.Render("No recent activity found."))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("Press q or Esc to close."))
		}
		return b.String()
	}

	// Tab bar
	b.WriteString(m.renderTabBar())
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
		if m.HasActiveFilters() {
			lines = append(lines, MutedStyle.Render("No tasks match current filters. Press q or Esc to clear."))
		} else {
			lines = append(lines, MutedStyle.Render("No matching tasks."))
		}
	}

	b.WriteString(strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n"))
	b.WriteString("\n\n")

	left := HelpStyle.Render("j/k nav · [/] tabs · Enter open · p preview · / filter · r refresh · ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.Width()))

	return b.String()
}

func (m *RecentActivityModel) renderTabBar() string {
	return renderWrappingTabBar(recentActivityTabNames, m.activeTab, m.Width())
}

func (m *RecentActivityModel) renderTaskLine(dt DailyReviewTask, selected bool) string {
	task := dt.Task

	cursor := "  "
	if selected {
		cursor = "> "
	}

	projectName := util.Truncate(dt.Project.Name, 12)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))

	shortID := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

	maxTitleLen := m.Width() - 50
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	relTime := MutedStyle.Render(storage.FormatRelativeTime(task.UpdatedAt))

	line := fmt.Sprintf("%s%s %s %s %s %s  %s", cursor, projectStr, shortID, statusIcon, typeIcon, title, relTime)

	if selected {
		line = SelectedStyle.Render(line)
	}
	return line
}
