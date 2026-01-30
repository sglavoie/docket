package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	filterContext       string   // Empty string means "all contexts"
	filterCustomContext string   // Custom context filter (mutually exclusive with filterContext)
	contexts            []string // Dynamically discovered contexts from projects

	// Confirmation state
	confirmAction string          // "delete" when confirming deletion
	deleteTarget  *models.Project // project to delete

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // "g" when waiting for second key
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel(cfg *config.Config) *DashboardModel {
	m := &DashboardModel{
		config:              cfg,
		cursor:              0,
		filterContext:       cfg.FilterContext,       // Load persisted context filter
		filterCustomContext: cfg.FilterCustomContext, // Load persisted custom context filter
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
		_ = storage.LoadProject(p)
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
	// Apply custom context filter first (list of specific project IDs)
	if m.filterCustomContext != "" {
		if !m.config.IsProjectInCustomContext(m.filterCustomContext, p.ID) {
			return false
		}
	} else if m.filterContext != "" && p.Context != m.filterContext {
		// Apply regular context filter
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
	if m.SearchQuery() == "" && m.filterContext == "" && m.filterCustomContext == "" {
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
	m.filterContext = cfg.FilterContext
	m.filterCustomContext = cfg.FilterCustomContext
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
			if m.filterContext != "" || m.filterCustomContext != "" || m.SearchQuery() != "" {
				m.filterContext = ""
				m.filterCustomContext = ""
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
		case "ctrl+r":
			m.loadProjects()
		case "R":
			return m, func() tea.Msg {
				return ToggleRecentTasksMsg{}
			}
		case "n":
			return m, func() tea.Msg {
				return CreateProjectMsg{}
			}
		case "N":
			return m, func() tea.Msg {
				return ShowGlobalNotesMsg{}
			}
		case "a":
			m.showArchived = !m.showArchived
			m.cursor = 0
			m.loadProjects()
		case ",":
			return m, func() tea.Msg {
				return SettingsMsg{}
			}
		case "S":
			sandbox := m.config.GetSandboxProject()
			if sandbox != nil {
				return m, func() tea.Msg {
					return SelectProjectMsg{Project: sandbox}
				}
			}
		case "D":
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				if project.Sandbox {
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("cannot delete the sandbox project")}
					}
				}
				m.confirmAction = "delete"
				m.deleteTarget = project
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
		case "r":
			return m, func() tea.Msg {
				return ShowDailyReviewMsg{}
			}
		case "f":
			return m, func() tea.Msg {
				return ShowFocusViewMsg{}
			}
		case "l":
			return m, func() tea.Msg {
				return ShowRecentActivityMsg{}
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
		case "W":
			return m, func() tea.Msg {
				return ShowWeeklyReviewMsg{}
			}
		case "w":
			return m, func() tea.Msg {
				return ShowGlobalInboxProcessorMsg{}
			}
		case "b":
			// Open blockers for selected project
			if len(visible) > 0 && m.cursor < len(visible) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return ShowBlockersMsg{Project: project}
				}
			}
		case "ctrl+w":
			return m, func() tea.Msg {
				return ShowWorkflowRulesMsg{}
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

// HasTextInput returns true when dashboard search is active.
func (m *DashboardModel) HasTextInput() bool {
	return m.HasActiveSearch()
}

// ShouldCaptureEsc returns true when dashboard search or prefix mode is active.
func (m *DashboardModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch() || m.HasPendingPrefix()
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

	// Filter bar: search and context on same line when both present
	chromeHeight := 0
	hasSearch := m.IsSearchMode() || m.SearchQuery() != ""
	hasContext := m.filterContext != ""
	hasCustomContext := m.filterCustomContext != ""

	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		if hasCustomContext {
			b.WriteString(" ")
			b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[+ %s]", m.filterCustomContext)))
		} else if hasContext {
			b.WriteString(" ")
			b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[%s]", m.filterContext)))
		}
		b.WriteString("\n")
		chromeHeight++
	} else if hasSearch || hasContext || hasCustomContext {
		var parts []string
		if m.SearchQuery() != "" {
			parts = append(parts, MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery())))
		}
		if hasCustomContext {
			parts = append(parts, FilterBadgeStyle.Render(fmt.Sprintf("[+ %s]", m.filterCustomContext)))
		} else if hasContext {
			parts = append(parts, FilterBadgeStyle.Render(fmt.Sprintf("[%s]", m.filterContext)))
		}
		b.WriteString(strings.Join(parts, " · "))
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(Esc clear)"))
		b.WriteString("\n")
		chromeHeight++
	}

	visible := m.visibleProjects()

	// Determine if we use split-pane layout
	useSplitPane := m.width >= 81

	if useSplitPane {
		b.WriteString(m.renderSplitPane(visible, chromeHeight))
	} else {
		b.WriteString(m.renderFullWidthDashboard(visible, chromeHeight))
	}

	// Prefix popup
	if popup := m.renderPrefixPopup(); popup != "" {
		b.WriteString("\n")
		b.WriteString(popup)
		b.WriteString("\n")
	}

	return b.String()
}

