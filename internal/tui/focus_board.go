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
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var focusBoardTabNames = []string{"All", "External", "Tasks", "By Source", "By Priority", "Closed"}

const (
	boardTabAll        = 0
	boardTabExternal   = 1
	boardTabTasks      = 2
	boardTabBySource   = 3
	boardTabByPriority = 4
	boardTabClosed     = 5
)

// boardRow is a unified display item: either a BoardItem or an aggregated task.
type boardRow struct {
	BoardItem *models.BoardItem // non-nil for board items
	Task      *models.Task      // non-nil for aggregated tasks
	Project   *models.Project   // set for aggregated tasks
	Priority  int               // for sorting
	Source    string            // for grouping
}

func (r boardRow) isBoardItem() bool { return r.BoardItem != nil }
func (r boardRow) isTask() bool      { return r.Task != nil }

func (r boardRow) title() string {
	if r.BoardItem != nil {
		return r.BoardItem.Title
	}
	if r.Task != nil {
		return r.Task.Title
	}
	return ""
}

// FocusBoardModel displays board items + aggregated tasks in a unified view.
type FocusBoardModel struct {
	BaseView
	InteractiveViewState

	config         *config.Config
	projectService *ProjectService

	// Data
	boardItems []*models.BoardItem
	rows       []boardRow
	closedRows []boardRow
	filtered   []boardRow

	// Tabs
	activeTab int

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
func NewFocusBoardModel(cfg *config.Config) *FocusBoardModel {
	ti := textinput.New()
	ti.Placeholder = "Item title (--source X --url Y --priority N)"
	ti.CharLimit = 200
	ti.Width = 60

	ei := textinput.New()
	ei.Placeholder = "New title"
	ei.CharLimit = 200
	ei.Width = 60

	m := &FocusBoardModel{
		config:         cfg,
		projectService: NewProjectService(),
		addInput:       ti,
		editInput:      ei,
	}
	m.loadItems()
	return m
}

// loadItems loads board items from disk and aggregates tasks.
func (m *FocusBoardModel) loadItems() {
	m.rows = nil
	m.boardItems = nil
	m.closedRows = nil

	// Load board items
	bd, err := storage.LoadBoard(m.config.DataDir)
	if err == nil {
		for _, item := range bd.Items {
			row := boardRow{
				BoardItem: item,
				Priority:  item.Priority,
				Source:    item.Source,
			}
			if item.IsOpen() {
				m.boardItems = append(m.boardItems, item)
				m.rows = append(m.rows, row)
			} else if item.IsClosed() {
				m.closedRows = append(m.closedRows, row)
			}
		}
	}

	// Aggregate active/due tasks across projects
	now := time.Now()
	today := util.StartOfDay(now)
	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := m.projectService.Load(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
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

	// Sort by priority ascending, then by creation/update time
	sort.SliceStable(m.rows, func(i, j int) bool {
		if m.rows[i].Priority != m.rows[j].Priority {
			return m.rows[i].Priority < m.rows[j].Priority
		}
		return false // preserve original order within same priority
	})

	m.applyFilter()
	m.initBoardCaches()
}

// applyFilter filters rows based on the active tab.
func (m *FocusBoardModel) applyFilter() {
	switch m.activeTab {
	case boardTabExternal:
		m.filtered = nil
		for _, r := range m.rows {
			if r.isBoardItem() {
				m.filtered = append(m.filtered, r)
			}
		}
	case boardTabTasks:
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
	case boardTabClosed:
		m.filtered = make([]boardRow, len(m.closedRows))
		copy(m.filtered, m.closedRows)
	default:
		m.filtered = make([]boardRow, len(m.rows))
		copy(m.filtered, m.rows)
	}
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
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

// handleKeyMsg handles keyboard input in normal mode.
func (m *FocusBoardModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Navigation
	if handleListNavigation(key, &m.cursor, len(m.filtered)) {
		return m, nil
	}

	switch key {
	case "esc", "q":
		return m, func() tea.Msg { return CloseFocusBoardMsg{} }

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

	// Quick-add
	case "a":
		m.addMode = true
		m.addInput.SetValue("")
		m.addInput.Focus()
		return m, textinput.Blink

	// Quick-edit title
	case "E":
		if row := m.getRowAtCursor(); row != nil && row.isBoardItem() {
			m.editMode = true
			m.editIdx = m.cursor
			m.editInput.SetValue(row.BoardItem.Title)
			m.editInput.Focus()
			return m, textinput.Blink
		}

	// Mark done
	case "x":
		if row := m.getRowAtCursor(); row != nil && row.isTask() {
			m.statusMessage = "Use project tree to manage tasks"
			return m, nil
		}
		return m.markDone()

	// Dismiss
	case "X":
		if row := m.getRowAtCursor(); row != nil && row.isTask() {
			m.statusMessage = "Use project tree to manage tasks"
			return m, nil
		}
		return m.dismiss()

	// Toggle done/open
	case "d":
		if row := m.getRowAtCursor(); row != nil && row.isTask() {
			m.statusMessage = "Use project tree to manage tasks"
			return m, nil
		}
		return m.toggleDone()

	// Cycle priority
	case "p":
		if row := m.getRowAtCursor(); row != nil && row.isTask() {
			m.statusMessage = "Use project tree to manage tasks"
			return m, nil
		}
		return m.cyclePriority()

	// Open URL / navigate to task
	case "enter":
		return m.handleEnter()

	// Open URL directly
	case "o":
		if row := m.getRowAtCursor(); row != nil && row.isBoardItem() && row.BoardItem.URL != "" {
			return m, openBrowserCmd(row.BoardItem.URL)
		}

	// Edit in editor
	case "e":
		if row := m.getRowAtCursor(); row != nil && row.isBoardItem() {
			return m, m.editInEditor(row.BoardItem)
		}

	// Delete permanently
	case "D":
		if row := m.getRowAtCursor(); row != nil {
			if row.isTask() {
				m.statusMessage = "Use project tree to manage tasks"
				return m, nil
			}
			if row.isBoardItem() {
				m.confirm.Show("delete", fmt.Sprintf("Delete board item #%d? (y/n)", row.BoardItem.ShortID))
				m.statusMessage = m.confirm.Prompt()
				return m, nil
			}
		}

	// Handle confirm dialog
	case "y":
		if m.confirm.IsActive() {
			confirmed, handled := m.confirm.HandleKey(key)
			if handled && confirmed {
				return m.deleteItem()
			}
			m.statusMessage = ""
			return m, nil
		}
	case "n":
		if m.confirm.IsActive() {
			_, handled := m.confirm.HandleKey(key)
			if handled {
				m.statusMessage = ""
				return m, nil
			}
		}

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
			if row.isBoardItem() {
				row.BoardItem.Title = value
				row.BoardItem.UpdatedAt = time.Now()
				if err := storage.SaveBoardItem(m.config.DataDir, row.BoardItem); err != nil {
					m.statusMessage = "Failed to save: " + err.Error()
				} else {
					m.statusMessage = "Title updated"
					m.loadItems()
					m.clampCursor()
				}
			}
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
}

// createItemFromInput parses the add input and creates a board item.
func (m *FocusBoardModel) createItemFromInput(input string) (tea.Model, tea.Cmd) {
	m.addMode = false
	m.addInput.Blur()
	m.resetBoardAutocomplete()

	result := parseBoardAddInput(input)

	bd, err := storage.LoadBoard(m.config.DataDir)
	if err != nil {
		m.statusMessage = "Failed to load board: " + err.Error()
		return m, nil
	}

	item := models.NewBoardItem(uuid.New().String()[:8], bd.NextShortID, result.title)
	if result.source != "" {
		item.Source = result.source
	}
	if result.url != "" {
		item.URL = result.url
	}
	if result.priority >= 1 && result.priority <= 5 {
		item.Priority = result.priority
	}
	if len(result.tags) > 0 {
		item.Tags = result.tags
	}

	// Resolve project slug to ID/Name
	if result.projectSlug != "" {
		for _, p := range m.acProjectCache {
			if p.Slug == result.projectSlug {
				item.ProjectID = p.ID
				item.ProjectName = p.Name
				break
			}
		}
	}

	item.Filename = models.GenerateBoardItemFilename(item.ShortID, item.Title)

	if err := storage.SaveBoardItem(m.config.DataDir, item); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
		return m, nil
	}
	if err := storage.SaveBoardIndex(m.config.DataDir, bd.NextShortID+1); err != nil {
		m.statusMessage = "Failed to save index: " + err.Error()
		return m, nil
	}

	m.statusMessage = fmt.Sprintf("Added #%d: %s", item.ShortID, item.Title)
	m.loadItems()
	m.clampCursor()
	return m, nil
}

// markDone marks the current board item as done.
func (m *FocusBoardModel) markDone() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}
	if row.isBoardItem() {
		row.BoardItem.MarkDone()
		if err := storage.SaveBoardItem(m.config.DataDir, row.BoardItem); err != nil {
			m.statusMessage = "Failed to save: " + err.Error()
		} else {
			m.statusMessage = fmt.Sprintf("Marked #%d done", row.BoardItem.ShortID)
			m.loadItems()
			m.clampCursor()
		}
	}
	return m, nil
}

// dismiss marks the current board item as dismissed.
func (m *FocusBoardModel) dismiss() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil || !row.isBoardItem() {
		return m, nil
	}
	row.BoardItem.MarkDismissed()
	if err := storage.SaveBoardItem(m.config.DataDir, row.BoardItem); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Dismissed #%d", row.BoardItem.ShortID)
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// toggleDone toggles between done and open.
func (m *FocusBoardModel) toggleDone() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil || !row.isBoardItem() {
		return m, nil
	}
	if row.BoardItem.IsOpen() {
		row.BoardItem.MarkDone()
		m.statusMessage = fmt.Sprintf("Marked #%d done", row.BoardItem.ShortID)
	} else {
		row.BoardItem.Reopen()
		m.statusMessage = fmt.Sprintf("Reopened #%d", row.BoardItem.ShortID)
	}
	if err := storage.SaveBoardItem(m.config.DataDir, row.BoardItem); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// cyclePriority cycles the priority of the current board item.
func (m *FocusBoardModel) cyclePriority() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil || !row.isBoardItem() {
		return m, nil
	}
	row.BoardItem.Priority = row.BoardItem.Priority%5 + 1
	row.BoardItem.UpdatedAt = time.Now()
	if err := storage.SaveBoardItem(m.config.DataDir, row.BoardItem); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Priority → P%d", row.BoardItem.Priority)
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

	if row.isTask() {
		return m, func() tea.Msg {
			return FocusBoardSelectMsg{Task: row.Task, Project: row.Project}
		}
	}

	if row.isBoardItem() {
		// If linked to a task, navigate to that task
		if row.BoardItem.LinkedTaskID != "" {
			task, project := m.findLinkedTask(row.BoardItem.LinkedTaskID)
			if task != nil && project != nil {
				return m, func() tea.Msg {
					return FocusBoardSelectMsg{Task: task, Project: project}
				}
			}
		}
		// Otherwise open URL if present
		if row.BoardItem.URL != "" {
			return m, openBrowserCmd(row.BoardItem.URL)
		}
	}

	return m, nil
}

