package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ProjectFocusViewModel displays active-flagged tasks for a single project.
type ProjectFocusViewModel struct {
	BaseView
	InteractiveViewState

	project *models.Project
	cfg     *config.Config

	// Tasks
	tasks []*models.Task

	// Sub-tabs
	subTab           int
	subTabRowFocused bool
	subTabNames      []string
}

// NewProjectFocusViewModel creates a new project focus view model.
func NewProjectFocusViewModel(project *models.Project, cfg *config.Config) *ProjectFocusViewModel {
	m := &ProjectFocusViewModel{
		project: project,
		cfg:     cfg,
	}
	m.loadTasks()
	return m
}

// loadTasks collects active-flagged tasks from the project.
func (m *ProjectFocusViewModel) loadTasks() {
	m.tasks = nil

	for _, task := range m.project.Tasks {
		if !task.Active {
			continue
		}
		// Skip completed tasks
		if task.Status.IsComplete() {
			continue
		}
		// Skip snoozed tasks
		if task.IsSnoozed() {
			continue
		}
		m.tasks = append(m.tasks, task)
	}

	// Sort by priority (ascending, 1=highest), then by UpdatedAt (most recent first)
	sort.Slice(m.tasks, func(i, j int) bool {
		if m.tasks[i].Priority != m.tasks[j].Priority {
			return m.tasks[i].Priority < m.tasks[j].Priority
		}
		return m.tasks[i].UpdatedAt.After(m.tasks[j].UpdatedAt)
	})

	m.computeSubTabs()
}

// computeSubTabs builds dynamic sub-tab names from unique CustomSection values.
func (m *ProjectFocusViewModel) computeSubTabs() {
	seen := map[string]bool{}
	var names []string
	for _, task := range m.tasks {
		section := task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if !seen[section] {
			seen[section] = true
			names = append(names, section)
		}
	}
	sort.Strings(names)
	m.subTabNames = names

	if m.subTab > len(m.subTabNames) {
		m.subTab = 0
	}
}

// activeSubTabFilter returns the selected sub-tab value, or "" if no sub-tab filtering.
func (m *ProjectFocusViewModel) activeSubTabFilter() string {
	if len(m.subTabNames) <= 1 || m.subTab <= 0 || m.subTab > len(m.subTabNames) {
		return ""
	}
	return m.subTabNames[m.subTab-1]
}

// filteredTasks returns tasks filtered by the active sub-tab.
func (m *ProjectFocusViewModel) filteredTasks() []*models.Task {
	filter := m.activeSubTabFilter()
	if filter == "" {
		return m.tasks
	}
	var result []*models.Task
	for _, task := range m.tasks {
		section := task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if section == filter {
			result = append(result, task)
		}
	}
	return result
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *ProjectFocusViewModel) getTaskAtCursor() *models.Task {
	tasks := m.filteredTasks()
	if m.cursor < 0 || m.cursor >= len(tasks) {
		return nil
	}
	return tasks[m.cursor]
}

// HasActiveSearch returns false (no search in this view).
func (m *ProjectFocusViewModel) HasActiveSearch() bool {
	return false
}

func (m *ProjectFocusViewModel) Init() tea.Cmd {
	return nil
}