// renderSplitPane renders the dashboard as a left (project list) + right (info panel) layout.
func (m *DashboardModel) renderSplitPane(visible []*models.Project, chromeHeight int) string {
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth - 1 // 1 char divider

	cardWidth := leftWidth - 1
	if cardWidth < 20 {
		cardWidth = 20
	}

	// Build project card
	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, cardWidth, 0)

	cfg := DashboardCardConfig()
	leftContent := renderCard(cardTitle, cardContent, cardWidth, cfg)

	// Get selected project for right panel
	var selected *models.Project
	if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
		selected = visible[m.cursor]
	}

	rightContent := m.renderRightPanel(rightWidth, m.height-chromeHeight, selected)

	// Render vertical divider
	dividerHeight := m.height - chromeHeight
	if dividerHeight < 1 {
		dividerHeight = 1
	}
	divider := renderDashboardVerticalDivider(dividerHeight)

	// Join horizontally
	leftStyled := lipgloss.NewStyle().Width(leftWidth).Render(leftContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, divider, rightContent)
}

// renderFullWidthDashboard renders the original full-width dashboard layout.
func (m *DashboardModel) renderFullWidthDashboard(visible []*models.Project, chromeHeight int) string {
	var b strings.Builder

	cardWidth := m.width - 2
	if cardWidth < 20 {
		cardWidth = 20
	}

	// Calculate space for bottom section (Recent Tasks + Stats cards)
	bottomHeight := 0
	showRecentTasks := m.config.GetShowRecentTasks()
	if showRecentTasks && m.SearchQuery() == "" && m.config.CurrentProject != "" {
		current := m.config.GetCurrentProject()
		if current != nil && len(current.Tasks) > 0 {
			bottomHeight = 14
		}
	}

	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, cardWidth, bottomHeight)

	cfg := DashboardCardConfig()
	b.WriteString(renderCard(cardTitle, cardContent, cardWidth, cfg))
	b.WriteString("\n")

	// Current project details (only show when not searching and toggle is on)
	if showRecentTasks && m.SearchQuery() == "" && m.config.CurrentProject != "" {
		current := m.config.GetCurrentProject()
		if current != nil && len(current.Tasks) > 0 {
			b.WriteString("\n")

			recentContent := m.renderRecentTasksContentWithWidth(current, 40)
			b.WriteString(renderCard("Recent", recentContent, cardWidth, cfg))
			b.WriteString("\n")

			statsContent := m.renderStatsContent(current)
			if statsContent != "" {
				b.WriteString(renderCard("Stats", statsContent, cardWidth, cfg))
			}
		}
	}

	return b.String()
}

// projectCardTitle returns the title for the project list card.
func (m *DashboardModel) projectCardTitle(visible []*models.Project) string {
	if m.showArchived {
		return fmt.Sprintf("Archived (%d)", len(visible))
	}
	return fmt.Sprintf("Projects (%d)", len(visible))
}

