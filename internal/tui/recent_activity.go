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
}

// NewRecentActivityModel creates a new recent activity model.
func NewRecentActivityModel(cfg *config.Config) *RecentActivityModel {
	m := &RecentActivityModel{
		config: cfg,
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
	// First filter by tab
	var tabFiltered []DailyReviewTask
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

// ShouldCaptureEsc returns true when search is active or a query is set.
func (m *RecentActivityModel) ShouldCaptureEsc() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// SetSize sets the view dimensions.
func (m *RecentActivityModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
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

	switch msg.String() {
	case "esc":
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseRecentActivityMsg{}
		}

	case "q":
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
		if m.activeTab > 0 {
			m.activeTab--
		} else {
			m.activeTab = len(recentActivityTabNames) - 1
		}
		m.applyFilter()
		m.clampCursor()
		return m, nil

	case "]", "l":
		if m.activeTab < len(recentActivityTabNames)-1 {
			m.activeTab++
		} else {
			m.activeTab = 0
		}
		m.applyFilter()
		m.clampCursor()
		return m, nil

	case "r":
		m.loadTasks()
		m.clampCursor()

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

func (m *RecentActivityModel) clampCursor() {
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// GetHelpItems returns help items for this view.
func (m *RecentActivityModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewRecentActivity)
}

func (m *RecentActivityModel) View() string {
	if len(m.filtered) == 0 && !m.IsSearchMode() && m.SearchQuery() == "" {
		var b strings.Builder
		b.WriteString(MutedStyle.Render("No recent activity found."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press q or Esc to close."))
		return renderOverlayCard(b.String(), m.Width(), m.Height(), OverlayCardConfig{
			Title:    "Latest Activity",
			Subtitle: "no tasks",
			Centered: true,
		})
	}

	var b strings.Builder

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Search input
	if m.IsSearchMode() {
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d/%d)", m.SearchQuery(), len(m.filtered), len(m.tasks))))
		b.WriteString("\n")
	}

	contentHeight := m.Height() - 11 // account for tab bar
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
		lines = append(lines, MutedStyle.Render("No matching tasks."))
	}

	// Scrolling
	scrollOffset := 0
	if m.cursor >= contentHeight {
		scrollOffset = m.cursor - contentHeight + 1
	}

	visibleLines := lines
	if scrollOffset > 0 && scrollOffset < len(lines) {
		visibleLines = lines[scrollOffset:]
	}
	if len(visibleLines) > contentHeight {
		visibleLines = visibleLines[:contentHeight]
	}

	b.WriteString(strings.Join(visibleLines, "\n"))
	b.WriteString("\n\n")

	footer := HelpStyle.Render("j/k nav | [/] tabs | Enter open | p preview | / filter | r refresh | ? help")
	b.WriteString(footer)

	subtitle := fmt.Sprintf("%d tasks", len(m.filtered))
	if m.SearchQuery() != "" || m.IsSearchMode() {
		subtitle = fmt.Sprintf("%d/%d tasks", len(m.filtered), len(m.tasks))
	}

	return renderOverlayCard(b.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Latest Activity",
		Subtitle: subtitle,
		Centered: true,
	})
}

func (m *RecentActivityModel) renderTabBar() string {
	return RenderTabBar(recentActivityTabNames, m.activeTab, min(m.Width()-6, 54)-2, 1)
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