func (m *ProjectFocusViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(
		msg,
		m.SetSize,
		func() { _ = defaultPersister.Load(m.project); m.loadTasks() },
		m.clampCursor,
		m.setStatus,
	); handled {
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

// handleKeyMsg handles keyboard input.
func (m *ProjectFocusViewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.ClearError()
	m.clearStatus()

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, ",dg", "", false); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Sub-tab navigation
	switch key {
	case "(", ")":
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = !m.subTabRowFocused
		}
		return m, nil
	case "[", "h":
		if len(m.subTabNames) > 1 {
			CycleTab(&m.subTab, len(m.subTabNames)+1, -1)
			m.clampCursor()
			return m, nil
		}
	case "]", "l":
		if len(m.subTabNames) > 1 {
			CycleTab(&m.subTab, len(m.subTabNames)+1, 1)
			m.clampCursor()
			return m, nil
		}
	}

	tasks := m.filteredTasks()

	if handled, cmd := handleListNavigation(key, &m.cursor, len(tasks), m.ContentHeight(), &m.cursorFlash); handled {
		return m, cmd
	}

	switch key {
	case "esc":
		return m, func() tea.Msg {
			return CloseProjectFocusViewMsg{}
		}

	case "enter":
		// Close and return to project tree (task is already in this project)
		return m, func() tea.Msg {
			return CloseProjectFocusViewMsg{}
		}

	// Toggle active flag (remove from focus)
	case "*":
		return m.toggleActive()

	// Priority changes
	case "1", "2", "3", "4", "5":
		return m.changeTaskPriority(key)

	// Edit task
	case "e":
		if task := m.getTaskAtCursor(); task != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: task}
			}
		}
		return m, nil

	// Refresh
	case "r":
		_ = defaultPersister.Load(m.project)
		m.loadTasks()
		m.clampCursor()
		m.setStatus("Refreshed")
		return m, nil

	// Copy
	case "y", "Y", "ctrl+y":
		if cmd, handled := clipboardCmd(key, m.getTaskAtCursor()); handled {
			return m, cmd
		}
		return m, nil

	// Help
	case "?":
		return m, helpCmd(ViewProjectFocusView, "Project Focus View", m.GetHelpItems())
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *ProjectFocusViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, nil); handled {
		return model, cmd
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// prefixDispatchView implementation for ProjectFocusViewModel.

func (m *ProjectFocusViewModel) getTaskAndProject() (*models.Task, *models.Project) {
	return m.getTaskRef()
}

func (m *ProjectFocusViewModel) selectMsg(_ *models.Task, _ *models.Project) tea.Msg {
	return CloseProjectFocusViewMsg{}
}

func (m *ProjectFocusViewModel) handleFilterPrefix(_ string) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *ProjectFocusViewModel) handleFilterSubPrefix(_, _ string) (tea.Model, tea.Cmd) {
	return m, nil
}

// taskOpsView implementation for ProjectFocusViewModel.

func (m *ProjectFocusViewModel) getTaskRef() (*models.Task, *models.Project) {
	task := m.getTaskAtCursor()
	if task == nil {
		return nil, nil
	}
	return task, m.project
}

func (m *ProjectFocusViewModel) getPersister() Persister           { return defaultPersister }
func (m *ProjectFocusViewModel) getConfig() *config.Config         { return m.cfg }
func (m *ProjectFocusViewModel) getConfigManager() *config.Manager { return nil }
func (m *ProjectFocusViewModel) reloadData()                       { m.loadTasks() }
func (m *ProjectFocusViewModel) setErr(err error)                  { m.SetError(err) }
func (m *ProjectFocusViewModel) setStatus(msg string)              { m.InteractiveViewState.setStatus(msg) }

func (m *ProjectFocusViewModel) toggleActive() (tea.Model, tea.Cmd) {
	if task := m.getTaskAtCursor(); task != nil {
		return taskOpsRemoveFromFocus(m, task.ShortID)
	}
	return m, nil
}

func (m *ProjectFocusViewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	return taskOpsChangePriority(m, key)
}

// clampCursor ensures cursor is within valid range.
func (m *ProjectFocusViewModel) clampCursor() {
	m.ClampCursorTo(len(m.filteredTasks()))
}

// GetHelpItems returns help items for this view.
func (m *ProjectFocusViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewProjectFocusView)
}

func (m *ProjectFocusViewModel) HandledKeys() []string {
	return concatKeys(
		listNavigationKeys(),
		statusPrefixKeys(),
		dueDatePrefixKeys(),
		[]string{
			"g", "[", "]",
			"enter", "*", "e", "r", "y", "Y",
			"1", "2", "3", "4", "5",
			"esc",
		},
	)
}