// renderProjectList renders the project list content for a card of the given width.
// bottomHeight is reserved space below the card (only used in full-width mode).
func (m *DashboardModel) renderProjectList(visible []*models.Project, chromeHeight, cardWidth, bottomHeight int) string {
	var cardContent strings.Builder

	if len(m.projects) == 0 {
		if m.showArchived {
			cardContent.WriteString(MutedStyle.Render("No archived projects."))
		} else {
			cardContent.WriteString(MutedStyle.Render("No projects yet. Press n to create one."))
		}
		return cardContent.String()
	}
	if len(visible) == 0 {
		cardContent.WriteString(MutedStyle.Render("No projects match your filters."))
		return cardContent.String()
	}

	// Calculate max visible projects
	cardOverhead := 2
	maxVisible := m.height - chromeHeight - bottomHeight - cardOverhead - 1
	if maxVisible < 3 {
		maxVisible = 3
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}
	if scrollOffset > len(visible)-maxVisible {
		scrollOffset = len(visible) - maxVisible
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Scroll-up indicator inside card
	if scrollOffset > 0 {
		cardContent.WriteString(MutedStyle.Render(fmt.Sprintf("↑ %d more above", scrollOffset)))
		cardContent.WriteString("\n")
	}

	// Project list (windowed)
	end := scrollOffset + maxVisible
	if end > len(visible) {
		end = len(visible)
	}

	pinnedNum := 0
	for i := 0; i < scrollOffset; i++ {
		if visible[i].Pinned {
			pinnedNum++
		}
	}

	for i := scrollOffset; i < end; i++ {
		p := visible[i]
		selected := i == m.cursor
		style := NormalStyle
		numberPrefix := "  "
		if p.Pinned {
			pinnedNum++
			if pinnedNum <= 9 {
				numberPrefix = PinnedNumberStyle.Render(fmt.Sprintf("%d", pinnedNum)) + " "
			}
		}
		if selected {
			if p.Pinned {
				style = BoldStyle.Foreground(ColorSuccess)
			} else {
				style = BoldStyle.Foreground(ColorPrimary)
			}
		} else if p.Pinned {
			style = PinnedProjectStyle
		}

		total := p.CountTasks()
		active := p.CountActiveTasks()
		done := p.CountCompletedTasks()

		// Count urgency indicators
		overdue := p.CountOverdueTasks()
		dueSoonDays := m.config.GetReviewDueSoonDays()
		dueSoon := p.CountDueSoonTasks(dueSoonDays)
		blocked := p.CountBlockedTasks()
		blockerCount := storage.GetActiveBlockerCount(p)

		currentMarker := ""
		if p.ID == m.config.CurrentProject {
			currentMarker = " " + MutedStyle.Foreground(ColorSecondary).Render("*")
		}
		name := style.Render(p.Name) + currentMarker
		// Only show context badge if no context filter is active (avoids redundancy)
		context := ""
		if m.filterContext == "" && p.Context != "" {
			context = MutedStyle.Render(fmt.Sprintf("[%s]", p.Context))
		}
		stats := MutedStyle.Render(fmt.Sprintf("%d tasks, %d active", total, active))

		// Build status indicator: urgency if issues, completion otherwise
		var statusIndicator string
		if overdue > 0 || dueSoon > 0 || blocked > 0 || blockerCount > 0 {
			var parts []string
			if overdue > 0 {
				parts = append(parts, OverdueStyle.Render(fmt.Sprintf("🔥%d", overdue)))
			}
			if dueSoon > 0 {
				parts = append(parts, DueSoonStyle.Render(fmt.Sprintf("⏰%d", dueSoon)))
			}
			if blocked > 0 {
				parts = append(parts, MutedStyle.Render(fmt.Sprintf("◈%d", blocked)))
			}
			if blockerCount > 0 {
				parts = append(parts, DueSoonStyle.Render(fmt.Sprintf("🚧%d", blockerCount)))
			}
			statusIndicator = strings.Join(parts, " ")
		} else {
			statusIndicator = MutedStyle.Render(fmt.Sprintf("✓%d/%d", done, total))
		}

		content := fmt.Sprintf("%s%s %s  %s  %s", numberPrefix, name, context, stats, statusIndicator)
		line := RenderSelectableRow(selected, false, content, cardWidth-4) // -4 for card padding
		cardContent.WriteString(line)
		if i < end-1 || (len(visible)-end) > 0 {
			cardContent.WriteString("\n")
		}
	}

	// Scroll-down indicator inside card
	if remaining := len(visible) - end; remaining > 0 {
		cardContent.WriteString(MutedStyle.Render(fmt.Sprintf("↓ %d more below", remaining)))
	}

	return cardContent.String()
}

// renderRightPanel renders the right-side info panel with recent activity and global alerts.
func (m *DashboardModel) renderRightPanel(width, height int, selected *models.Project) string {
	cfg := DashboardCardConfig()
	cardWidth := width
	if cardWidth < 20 {
		cardWidth = 20
	}

	// Split height: recent activity gets top portion, alerts get bottom
	// Recent activity card: ~60% of height, alerts: ~40%
	alertsCardHeight := 8 // enough for 4 alert lines + card border
	recentHeight := height - alertsCardHeight
	if recentHeight < 6 {
		recentHeight = 6
	}

	// Recent Activity card
	var recentContent string
	if selected == nil || len(selected.Tasks) == 0 {
		recentContent = MutedStyle.Render("No project selected.")
	} else {
		// Adapt truncation width to available panel width
		truncWidth := cardWidth - 30 // space for #ID, badges, time
		if truncWidth < 15 {
			truncWidth = 15
		}
		recentContent = m.renderRecentTasksContentWithWidth(selected, truncWidth)
	}

	var recentTitle string
	if selected != nil {
		recentTitle = fmt.Sprintf("Recent — %s", util.Truncate(selected.Name, cardWidth-16))
	} else {
		recentTitle = "Recent Activity"
	}

	var b strings.Builder
	b.WriteString(renderCard(recentTitle, recentContent, cardWidth, cfg))
	b.WriteString("\n\n")

	// Alerts card
	alertsContent := m.renderGlobalAlerts(cardWidth)
	b.WriteString(renderCard("Alerts", alertsContent, cardWidth, cfg))

	return b.String()
}

// renderGlobalAlerts aggregates alert counts across all active projects.
func (m *DashboardModel) renderGlobalAlerts(width int) string {
	var totalOverdue, totalDueToday, totalBlocked int
	var inboxProjects []string

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Reuse already-loaded projects when viewing active; otherwise load fresh
	var activeProjects []*models.Project
	if !m.showArchived {
		activeProjects = m.projects
	} else {
		activeProjects = m.config.GetActiveProjects()
		for _, p := range activeProjects {
			_ = storage.LoadProject(p)
		}
	}
	for _, p := range activeProjects {

		overdue := p.CountOverdueTasks()
		totalOverdue += overdue

		blocked := p.CountBlockedTasks()
		totalBlocked += blocked

		// Count due-today tasks
		for _, task := range p.Tasks {
			if task.DueDate != nil && !task.Status.IsComplete() {
				dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
				if dueDay.Equal(today) {
					totalDueToday++
				}
			}
		}

		// Count inbox tasks
		inboxCount := 0
		for _, task := range p.Tasks {
			if task.Section == models.SectionInbox && !task.Status.IsComplete() {
				inboxCount++
			}
		}
		if inboxCount >= 5 {
			inboxProjects = append(inboxProjects, fmt.Sprintf("%s(%d)", p.Name, inboxCount))
		}
	}

	var lines []string
	if totalOverdue > 0 {
		lines = append(lines, OverdueStyle.Render(fmt.Sprintf("🔥 %d overdue", totalOverdue)))
	}
	if totalDueToday > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("⏰ %d due today", totalDueToday)))
	}
	if totalBlocked > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("◈ %d blocked", totalBlocked)))
	}
	if len(inboxProjects) > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("📥 Inbox backlog: %s", strings.Join(inboxProjects, ", "))))
	}

	if len(lines) == 0 {
		return MutedStyle.Render("No alerts")
	}
	return strings.Join(lines, "\n")
}

