package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var focusBoardTabNames = []string{"All", "Actions", "Active/Due", "By Source", "By Priority", "By Score", "Closed"}

const (
	boardTabAll        = 0
	boardTabActions    = 1
	boardTabActiveDue  = 2
	boardTabBySource   = 3
	boardTabByPriority = 4
	boardTabByScore    = 5
	boardTabClosed     = 6
)

// boardRow is a unified display item: an Action task or an aggregated active/due task.
type boardRow struct {
	Task     *models.Task
	Project  *models.Project
	Priority int
	Source   string
}

func (r boardRow) isAction() bool {
	return r.Task != nil && r.Task.Type == models.TaskTypeAction
}

func (r boardRow) isTask() bool {
	return r.Task != nil && r.Task.Type != models.TaskTypeAction
}

func (r boardRow) title() string {
	if r.Task != nil {
		return r.Task.Title
	}
	return ""
}

// FocusBoardModel displays Action tasks + aggregated active/due tasks in a unified view.
type FocusBoardModel struct {
	BaseView
	InteractiveViewState

	config         *config.Config
	configManager  *config.Manager
	projectService *ProjectService

	// Data
	rows       []boardRow
	closedRows []boardRow
	filtered   []boardRow

	// Tabs
	activeTab int

	// Sort
	sortField     SortField
	sortAscending bool

	// Filter
	filter        CommonFilterState
	pendingPrefix string

	// Search
	searchMode  bool
	searchInput textinput.Model
	searchQuery string

	// Quick-add input
	addMode  bool
	addInput textinput.Model

	// Autocomplete state for add mode
	acMode         int
	acSuggestions  []suggestion
	acCursor       int
	acViewportTop  int
	acTriggerPos   int
	acProjectCache []*models.Project
	acTagCache     []string

	// Quick-edit mode
	editMode  bool
	editInput textinput.Model
	editIdx   int
}

// NewFocusBoardModel creates a new focus board model.
func NewFocusBoardModel(cfg *config.Config, manager *config.Manager) *FocusBoardModel {
	ti := textinput.New()
	ti.Placeholder = "Action title (%project !priority #tag -s source -u url)"
	ti.CharLimit = 200
	ti.Width = 60

	ei := textinput.New()
	ei.Placeholder = "New title"
	ei.CharLimit = 200
	ei.Width = 60

	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 100
	si.Width = 40

	m := &FocusBoardModel{
		config:         cfg,
		configManager:  manager,
		projectService: NewProjectService(),
		addInput:       ti,
		editInput:      ei,
		searchInput:    si,
		filter: CommonFilterState{
			Tags:  make(map[string]bool),
			Attrs: make(map[string]string),
		},
	}
	m.loadItems()
	return m
}

// loadItems loads Action tasks and aggregates active/due tasks across all projects.
func (m *FocusBoardModel) loadItems() {
	m.rows = nil
	m.closedRows = nil

	now := time.Now()
	today := util.StartOfDay(now)

	// Iterate all loadable projects (including internal ones like action-inbox)
	projects := m.config.GetAllLoadableProjects()
	for _, project := range projects {
		if project.Sandbox {
			continue
		}
		if err := m.projectService.Load(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			if task.Type == models.TaskTypeAction {
				row := boardRow{
					Task:     task,
					Project:  project,
					Priority: task.Priority,
					Source:   task.Source,
				}
				if task.Status.IsComplete() {
					m.closedRows = append(m.closedRows, row)
				} else {
					m.rows = append(m.rows, row)
				}
				continue
			}

			// Non-action tasks: include if active or due soon
			if task.Status.IsComplete() || task.IsSnoozed() {
				continue
			}
			isActive := task.Active
			isDueSoon := task.DueDate != nil && util.StartOfDay(*task.DueDate).Before(today.AddDate(0, 0, 1))
			if isActive || isDueSoon {
				m.rows = append(m.rows, boardRow{
					Task:     task,
					Project:  project,
					Priority: task.Priority,
					Source:   project.Name,
				})
			}
		}
	}

	// Sort by priority ascending, then preserve original order within same priority
	sort.SliceStable(m.rows, func(i, j int) bool {
		if m.rows[i].Priority != m.rows[j].Priority {
			return m.rows[i].Priority < m.rows[j].Priority
		}
		return false
	})

	m.applyFilter()
	m.initBoardCaches()
}