// deleteItem deletes the board item at cursor.
func (m *FocusBoardModel) deleteItem() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil || !row.isBoardItem() {
		return m, nil
	}
	if err := storage.DeleteBoardItem(m.config.DataDir, row.BoardItem); err != nil {
		m.statusMessage = "Failed to delete: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Deleted #%d", row.BoardItem.ShortID)
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// editInEditor opens the board item in an external editor.
func (m *FocusBoardModel) editInEditor(item *models.BoardItem) tea.Cmd {
	dir := storage.BoardDir(m.config.DataDir)
	path := dir + "/" + item.Filename

	editor := m.config.Editor
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return FocusBoardRefreshMsg{}
	})
}

// findLinkedTask looks up a task by UUID across loaded projects.
func (m *FocusBoardModel) findLinkedTask(taskID string) (*models.Task, *models.Project) {
	for _, project := range m.config.GetActiveProjects() {
		if err := m.projectService.Load(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			if task.ID == taskID {
				return task, project
			}
		}
	}
	return nil, nil
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

// ShouldCaptureEsc returns true when add mode or confirm dialog is active.
func (m *FocusBoardModel) ShouldCaptureEsc() bool {
	return m.addMode || m.editMode || m.confirm.IsActive()
}

// HasTextInput returns true when the add input is focused.
func (m *FocusBoardModel) HasTextInput() bool {
	return m.addMode || m.editMode
}

// GetHelpItems returns help items for this view.
func (m *FocusBoardModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "a", Description: "Quick-add board item"},
		{Key: "x", Description: "Mark item done"},
		{Key: "X", Description: "Dismiss item"},
		{Key: "d", Description: "Toggle done/open (reopen on Closed tab)"},
		{Key: "D", Description: "Delete permanently"},
		{Key: "p", Description: "Cycle priority"},
		{Key: "e", Description: "Edit in editor"},
		{Key: "E", Description: "Quick-edit title"},
		{Key: "o", Description: "Open URL"},
		{Key: "enter", Description: "Navigate to task / open URL"},
		{Key: "tab/[/]", Description: "Switch tabs"},
		{Key: "j/k", Description: "Navigate list"},
		{Key: "q/esc", Description: "Close"},
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