func (m *ProjectFocusViewModel) View() tea.View {
	tasks := m.filteredTasks()

	var b strings.Builder

	subtitle := fmt.Sprintf("%d active tasks", len(m.tasks))
	if len(m.tasks) == 0 {
		subtitle = "no active tasks"
	}
	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Focus: %s", m.project.Name), subtitle, m.AccentColor()))
	b.WriteString("\n\n")

	if len(m.tasks) == 0 {
		b.WriteString(MutedStyle.Render("No tasks in focus."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Mark tasks as active with * in the project tree."))
		b.WriteString("\n")
		left := HelpStyle.Render("q return")
		b.WriteString(renderStatusBarFooter(left, "", m.Width(), m.AccentColor()))
		return tea.NewView(b.String())
	}

	contentHeight := max(m.Height()-OverlayListChrome, 1)

	// Sub-tab bar
	subTabBar := m.renderSubTabBar()
	if subTabBar != "" {
		b.WriteString(subTabBar)
		b.WriteString("\n")
		contentHeight -= 2
	}

	// Compute column layout for badge alignment.
	var colLayout *ColumnLayout
	if m.Width() >= MinColumnAlignWidth {
		colLayout = ComputeTaskLineColumnLayout(tasks, false)
	}

	// Render task lines
	var lines []string
	maxTitleLen := m.Width() - 35

	for i, task := range tasks {
		isSelected := i == m.cursor
		line := m.renderTaskLine(task, isSelected, maxTitleLen, colLayout, i)
		lines = append(lines, line)
	}

	if len(tasks) == 0 {
		b.WriteString(MutedStyle.Render(EmptyStateFiltered))
		b.WriteString("\n\n")
	} else {
		b.WriteString(strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n"))
		b.WriteString("\n\n")
	}

	b.WriteString(renderStatusMessages(m.statusMessage, m.Error()))

	// Footer
	footerParts := "j/k nav · * unfocus · ,s/d/b/x status · 1-5 priority · dt/dm due"
	if len(m.subTabNames) > 1 {
		footerParts += " · [/] sections"
	}
	footerParts += " · ? help"
	left := HelpStyle.Render(footerParts)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(tasks)))
	b.WriteString(renderStatusBarFooter(left, right, m.Width(), m.AccentColor()))

	return tea.NewView(b.String())
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *ProjectFocusViewModel) renderSubTabBar() string {
	if len(m.subTabNames) <= 1 {
		return ""
	}

	tabs := make([]string, 0, len(m.subTabNames)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, m.subTabNames...)

	bar := RenderTabBar(tabs, m.subTab, min(m.Width()-6, 54)-2, 1, m.AccentColor())

	if m.subTabRowFocused {
		bar = FocusIndicator + bar
	} else {
		bar = " " + bar
	}
	return bar
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *ProjectFocusViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			",": StatusPrefixHints,
			"d": DueDatePrefixHints,
			"g": ProjectFocusNavigationPrefixHints,
		},
	}
}

// ProjectFocusNavigationPrefixHints contains "g" prefix hints for project focus view.
var ProjectFocusNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
}

// renderTaskLine renders a single task line.
func (m *ProjectFocusViewModel) renderTaskLine(task *models.Task, isSelected bool, maxTitleLen int, layout *ColumnLayout, rowIndex int) string {
	csStr := ""
	if task.CustomSection != "" {
		csColor := m.project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csStr = csStyle.Render("@ " + task.CustomSection)
	}

	dueStr := ""
	if task.DueDate != nil {
		dueStr = FormatRelativeDueTask(task, true, m.getConfig().GetTimeFormat())
	}

	// Build indicator groups
	orgGroup := csStr
	structureGroup := renderProgressFraction(task)
	contentGroup := renderTruncatedTags(task.Tags, 2, 8)
	postTitle := JoinIndicatorGroups(orgGroup, structureGroup, contentGroup)
	if postTitle != "" {
		postTitle = " " + postTitle
	}

	return TaskLine{
		Task:       task,
		IsSelected: isSelected,
		Active:     task.Active,
		Pinned:     task.Pinned,
		MaxTitle:   maxTitleLen,
		StageBadge: GetStageBadge(task.Stage),
		PostTitle:  postTitle,
		DueStr:     dueStr,
		ViewWidth:  m.Width(),
		Flash:      m.cursorFlash,
		Layout:     layout,
		RowIndex:   rowIndex,
	}.Render()
}

func (m *ProjectFocusViewModel) StatusBarLabel() string { return "Active Tasks" }