// applyFilter filters rows based on the active tab.
func (m *FocusBoardModel) applyFilter() {
	switch m.activeTab {
	case boardTabActions:
		m.filtered = nil
		for _, r := range m.rows {
			if r.isAction() {
				m.filtered = append(m.filtered, r)
			}
		}
	case boardTabActiveDue:
		m.filtered = nil
		for _, r := range m.rows {
			if r.isTask() {
				m.filtered = append(m.filtered, r)
			}
		}
	case boardTabBySource:
		m.filtered = make([]boardRow, len(m.rows))
		copy(m.filtered, m.rows)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Source < m.filtered[j].Source
		})
	case boardTabByPriority:
		m.filtered = make([]boardRow, len(m.rows))
		copy(m.filtered, m.rows)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Priority < m.filtered[j].Priority
		})
	case boardTabByScore:
		m.filtered = nil
		for _, r := range m.rows {
			profile := m.config.GetProjectScoringProfile(r.Project.ID)
			if profile != nil {
				m.filtered = append(m.filtered, r)
			}
		}
	case boardTabClosed:
		m.filtered = make([]boardRow, len(m.closedRows))
		copy(m.filtered, m.closedRows)
	default:
		m.filtered = make([]boardRow, len(m.rows))
		copy(m.filtered, m.rows)
	}

	// Apply common filters and search
	if hasActiveCommonFilters(&m.filter) || m.searchQuery != "" {
		var passed []boardRow
		for _, r := range m.filtered {
			if hasActiveCommonFilters(&m.filter) && !passesCommonFilters(r.Task, &m.filter, r.Project.ID, m.config) {
				continue
			}
			if m.searchQuery != "" && !m.matchesSearch(r) {
				continue
			}
			passed = append(passed, r)
		}
		m.filtered = passed
	}

	// Apply sorting: user-selected field, or default score-descending on By Score tab
	if m.sortField != SortByDefault {
		m.applySorting()
	} else if m.activeTab == boardTabByScore {
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.scorePercent(m.filtered[i]) > m.scorePercent(m.filtered[j])
		})
	}
}

// applySorting sorts the filtered rows based on the current sort field.
func (m *FocusBoardModel) applySorting() {
	sort.SliceStable(m.filtered, func(i, j int) bool {
		ri, rj := m.filtered[i], m.filtered[j]
		if m.sortField == SortByScore {
			si, sj := m.scorePercent(ri), m.scorePercent(rj)
			if si != sj {
				cmp := -1
				if si < sj {
					cmp = 1
				}
				return ApplySortDirection(cmp, m.sortAscending)
			}
			return false
		}
		cmp := CompareTasksCrossProject(
			ri.Task, rj.Task, m.sortField,
			ri.Project.ID, rj.Project.ID, m.config,
		)
		return ApplySortDirection(cmp, m.sortAscending)
	})
}

// scorePercent returns the normalized score percentage for a task row.
func (m *FocusBoardModel) scorePercent(r boardRow) float64 {
	profile := m.config.GetProjectScoringProfile(r.Project.ID)
	if profile == nil {
		return 0
	}
	score := scoring.ComputeScore(r.Task, profile)
	return scoring.ScorePercent(score, profile)
}

// cycleSortField toggles sort direction if same field, or switches field if different.
func (m *FocusBoardModel) cycleSortField(newField SortField) {
	if m.sortField == newField {
		m.sortAscending = !m.sortAscending
	} else {
		m.sortField = newField
		m.sortAscending = true
	}
	m.applyFilter()
	m.clampCursor()
	direction := "↑"
	if !m.sortAscending {
		direction = "↓"
	}
	m.statusMessage = fmt.Sprintf("Sort: %s %s", newField, direction)
}

func (m *FocusBoardModel) Init() tea.Cmd {
	return nil
}

