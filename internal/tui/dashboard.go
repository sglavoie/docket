package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// DashboardModel represents the dashboard view.
type DashboardModel struct {
	BaseSearchableView
	config       *config.Config
	projects     []*models.Project
	cursor       int
	showArchived bool

	// Context filter state
	filterContext string   // Empty string means "all contexts"
	contexts      []string // Dynamically discovered contexts from projects

	// Confirmation state
	confirmAction string          // "delete" when confirming deletion
	deleteTarget  *models.Project // project to delete

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // "g" when waiting for second key
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel(cfg *config.Config) *DashboardModel {
	m := &DashboardModel{
		config: cfg,
		cursor: 0,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.loadProjects()
	return m
}

func (m *DashboardModel) loadProjects() {
	if m.showArchived {
		m.projects = m.config.GetArchivedProjects()
	} else {
		m.projects = m.config.GetActiveProjects()
	}
	for _, p := range m.projects {
		storage.LoadProject(p)
	}
	m.contexts = m.getAllContexts()
}

// getAllContexts returns a sorted list of unique contexts from loaded projects.
func (m *DashboardModel) getAllContexts() []string {
	contextSet := make(map[string]bool)
	for _, p := range m.projects {
		if p.Context != "" {
			contextSet[p.Context] = true
		}
	}
	contexts := make([]string, 0, len(contextSet))
	for ctx := range contextSet {
		contexts = append(contexts, ctx)
	}
	sort.Strings(contexts)
	return contexts
}

func (m *DashboardModel) matchesFilters(p *models.Project) bool {
	// Apply context filter first
	if m.filterContext != "" && p.Context != m.filterContext {
		return false
	}

	// Then apply search filter
	if m.SearchQuery() == "" {
		return true
	}
	query := strings.TrimSpace(m.SearchQuery())

	if fuzzy.MatchTerms(p.Name, query) {
		return true
	}
	if fuzzy.MatchTerms(p.Context, query) {
		return true
	}
	if fuzzy.MatchTerms(p.Description, query) {
		return true
	}
	return false
}

func (m *DashboardModel) visibleProjects() []*models.Project {
	if m.SearchQuery() == "" && m.filterContext == "" {
		return m.projects
	}
	var visible []*models.Project
	for _, p := range m.projects {
		if m.matchesFilters(p) {
			visible = append(visible, p)
		}
	}
	return visible
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *DashboardModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// Refresh reloads the dashboard data.
func (m *DashboardModel) Refresh(cfg *config.Config) {
	m.config = cfg
	m.loadProjects()
}

// ResetCursor moves the cursor to the first project.
func (m *DashboardModel) ResetCursor() {
	m.cursor = 0
}

// SetSize sets the dashboard dimensions.
func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *DashboardModel) Init() tea.Cmd {
	return nil
}

func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleProjects()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle confirmation mode
		if m.confirmAction != "" {
			switch msg.String() {
			case "y", "Y":
				if m.confirmAction == "delete" && m.deleteTarget != nil {
					project := m.deleteTarget
					m.confirmAction = ""
					m.deleteTarget = nil
					return m, m.deleteProject(project)
				}
			case "n", "N", "esc":
				m.confirmAction = ""
				m.deleteTarget = nil
				return m, nil
			}
			return m, nil
		}

		// Handle search mode
		if m.IsSearchMode() {
			handled, cmd, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				visible = m.visibleProjects()
				m.cursor = ClampCursor(m.cursor, len(visible))
			}
			if handled {
				return m, cmd
			}
			return m, cmd
		}

		// Handle pending prefix keys (Vim-style gg)
		if m.pendingPrefix != "" {
			return m.handlePrefixKey(msg.String(), visible)
		}

		switch msg.String() {
		case "\\":
			return m, m.ActivateSearch()
		case "/":
			return m, func() tea.Msg {
				return ShowGlobalSearchMsg{}
			}
		case "esc":
			// Clear filters if active
			if m.filterContext != "" || m.SearchQuery() != "" {
				m.filterContext = ""
				m.ClearSearch()
				return m, nil
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(visible)-1 {
				m.cursor++
			}
		case "enter":
			if CursorInBounds(m.cursor, len(visible)) {
				return m, func() tea.Msg {
					return SelectProjectMsg{Project: visible[m.cursor]}
				}
			}
		case "?":
			return m, func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewDashboard,
					ViewName:   "Dashboard",
					Items:      m.GetHelpItems(),
				}
			}
		case "r":
			m.loadProjects()
		case "R":
			return m, func() tea.Msg {
				return ToggleRecentTasksMsg{}
			}
		case "n":
			return m, func() tea.Msg {
				return CreateProjectMsg{}
			}
		case "a":
			m.showArchived = !m.showArchived
			m.cursor = 0
			m.loadProjects()
		case ",":
			return m, func() tea.Msg {
				return SettingsMsg{}
			}
		case "D":
			if CursorInBounds(m.cursor, len(visible)) {
				m.confirmAction = "delete"
				m.deleteTarget = visible[m.cursor]
			}
		case "e":
			if CursorInBounds(m.cursor, len(visible)) {
				return m, func() tea.Msg {
					return EditProjectMsg{Project: visible[m.cursor]}
				}
			}
		case "p":
			if CursorInBounds(m.cursor, len(visible)) {
				return m, func() tea.Msg {
					return ShowProjectPreviewMsg{Project: visible[m.cursor]}
				}
			}
		case "c":
			// Open context picker popup
			return m, func() tea.Msg {
				return ShowContextPickerMsg{Dashboard: m}
			}
		case "ctrl+d":
			return m, func() tea.Msg {
				return ShowDueDateOverviewMsg{}
			}
		case "ctrl+t":
			return m, func() tea.Msg {
				return ShowGlobalTagManagerMsg{}
			}
		case "@":
			return m, func() tea.Msg {
				return ShowProjectCycleManagerMsg{}
			}
		case "`":
			return m, func() tea.Msg {
				return CycleProjectMsg{Forward: true}
			}
		case "~":
			return m, func() tea.Msg {
				return CycleProjectMsg{Forward: false}
			}
		case "ctrl+p":
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, m.toggleProjectPinned(project)
			}
		case "s":
			if CursorInBounds(m.cursor, len(visible)) {
				return m, func() tea.Msg {
					return SetCurrentProjectMsg{Project: visible[m.cursor]}
				}
			}
		case "T":
			return m, func() tea.Msg {
				return ShowTrashedProjectsMsg{}
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Quick select pinned project by number
			n := int(msg.String()[0] - '0')
			pinnedCount := 0
			for _, p := range visible {
				if p.Pinned {
					pinnedCount++
					if pinnedCount == n {
						project := p // capture for closure
						return m, func() tea.Msg {
							return SelectProjectMsg{Project: project}
						}
					}
				}
			}
		case "g":
			// Start prefix sequence for gg (Vim-style)
			m.pendingPrefix = "g"
			return m, nil
		case "G":
			// Go to last item (Vim-style)
			if len(visible) > 0 {
				m.cursor = len(visible) - 1
			}
		}
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *DashboardModel) handlePrefixKey(key string, visible []*models.Project) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	if prefix == "g" && key == "g" {
		// gg: Go to first item
		m.cursor = 0
		return m, nil
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *DashboardModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

func (m *DashboardModel) deleteProject(project *models.Project) tea.Cmd {
	return func() tea.Msg {
		// Purge any existing trash entry with same slug
		if err := storage.PurgeFromTrash(project.Slug, m.config.DataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to purge old trash entry: %w", err)}
		}

		// Move project directory to trash
		if err := storage.MoveProjectToTrash(project, m.config.DataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to move project to trash: %w", err)}
		}

		return ProjectDeletedMsg{Project: project}
	}
}

func (m *DashboardModel) toggleProjectPinned(project *models.Project) tea.Cmd {
	return func() tea.Msg {
		project.TogglePinned()
		return ProjectPinnedToggledMsg{Project: project}
	}
}

func (m *DashboardModel) View() string {
	var b strings.Builder

	// Confirmation prompt
	if m.confirmAction == "delete" && m.deleteTarget != nil {
		b.WriteString("\n")
		b.WriteString(ConfirmStyle.Render(fmt.Sprintf("Delete project '%s'? (y/n)", m.deleteTarget.Name)))
		b.WriteString("\n\n")
	}

	// Search bar
	chromeHeight := 0
	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
		chromeHeight++
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Search: \"%s\" ", m.SearchQuery())))
		b.WriteString(HelpStyle.Render("(\\ to edit, Esc to clear)"))
		b.WriteString("\n")
		chromeHeight++
	}

	// Context filter
	if m.filterContext != "" {
		b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[context: %s]", m.filterContext)))
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(c to cycle, Esc to clear)"))
		b.WriteString("\n")
		chromeHeight++
	}
	b.WriteString("\n")
	chromeHeight++ // separator line

	visible := m.visibleProjects()

	if len(m.projects) == 0 {
		if m.showArchived {
			b.WriteString(MutedStyle.Render("No archived projects."))
		} else {
			b.WriteString(MutedStyle.Render("No projects yet. Create one with: tt project new <name>"))
		}
		b.WriteString("\n")
	} else if len(visible) == 0 {
		b.WriteString(MutedStyle.Render("No projects match your filters."))
		b.WriteString("\n")
	} else {
		// Projects header
		if m.showArchived {
			b.WriteString(SubtitleStyle.Render("ARCHIVED PROJECTS"))
		} else {
			b.WriteString(SubtitleStyle.Render("PROJECTS"))
		}
		b.WriteString("\n")
		chromeHeight++ // header line

		// Calculate space for bottom section (Recent Tasks + Stats)
		bottomHeight := 0
		showRecentTasks := m.config.GetShowRecentTasks()
		if showRecentTasks && m.SearchQuery() == "" && m.config.CurrentProject != "" {
			current := m.config.GetCurrentProject()
			if current != nil && len(current.Tasks) > 0 {
				// Recent Tasks: 2 blank lines + header + up to 5 tasks = ~8 lines
				// Stats: types line + status line + optional overdue = ~3 lines
				bottomHeight = 12
			}
		}

		// Calculate max visible projects
		// Reserve space for scroll indicators (up to 2 lines if both needed)
		scrollIndicatorSpace := 2
		maxVisible := m.height - chromeHeight - bottomHeight - scrollIndicatorSpace
		if maxVisible < 3 {
			maxVisible = 3 // minimum visible projects
		}

		// Calculate scroll offset to keep cursor visible
		scrollOffset := 0
		if m.cursor >= maxVisible {
			scrollOffset = m.cursor - maxVisible + 1
		}

		// Ensure we don't scroll past the end
		if scrollOffset > len(visible)-maxVisible {
			scrollOffset = len(visible) - maxVisible
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}

		// Show scroll-up indicator
		if scrollOffset > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ↑ %d more above", scrollOffset)))
			b.WriteString("\n")
		}

		// Project list (windowed)
		end := scrollOffset + maxVisible
		if end > len(visible) {
			end = len(visible)
		}

		pinnedNum := 0
		// Count pinned projects before scroll offset for correct numbering
		for i := 0; i < scrollOffset; i++ {
			if visible[i].Pinned {
				pinnedNum++
			}
		}

		for i := scrollOffset; i < end; i++ {
			p := visible[i]
			cursor := "  "
			style := NormalStyle
			numberPrefix := "  " // Two spaces to align with numbered items
			if p.Pinned {
				pinnedNum++
				if pinnedNum <= 9 {
					numberPrefix = PinnedNumberStyle.Render(fmt.Sprintf("%d", pinnedNum)) + " "
				}
			}
			if i == m.cursor {
				cursor = "> "
				if p.Pinned {
					style = BoldStyle.Foreground(ColorSuccess)
				} else {
					style = BoldStyle.Foreground(ColorPrimary)
				}
			} else if p.Pinned {
				style = PinnedProjectStyle
			}

			// Calculate stats
			total := p.CountTasks()
			active := p.CountActiveTasks()
			done := p.CountCompletedTasks()

			progress := 0.0
			if total > 0 {
				progress = float64(done) / float64(total) * 100
			}

			// Format line
			currentMarker := ""
			if p.ID == m.config.CurrentProject {
				currentMarker = " " + MutedStyle.Foreground(ColorSecondary).Render("*")
			}
			name := style.Render(p.Name) + currentMarker
			context := MutedStyle.Render(fmt.Sprintf("[%s]", p.Context))
			stats := MutedStyle.Render(fmt.Sprintf("%d tasks, %d active", total, active))
			progressBar := RenderProgressBar(progress, 10)

			line := fmt.Sprintf("%s%s%s %s  %s  %s", numberPrefix, cursor, name, context, stats, progressBar)
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Show scroll-down indicator
		if remaining := len(visible) - end; remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ↓ %d more below", remaining)))
			b.WriteString("\n")
		}

		// Current project details (only show when not searching and toggle is on)
		if showRecentTasks && m.SearchQuery() == "" && m.config.CurrentProject != "" {
			current := m.config.GetCurrentProject()
			if current != nil {
				b.WriteString("\n")
				m.renderRecentTasks(&b, current)
				m.renderStats(&b, current)
			}
		}
	}

	// Prefix popup
	if popup := m.renderPrefixPopup(); popup != "" {
		b.WriteString("\n")
		b.WriteString(popup)
		b.WriteString("\n")
	}

	return b.String()
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
func (m *DashboardModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		"g": NavigationPrefixHints,
	}, m.width)
}