// renderDashboardVerticalDivider renders a vertical divider for the dashboard split pane.
func renderDashboardVerticalDivider(height int) string {
	if height <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < height; i++ {
		b.WriteString("│")
		if i < height-1 {
			b.WriteString("\n")
		}
	}
	return MutedStyle.Render(b.String())
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
func (m *DashboardModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		"g": NavigationPrefixHints,
	}, m.width)
}

func (m *DashboardModel) renderRecentTasksContent(project *models.Project) string {
	return m.renderRecentTasksContentWithWidth(project, 40)
}

func (m *DashboardModel) renderRecentTasksContentWithWidth(project *models.Project, truncWidth int) string {
	recentTasks := storage.GetRecentTasks(project.Tasks, 5)
	if len(recentTasks) == 0 {
		return MutedStyle.Render("No tasks yet.")
	}

	var lines []string
	for _, task := range recentTasks {
		line := fmt.Sprintf("%s %s %s %s  %s",
			MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)),
			GetTypeBadge(string(task.Type), task.Type.Icon()),
			GetStatusBadge(string(task.Status), task.Status.Icon()),
			util.Truncate(task.Title, truncWidth),
			MutedStyle.Render(storage.FormatRelativeTime(task.UpdatedAt)))

		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m *DashboardModel) renderStatsContent(project *models.Project) string {
	if len(project.Tasks) == 0 {
		return ""
	}

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

	var lines []string
	lines = append(lines, MutedStyle.Render("Types: ")+strings.Join(typeStats, " "))
	lines = append(lines, MutedStyle.Render("Status: ")+strings.Join(statusStats, " "))

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
		lines = append(lines, OverdueStyle.Render(fmt.Sprintf("Overdue: %d", overdueCount)))
	}

	return strings.Join(lines, "\n")
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