func (m *FocusBoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		inputWidth := max(msg.Width-8, 30)
		m.addInput.Width = inputWidth
		m.editInput.Width = inputWidth
		m.searchInput.Width = max(msg.Width-20, 30)
		return m, nil

	case FocusBoardRefreshMsg:
		m.loadItems()
		m.clampCursor()
		return m, nil

	case tea.KeyMsg:
		if m.addMode {
			return m.handleAddInput(msg)
		}
		if m.editMode {
			return m.handleEditInput(msg)
		}
		if m.searchMode {
			return m.handleSearchInput(msg)
		}
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

// handleKeyMsg handles keyboard input in normal mode.
func (m *FocusBoardModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Handle confirmation dialog before other key processing
	if m.confirm.IsActive() {
		return m.handleConfirmKey(key)
	}

	// Navigation
	if handleListNavigation(key, &m.cursor, len(m.filtered)) {
		return m, nil
	}

	switch key {
	case "esc", "q":
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.statusMessage = "Filters cleared"
			return m, nil
		}
		return m, func() tea.Msg { return CloseFocusBoardMsg{} }

	// Filter prefix
	case "F":
		m.pendingPrefix = "F"
		return m, nil

	// Search
	case "/":
		m.searchMode = true
		m.searchInput.SetValue(m.searchQuery)
		m.searchInput.Focus()
		return m, textinput.Blink

	// Tab navigation
	case "tab", "]", "l":
		CycleTab(&m.activeTab, len(focusBoardTabNames), 1)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	case "shift+tab", "[", "h":
		CycleTab(&m.activeTab, len(focusBoardTabNames), -1)
		m.applyFilter()
		m.clampCursor()
		return m, nil

	// Quick-add action
	case "a":
		m.addMode = true
		m.addInput.SetValue("")
		m.addInput.Focus()
		return m, textinput.Blink

	// Quick-edit title
	case "E":
		if row := m.getRowAtCursor(); row != nil && row.isAction() {
			m.editMode = true
			m.editIdx = m.cursor
			m.editInput.SetValue(row.Task.Title)
			m.editInput.Focus()
			return m, textinput.Blink
		}

	// Mark done
	case "x":
		return m.markDone()

	// Dismiss (cancel)
	case "X":
		return m.dismiss()

	// Toggle done/open
	case "d":
		return m.toggleDone()

	// Set priority
	case "p":
		if row := m.getRowAtCursor(); row != nil {
			return m, func() tea.Msg {
				return ShowPriorityPickerMsg{Task: row.Task, Project: row.Project}
			}
		}

	// Open URL / navigate to task
	case "enter":
		return m.handleEnter()

	// Open URL directly
	case "o":
		if row := m.getRowAtCursor(); row != nil && row.Task.URL != "" {
			return m, openBrowserCmd(row.Task.URL)
		}

	// Edit task
	case "e":
		if row := m.getRowAtCursor(); row != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: row.Task, Project: row.Project}
			}
		}

	// Delete permanently
	case "D":
		if row := m.getRowAtCursor(); row != nil {
			if row.isAction() {
				m.confirm.Show("delete", fmt.Sprintf("Delete action #%d? (y/n)", row.Task.ShortID))
				m.statusMessage = m.confirm.Prompt()
				return m, nil
			}
			m.statusMessage = "Use project tree to delete non-action tasks"
		}
		return m, nil

	// Sort keys (1-5 for sort field, 0 to reset)
	case "1", "2", "3", "4", "5", "0":
		if field, isReset, ok := SortFieldForKey(key); ok {
			if isReset {
				m.sortField = SortByDefault
				m.sortAscending = true
				m.applyFilter()
				m.clampCursor()
				m.statusMessage = "Sort reset"
			} else {
				m.cycleSortField(field)
			}
		}
		return m, nil

	// Assign to project (move task to another project)
	case "P":
		if row := m.getRowAtCursor(); row != nil && row.Project != nil {
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: row.Task, SourceProject: row.Project}
			}
		}
		return m, nil

	// Score editor
	case "S":
		if row := m.getRowAtCursor(); row != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: row.Task, Project: row.Project}
			}
		}
		return m, nil

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewFocusBoard,
				ViewName:   "Focus Board",
				Items:      m.GetHelpItems(),
			}
		}
	}

	return m, nil
}

// handleAddInput handles keyboard input in add mode.
func (m *FocusBoardModel) handleAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Let autocomplete handle keys first when active
	if m.handleBoardAutocompleteKey(key) {
		return m, nil
	}

	switch key {
	case "esc":
		m.addMode = false
		m.addInput.Blur()
		m.resetBoardAutocomplete()
		return m, nil
	case "enter":
		if m.acMode != acModeNone {
			return m, nil
		}
		value := strings.TrimSpace(m.addInput.Value())
		if value == "" {
			m.addMode = false
			m.addInput.Blur()
			m.resetBoardAutocomplete()
			return m, nil
		}
		return m.createItemFromInput(value)
	default:
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		m.refreshBoardAutocomplete()
		return m, cmd
	}
}