func (m *DashboardModel) renderRecentTasks(b *strings.Builder, project *models.Project) {
	recentTasks := storage.GetRecentTasks(project.Tasks, 5)
	if len(recentTasks) == 0 {
		b.WriteString(MutedStyle.Render("No tasks yet."))
		b.WriteString("\n")
		return
	}

	b.WriteString("\n")
	b.WriteString(BoldStyle.Render("Recent Tasks"))
	b.WriteString("\n")

	for _, task := range recentTasks {
		typeStyle := GetTypeStyle(string(task.Type))
		statusStyle := GetStatusStyle(string(task.Status))

		line := fmt.Sprintf("  %s %s %s %s  %s",
			MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)),
			typeStyle.Render(task.Type.Icon()),
			statusStyle.Render(task.Status.Icon()),
			util.Truncate(task.Title, 40),
			MutedStyle.Render(storage.FormatRelativeTime(task.UpdatedAt)))

		b.WriteString(line)
		b.WriteString("\n")
	}
}

func (m *DashboardModel) renderStats(b *strings.Builder, project *models.Project) {
	if len(project.Tasks) == 0 {
		return
	}

	b.WriteString("\n")

	// Count by type
	typeCounts := make(map[models.TaskType]int)
	statusCounts := make(map[models.TaskStatus]int)

	for _, task := range project.Tasks {
		typeCounts[task.Type]++
		statusCounts[task.Status]++
	}

	// Type stats
	var typeStats []string
	for _, tt := range models.AllTaskTypes() {
		if count := typeCounts[tt]; count > 0 {
			style := GetTypeStyle(string(tt))
			typeStats = append(typeStats, style.Render(fmt.Sprintf("%s:%d", tt.Icon(), count)))
		}
	}

	// Status stats
	var statusStats []string
	for _, s := range models.AllTaskStatuses() {
		if count := statusCounts[s]; count > 0 {
			style := GetStatusStyle(string(s))
			statusStats = append(statusStats, style.Render(fmt.Sprintf("%s:%d", s.Icon(), count)))
		}
	}

	b.WriteString(MutedStyle.Render("Types: "))
	b.WriteString(strings.Join(typeStats, " "))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Status: "))
	b.WriteString(strings.Join(statusStats, " "))
	b.WriteString("\n")

	// Overdue count
	overdueCount := 0
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, task := range project.Tasks {
		if task.DueDate != nil && !task.Status.IsComplete() {
			dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
			if dueDay.Before(today) {
				overdueCount++
			}
		}
	}
	if overdueCount > 0 {
		b.WriteString(OverdueStyle.Render(fmt.Sprintf("Overdue: %d", overdueCount)))
		b.WriteString("\n")
	}
}

// GetHelpItems returns the help items for the dashboard view.
func (m *DashboardModel) GetHelpItems() []HelpItem {
	toggleArchiveDesc := "Show archived"
	if m.showArchived {
		toggleArchiveDesc = "Show active"
	}
	toggleRecentDesc := "Hide Recent Tasks"
	if !m.config.GetShowRecentTasks() {
		toggleRecentDesc = "Show Recent Tasks"
	}
	overrides := map[string]string{
		"a": toggleArchiveDesc,
		"R": toggleRecentDesc,
	}
	return GetKeybindingRegistry().GetForViewWithOverrides(ViewDashboard, overrides)
}