// handleEditInput handles keyboard input in edit mode.
func (m *FocusBoardModel) handleEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editMode = false
		m.editInput.Blur()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.editInput.Value())
		m.editMode = false
		m.editInput.Blur()
		if value == "" {
			return m, nil
		}
		if m.editIdx >= 0 && m.editIdx < len(m.filtered) {
			row := m.filtered[m.editIdx]
			row.Project.MutateTask(row.Task.ID, func(t *models.Task) {
				t.Title = value
			})
			if err := saveProjectSync(row.Project); err != nil {
				m.statusMessage = "Failed to save: " + err.Error()
			} else {
				m.statusMessage = "Title updated"
				m.loadItems()
				m.clampCursor()
			}
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
}

// handleSearchInput processes keyboard input while in search mode.
func (m *FocusBoardModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searchInput.Blur()
		m.searchQuery = ""
		m.searchInput.SetValue("")
		m.applyFilter()
		m.clampCursor()
		return m, nil

	case "enter":
		m.searchMode = false
		m.searchInput.Blur()
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m.applyFilter()
		m.clampCursor()
		return m, nil

	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m.applyFilter()
		m.clampCursor()
		return m, cmd
	}
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *FocusBoardModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	if key == "esc" {
		return m, nil
	}

	if isFilterSubPrefix(prefix) {
		return m.handleFocusBoardFilterSubPrefix(prefix, key)
	}

	if prefix == "F" {
		return m.handleFocusBoardFilterPrefix(key)
	}

	return m, nil
}

// handleConfirmKey handles key input while the confirmation dialog is active.
func (m *FocusBoardModel) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		m.confirm.Cancel()
		m.statusMessage = ""
		return m.deleteItem()
	case "n", "N", "esc":
		m.confirm.Cancel()
		m.statusMessage = ""
		return m, nil
	}
	return m, nil
}

// createItemFromInput parses the add input and creates an Action task.
func (m *FocusBoardModel) createItemFromInput(input string) (tea.Model, tea.Cmd) {
	m.addMode = false
	m.addInput.Blur()
	m.resetBoardAutocomplete()

	result := parseBoardAddInput(input)

	// Determine target project
	var targetProject *models.Project

	if result.projectSlug != "" {
		for _, p := range m.acProjectCache {
			if p.Slug == result.projectSlug {
				targetProject = p
				break
			}
		}
	}

	if targetProject == nil {
		// Use action inbox as default
		project, created := m.config.EnsureActionInboxProject()
		targetProject = project
		if created {
			// Ensure project directory exists and save config
			if err := storage.CreateProjectDir(targetProject); err != nil {
				m.statusMessage = "Failed to create action inbox: " + err.Error()
				return m, nil
			}
			if err := storage.SaveProject(targetProject); err != nil {
				m.statusMessage = "Failed to save action inbox: " + err.Error()
				return m, nil
			}
			if m.configManager != nil {
				_ = m.configManager.Save(m.config)
			}
		}
	}

	if err := m.projectService.Load(targetProject); err != nil {
		m.statusMessage = "Failed to load project: " + err.Error()
		return m, nil
	}

	task := models.NewTask(uuid.New().String(), targetProject.GetNextShortID(), models.TaskTypeAction, result.title)
	if result.source != "" {
		task.Source = result.source
	}
	if result.url != "" {
		task.URL = result.url
	}
	if result.priority >= 1 && result.priority <= 5 {
		task.Priority = result.priority
	}
	if len(result.tags) > 0 {
		task.Tags = result.tags
	}

	targetProject.Tasks = append(targetProject.Tasks, task)
	targetProject.BuildTaskTree()

	if err := saveProjectSync(targetProject); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
		return m, nil
	}

	m.statusMessage = fmt.Sprintf("Added #%d: %s", task.ShortID, task.Title)
	m.loadItems()
	m.clampCursor()
	return m, nil
}

// markDone marks the current task as done.
func (m *FocusBoardModel) markDone() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}
	task := row.Task
	project := row.Project

	if msg := task.CanTransitionTo(models.StatusDone); msg != "" {
		m.statusMessage = msg
		return m, nil
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusDone)
	})
	if err := saveProjectSync(project); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Marked #%d done", task.ShortID)
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// dismiss cancels the current task.
func (m *FocusBoardModel) dismiss() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}
	task := row.Task
	project := row.Project

	if msg := task.CanTransitionTo(models.StatusCanceled); msg != "" {
		m.statusMessage = msg
		return m, nil
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusCanceled)
	})
	if err := saveProjectSync(project); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Dismissed #%d", task.ShortID)
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// toggleDone toggles between done and open.
func (m *FocusBoardModel) toggleDone() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}
	task := row.Task
	project := row.Project

	var targetStatus models.TaskStatus
	if task.Status.IsComplete() {
		targetStatus = models.StatusTodo
	} else {
		targetStatus = models.StatusDone
	}

	if msg := task.CanTransitionTo(targetStatus); msg != "" {
		m.statusMessage = msg
		return m, nil
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(targetStatus)
	})
	if err := saveProjectSync(project); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		if targetStatus == models.StatusDone {
			m.statusMessage = fmt.Sprintf("Marked #%d done", task.ShortID)
		} else {
			m.statusMessage = fmt.Sprintf("Reopened #%d", task.ShortID)
		}
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// handleEnter opens URL or navigates to task.
func (m *FocusBoardModel) handleEnter() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}

	// For all tasks, navigate to the task in its project
	if row.isTask() {
		return m, func() tea.Msg {
			return FocusBoardSelectMsg{Task: row.Task, Project: row.Project}
		}
	}

	// For actions: navigate to task or open URL
	if row.isAction() {
		if row.Task.URL != "" {
			return m, openBrowserCmd(row.Task.URL)
		}
		// No URL — open task detail
		return m, func() tea.Msg {
			return ShowTaskDetailMsg{Task: row.Task, Project: row.Project}
		}
	}

	return m, nil
}

// deleteItem deletes the action task at cursor.
func (m *FocusBoardModel) deleteItem() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil || !row.isAction() {
		return m, nil
	}

	task := row.Task
	project := row.Project

	// Clean up attachments
	_ = storage.CleanupTaskAttachments(project, task)

	// Remove task from project
	var remaining []*models.Task
	for _, t := range project.Tasks {
		if t.ID != task.ID {
			remaining = append(remaining, t)
		}
	}
	project.Tasks = remaining
	project.BuildTaskTree()

	if err := saveProjectSync(project); err != nil {
		m.statusMessage = "Failed to delete: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Deleted #%d", task.ShortID)
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// getRowAtCursor returns the row at the current cursor position.
func (m *FocusBoardModel) getRowAtCursor() *boardRow {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

// clampCursor ensures cursor is within valid range.
func (m *FocusBoardModel) clampCursor() {
	m.cursor = ClampCursor(m.cursor, len(m.filtered))
}

// ShouldCaptureEsc returns true when add mode, search mode, or confirm dialog is active.
func (m *FocusBoardModel) ShouldCaptureEsc() bool {
	return m.addMode || m.editMode || m.searchMode || m.confirm.IsActive() || m.pendingPrefix != "" || m.HasActiveFilters()
}

// HasTextInput returns true when a text input is focused.
func (m *FocusBoardModel) HasTextInput() bool {
	return m.addMode || m.editMode || m.searchMode
}

// GetHelpItems returns help items for this view.
func (m *FocusBoardModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "a", Description: "Quick-add action"},
		{Key: "x", Description: "Mark done"},
		{Key: "X", Description: "Dismiss (cancel)"},
		{Key: "d", Description: "Toggle done/open"},
		{Key: "D", Description: "Delete action permanently"},
		{Key: "p", Description: "Set priority"},
		{Key: "e", Description: "Edit task"},
		{Key: "E", Description: "Quick-edit title"},
		{Key: "P", Description: "Assign project"},
		{Key: "S", Description: "Edit score dimensions"},
		{Key: "o", Description: "Open URL"},
		{Key: "enter", Description: "Navigate to task / open URL"},
		{Key: "tab/[/]", Description: "Switch tabs"},
		{Key: "F", Description: "Filter (F-prefix)"},
		{Key: "/", Description: "Search"},
		{Key: "Fx", Description: "Clear filters"},
		{Key: "1-5", Description: "Sort by field (pri/due/new/sts/scr)"},
		{Key: "0", Description: "Reset sort"},
		{Key: "j/k", Description: "Navigate list"},
		{Key: "q/esc", Description: "Close (clears filters first)"},
	}
}

// openBrowserCmd opens a URL in the default browser.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Start()
		return nil
	}
}
