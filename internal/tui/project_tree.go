package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// SortField represents the field to sort tasks by.
type SortField int

const (
	SortByDefault   SortField = iota // existing order (by type hierarchy)
	SortByPriority                   // high priority first (1 = highest)
	SortByDueDate                    // earliest due date first
	SortByCreatedAt                  // by creation date
	SortByStatus                     // by workflow status order
)

// DueDateFilter represents the due date filter options.
type DueDateFilter int

const (
	DueDateAll      DueDateFilter = iota // show all tasks
	DueDateOverdue                       // only overdue tasks
	DueDateToday                         // tasks due today
	DueDateThisWeek                      // tasks due within 7 days
	DueDateNone                          // tasks with no due date
)

// String returns the display name for a sort field.
func (s SortField) String() string {
	switch s {
	case SortByPriority:
		return "priority"
	case SortByDueDate:
		return "due date"
	case SortByCreatedAt:
		return "created"
	case SortByStatus:
		return "status"
	default:
		return "default"
	}
}

// TreeNode represents a node in the task tree.
type TreeNode struct {
	Task     *models.Task
	Level    int
	Expanded bool
	Visible  bool
}

// ProjectTreeModel represents the project tree view.
type ProjectTreeModel struct {
	project       *models.Project
	config        *config.Config
	configManager *config.Manager
	nodes         []*TreeNode
	cursor        int
	width         int
	height        int

	// Filter options
	showCompleted  bool
	filterType     models.TaskType
	filterStatus   models.TaskStatus // filter by specific status
	filterPriority int               // 0=all, 1=high only (<=2)
	filterTag      string            // filter by tag name
	filterDueDate  DueDateFilter     // filter by due date

	// Sort options
	sortField     SortField // current sort field
	sortAscending bool      // sort direction

	// Search state
	searchMode  bool
	searchInput textinput.Model
	searchQuery string

	// Confirmation state for destructive actions
	confirmAction string       // "", "archive", "unarchive", "delete"
	deleteTarget  *models.Task // Task to delete when confirmAction="delete"
}

// NewProjectTreeModel creates a new project tree model.
func NewProjectTreeModel(project *models.Project, cfg *config.Config, manager *config.Manager) *ProjectTreeModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search tasks..."
	searchInput.CharLimit = 100
	searchInput.Width = 40

	m := &ProjectTreeModel{
		project:        project,
		config:         cfg,
		configManager:  manager,
		cursor:         0,
		showCompleted:  false,
		filterType:     "",
		filterStatus:   "",
		filterPriority: 0,
		filterTag:      "",
		sortField:      SortByDefault,
		sortAscending:  true,
		searchMode:     false,
		searchInput:    searchInput,
		searchQuery:    "",
	}
	m.buildTree()
	return m
}

func (m *ProjectTreeModel) buildTree() {
	m.nodes = []*TreeNode{}

	// Get root tasks
	rootTasks := m.project.GetRootTasks()

	// Build nodes recursively
	for _, task := range rootTasks {
		m.addTaskNode(task, 0, true)
	}

	m.sortNodes()
	m.updateVisibility()
}

func (m *ProjectTreeModel) addTaskNode(task *models.Task, level int, expanded bool) {
	node := &TreeNode{
		Task:     task,
		Level:    level,
		Expanded: expanded,
		Visible:  true,
	}
	m.nodes = append(m.nodes, node)

	// Add children
	for _, child := range task.Children {
		m.addTaskNode(child, level+1, false)
	}
}

func (m *ProjectTreeModel) updateVisibility() {
	parentExpanded := make(map[int]bool)
	parentExpanded[-1] = true

	for i, node := range m.nodes {
		// Check if parent is expanded
		parentLevel := node.Level - 1
		visible := true

		if parentLevel >= 0 {
			// Find parent node
			for j := i - 1; j >= 0; j-- {
				if m.nodes[j].Level == parentLevel {
					visible = m.nodes[j].Expanded && m.nodes[j].Visible
					break
				}
			}
		}

		// Apply filters
		if visible && !m.showCompleted && node.Task.IsComplete() {
			visible = false
		}

		if visible && m.filterType != "" && node.Task.Type != m.filterType {
			visible = false
		}

		// Apply status filter
		if visible && m.filterStatus != "" && node.Task.Status != m.filterStatus {
			visible = false
		}

		// Apply priority filter (1 = high priority only, showing tasks with priority <= 2)
		if visible && m.filterPriority == 1 && node.Task.Priority > 2 {
			visible = false
		}

		// Apply tag filter
		if visible && m.filterTag != "" && !node.Task.HasTag(m.filterTag) {
			visible = false
		}

		// Apply due date filter
		if visible && m.filterDueDate != DueDateAll {
			switch m.filterDueDate {
			case DueDateOverdue:
				visible = isOverdue(node.Task)
			case DueDateToday:
				visible = isDueToday(node.Task)
			case DueDateThisWeek:
				visible = isDueThisWeek(node.Task)
			case DueDateNone:
				visible = node.Task.DueDate == nil
			}
		}

		// Apply search filter
		if visible && m.searchQuery != "" {
			if !m.matchesSearch(node.Task) && !m.hasMatchingDescendant(node.Task) {
				visible = false
			}
		}

		node.Visible = visible
	}
}

func (m *ProjectTreeModel) matchesSearch(task *models.Task) bool {
	if m.searchQuery == "" {
		return true
	}
	query := strings.ToLower(m.searchQuery)

	if strings.Contains(strings.ToLower(task.Title), query) {
		return true
	}
	for _, tag := range task.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(task.Description), query) {
		return true
	}
	return false
}

func (m *ProjectTreeModel) hasMatchingDescendant(task *models.Task) bool {
	for _, child := range task.Children {
		if m.matchesSearch(child) || m.hasMatchingDescendant(child) {
			return true
		}
	}
	return false
}

// isOverdue returns true if the task is past its due date and not completed.
func isOverdue(task *models.Task) bool {
	if task.DueDate == nil || task.Status.IsComplete() {
		return false
	}
	today := time.Now().Truncate(24 * time.Hour)
	return task.DueDate.Before(today)
}

// isDueToday returns true if the task is due today.
func isDueToday(task *models.Task) bool {
	if task.DueDate == nil {
		return false
	}
	today := time.Now().Truncate(24 * time.Hour)
	dueDay := task.DueDate.Truncate(24 * time.Hour)
	return dueDay.Equal(today)
}

// isDueThisWeek returns true if the task is due within the next 7 days.
func isDueThisWeek(task *models.Task) bool {
	if task.DueDate == nil {
		return false
	}
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	weekEnd := today.AddDate(0, 0, 7)
	dueDay := task.DueDate.Truncate(24 * time.Hour)
	return !dueDay.Before(today) && dueDay.Before(weekEnd)
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *ProjectTreeModel) HasActiveSearch() bool {
	return m.searchMode || m.searchQuery != ""
}

func (m *ProjectTreeModel) visibleNodes() []*TreeNode {
	var visible []*TreeNode
	for _, node := range m.nodes {
		if node.Visible {
			visible = append(visible, node)
		}
	}
	return visible
}

// sortNodes sorts the nodes based on the current sort field.
// Sorting is applied within each hierarchy level to preserve parent-child relationships.
func (m *ProjectTreeModel) sortNodes() {
	if m.sortField == SortByDefault {
		return
	}

	// Group nodes by their parent
	nodesByParent := make(map[string][]*TreeNode)
	var rootNodes []*TreeNode

	for _, node := range m.nodes {
		if node.Task.ParentID == nil {
			rootNodes = append(rootNodes, node)
		} else {
			parentID := *node.Task.ParentID
			nodesByParent[parentID] = append(nodesByParent[parentID], node)
		}
	}

	// Sort root nodes
	m.sortNodeSlice(rootNodes)

	// Rebuild the nodes slice in sorted order
	m.nodes = []*TreeNode{}
	var addNodeWithChildren func(node *TreeNode)
	addNodeWithChildren = func(node *TreeNode) {
		m.nodes = append(m.nodes, node)
		children := nodesByParent[node.Task.ID]
		m.sortNodeSlice(children)
		for _, child := range children {
			addNodeWithChildren(child)
		}
	}

	for _, root := range rootNodes {
		addNodeWithChildren(root)
	}
}

// sortNodeSlice sorts a slice of nodes based on current sort settings.
func (m *ProjectTreeModel) sortNodeSlice(nodes []*TreeNode) {
	if len(nodes) <= 1 {
		return
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		return m.compareNodes(nodes[i], nodes[j])
	})
}

// compareNodes compares two nodes based on the current sort field and direction.
func (m *ProjectTreeModel) compareNodes(a, b *TreeNode) bool {
	var less bool

	switch m.sortField {
	case SortByPriority:
		// Lower priority number = higher priority (1 is highest)
		less = a.Task.Priority < b.Task.Priority

	case SortByDueDate:
		// nil due dates go to the end
		if a.Task.DueDate == nil && b.Task.DueDate == nil {
			less = false
		} else if a.Task.DueDate == nil {
			less = false // a goes after b
		} else if b.Task.DueDate == nil {
			less = true // a goes before b
		} else {
			less = a.Task.DueDate.Before(*b.Task.DueDate)
		}

	case SortByCreatedAt:
		less = a.Task.CreatedAt.Before(b.Task.CreatedAt)

	case SortByStatus:
		less = m.statusOrder(a.Task.Status) < m.statusOrder(b.Task.Status)

	default:
		less = false
	}

	if !m.sortAscending {
		less = !less
	}

	return less
}

// statusOrder returns the sort order for a status (workflow order).
func (m *ProjectTreeModel) statusOrder(status models.TaskStatus) int {
	switch status {
	case models.StatusTodo:
		return 0
	case models.StatusInProgress:
		return 1
	case models.StatusBlocked:
		return 2
	case models.StatusDone:
		return 3
	case models.StatusCancelled:
		return 4
	default:
		return 5
	}
}

// getAllTags returns all unique tags from the project's tasks.
func (m *ProjectTreeModel) getAllTags() []string {
	tagSet := make(map[string]bool)
	for _, task := range m.project.Tasks {
		for _, tag := range task.Tags {
			tagSet[tag] = true
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// clampCursor ensures the cursor is within valid bounds.
func (m *ProjectTreeModel) clampCursor() {
	visible := m.visibleNodes()
	if m.cursor >= len(visible) {
		m.cursor = len(visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// SetSize sets the view dimensions.
func (m *ProjectTreeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ProjectTreeModel) Init() tea.Cmd {
	return nil
}

func (m *ProjectTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleNodes()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle confirmation prompts first
		if m.confirmAction != "" {
			switch msg.String() {
			case "y", "Y":
				if m.confirmAction == "archive" {
					m.confirmAction = ""
					return m, m.archiveProject()
				} else if m.confirmAction == "unarchive" {
					m.confirmAction = ""
					return m, m.unarchiveProject()
				} else if m.confirmAction == "delete" {
					m.confirmAction = ""
					return m, m.deleteTask()
				}
			case "n", "N", "esc":
				m.confirmAction = ""
				m.deleteTarget = nil
				return m, nil
			}
			return m, nil
		}

		// Handle search mode
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.searchQuery = ""
				m.updateVisibility()
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				m.searchQuery = m.searchInput.Value()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.searchQuery = m.searchInput.Value()
				m.updateVisibility()
				// Reset cursor if out of bounds
				visible = m.visibleNodes()
				if m.cursor >= len(visible) {
					if len(visible) > 0 {
						m.cursor = len(visible) - 1
					} else {
						m.cursor = 0
					}
				}
				return m, cmd
			}
		}

		switch msg.String() {
		case "/":
			if m.confirmAction == "" {
				m.searchMode = true
				m.searchInput.Focus()
				return m, textinput.Blink
			}
		case "esc":
			// Clear search filter if active
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.updateVisibility()
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
			if len(visible) > 0 && m.cursor < len(visible) {
				return m, func() tea.Msg {
					return SelectTaskMsg{Task: visible[m.cursor].Task}
				}
			}
		case " ", "tab":
			// Toggle expand/collapse
			if len(visible) > 0 && m.cursor < len(visible) {
				node := visible[m.cursor]
				if len(node.Task.Children) > 0 {
					node.Expanded = !node.Expanded
					m.updateVisibility()
				}
			}
		case "E":
			// Expand all
			for _, node := range m.nodes {
				node.Expanded = true
			}
			m.updateVisibility()
		case "c":
			// Collapse all
			for _, node := range m.nodes {
				if node.Level > 0 {
					node.Expanded = false
				}
			}
			m.updateVisibility()
		case "d":
			// Toggle show completed
			m.showCompleted = !m.showCompleted
			m.updateVisibility()
			m.clampCursor()

		// Filter controls
		case "P":
			// Cycle priority filter: all -> high -> all
			if m.filterPriority == 0 {
				m.filterPriority = 1 // high priority only
			} else {
				m.filterPriority = 0 // all
			}
			m.updateVisibility()
			m.clampCursor()

		case "t":
			// Cycle type filter
			types := models.AllTaskTypes()
			if m.filterType == "" {
				m.filterType = types[0]
			} else {
				found := false
				for i, t := range types {
					if t == m.filterType {
						if i+1 < len(types) {
							m.filterType = types[i+1]
						} else {
							m.filterType = "" // back to all
						}
						found = true
						break
					}
				}
				if !found {
					m.filterType = ""
				}
			}
			m.updateVisibility()
			m.clampCursor()

		case "S":
			// Cycle status filter
			statuses := models.AllTaskStatuses()
			if m.filterStatus == "" {
				m.filterStatus = statuses[0]
			} else {
				found := false
				for i, s := range statuses {
					if s == m.filterStatus {
						if i+1 < len(statuses) {
							m.filterStatus = statuses[i+1]
						} else {
							m.filterStatus = "" // back to all
						}
						found = true
						break
					}
				}
				if !found {
					m.filterStatus = ""
				}
			}
			m.updateVisibility()
			m.clampCursor()

		case "T":
			// Cycle tag filter
			tags := m.getAllTags()
			if len(tags) == 0 {
				m.filterTag = ""
			} else if m.filterTag == "" {
				m.filterTag = tags[0]
			} else {
				found := false
				for i, tag := range tags {
					if tag == m.filterTag {
						if i+1 < len(tags) {
							m.filterTag = tags[i+1]
						} else {
							m.filterTag = "" // back to all
						}
						found = true
						break
					}
				}
				if !found {
					m.filterTag = ""
				}
			}
			m.updateVisibility()
			m.clampCursor()

		case "u":
			// Cycle due date filter: all → overdue → today → this week → no due date → all
			m.filterDueDate = (m.filterDueDate + 1) % 5
			m.updateVisibility()
			m.clampCursor()

		// Sort controls
		case "1":
			// Sort by priority
			if m.sortField == SortByPriority {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortField = SortByPriority
				m.sortAscending = true
			}
			m.sortNodes()
			m.updateVisibility()

		case "2":
			// Sort by due date
			if m.sortField == SortByDueDate {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortField = SortByDueDate
				m.sortAscending = true
			}
			m.sortNodes()
			m.updateVisibility()

		case "3":
			// Sort by created date
			if m.sortField == SortByCreatedAt {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortField = SortByCreatedAt
				m.sortAscending = true
			}
			m.sortNodes()
			m.updateVisibility()

		case "4":
			// Sort by status
			if m.sortField == SortByStatus {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortField = SortByStatus
				m.sortAscending = true
			}
			m.sortNodes()
			m.updateVisibility()

		case "0":
			// Reset to default sort
			m.sortField = SortByDefault
			m.sortAscending = true
			m.buildTree()

		case "?":
			return m, func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewProjectTree,
					ViewName:   "Project Tree",
					Items:      m.GetHelpItems(),
				}
			}
		case "ctrl+f":
			return m, func() tea.Msg {
				return ShowFilterMsg{ProjectTree: m}
			}
		case "r":
			// Reload project
			storage.LoadProject(m.project)
			m.buildTree()
		case "R":
			// View task with glow pager
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return GlowViewMsg{Task: task}
				}
			}
		case "ctrl+o":
			// Edit description in external editor
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return EditDescriptionMsg{Task: task, Project: m.project}
				}
			}
		case "A":
			// Archive/unarchive project
			if m.project.Archived {
				m.confirmAction = "unarchive"
			} else {
				m.confirmAction = "archive"
			}

		// Task status changes
		case "s":
			// Start task (in_progress)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status == models.StatusTodo || task.Status == models.StatusBlocked {
					task.Start()
					return m, m.saveAndRefresh()
				}
			}
		case "m":
			// Mark complete (done)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status.IsActive() {
					// Block completion if task has incomplete dependencies
					if incompleteDeps := m.project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
						depNames := make([]string, len(incompleteDeps))
						for i, dep := range incompleteDeps {
							depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
						}
						return m, func() tea.Msg {
							return ErrorMsg{Err: fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
								len(incompleteDeps), strings.Join(depNames, ", "))}
						}
					}
					task.Complete()
					return m, m.saveAndRefresh()
				}
			}
		case "b":
			// Block task
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status.IsActive() && task.Status != models.StatusBlocked {
					task.Block()
					return m, m.saveAndRefresh()
				}
			}
		case "x":
			// Cancel task
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status.IsActive() {
					task.Cancel()
					return m, m.saveAndRefresh()
				}
			}
		case "o":
			// Reopen task (back to todo)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status != models.StatusTodo {
					task.Reopen()
					return m, m.saveAndRefresh()
				}
			}

		// Task type changes
		case "h":
			// Promote task type (Task -> Story -> Epic)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanPromote(); ok {
					task.Promote()
					return m, m.saveAndRefresh()
				}
			}
		case "l":
			// Demote task type (Epic -> Story -> Task)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanDemote(); ok {
					task.Demote()
					return m, m.saveAndRefresh()
				}
			}
		case "H":
			// Cycle leaf type left (Idea -> Bug -> Task)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanCycleLeafType(true); ok {
					task.CycleLeafType(true)
					return m, m.saveAndRefresh()
				}
			}
		case "L":
			// Cycle leaf type right (Task -> Bug -> Idea)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanCycleLeafType(false); ok {
					task.CycleLeafType(false)
					return m, m.saveAndRefresh()
				}
			}

		// Task movement operations
		case "K":
			// Move task up among siblings
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				siblings := m.project.GetSiblings(task)
				var siblingIdx int = -1
				for i, s := range siblings {
					if s.ID == task.ID {
						siblingIdx = i
						break
					}
				}
				if siblingIdx > 0 {
					m.project.MoveTaskUp(task)
					return m, m.saveAndRefresh()
				}
			}
		case "J":
			// Move task down among siblings
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				siblings := m.project.GetSiblings(task)
				var siblingIdx int = -1
				for i, s := range siblings {
					if s.ID == task.ID {
						siblingIdx = i
						break
					}
				}
				if siblingIdx >= 0 && siblingIdx < len(siblings)-1 {
					m.project.MoveTaskDown(task)
					return m, m.saveAndRefresh()
				}
			}
		case "right":
			// Indent task (nest under task above)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				targetParent := m.project.GetTaskAbove(task)
				if targetParent != nil {
					if ok, _ := task.CanIndent(targetParent); ok {
						m.project.IndentTask(task)
						return m, m.saveAndRefresh()
					}
				}
			}
		case "left":
			// Outdent task (move to parent's level)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanOutdent(); ok {
					m.project.OutdentTask(task)
					return m, m.saveAndRefresh()
				}
			}

		// Task operations
		case "n":
			// Add new task
			if len(visible) > 0 && m.cursor < len(visible) {
				// Use selected task as potential parent
				return m, func() tea.Msg {
					return CreateTaskMsg{ParentTask: visible[m.cursor].Task}
				}
			}
			// No task selected, create root task
			return m, func() tea.Msg {
				return CreateTaskMsg{ParentTask: nil}
			}
		case "e":
			// Edit selected task
			if len(visible) > 0 && m.cursor < len(visible) {
				return m, func() tea.Msg {
					return EditTaskMsg{Task: visible[m.cursor].Task}
				}
			}
		case "D":
			// Delete task (requires confirmation)
			if len(visible) > 0 && m.cursor < len(visible) {
				m.confirmAction = "delete"
				m.deleteTarget = visible[m.cursor].Task
			}
		case "ctrl+d":
			// Set due date for selected task
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: task, Project: m.project}
				}
			}
		case "B":
			// Manage dependencies for selected task
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return ManageDependenciesMsg{Task: task, Project: m.project}
				}
			}
		case "z":
			// Undo last operation
			return m, m.performUndo()
		}
	}

	return m, nil
}

func (m *ProjectTreeModel) archiveProject() tea.Cmd {
	return func() tea.Msg {
		m.project.Archive()

		// Clear current project if it was the archived one
		if m.config.CurrentProject == m.project.ID {
			m.config.CurrentProject = ""
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return ProjectArchivedMsg{Project: m.project}
	}
}

func (m *ProjectTreeModel) unarchiveProject() tea.Cmd {
	return func() tea.Msg {
		m.project.Unarchive()

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return ProjectArchivedMsg{Project: m.project}
	}
}

// performUndo restores the project from backup.
func (m *ProjectTreeModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		if !storage.BackupExists(m.project) {
			return ErrorMsg{Err: fmt.Errorf("no backup available to undo")}
		}

		tasksBefore := len(m.project.Tasks)

		if err := storage.RestoreFromBackup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to undo: %w", err)}
		}

		if err := storage.LoadProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after undo: %w", err)}
		}

		m.buildTree()
		return UndoCompletedMsg{
			TasksBefore: tasksBefore,
			TasksAfter:  len(m.project.Tasks),
		}
	}
}

// saveAndRefresh saves the project and refreshes the tree view.
func (m *ProjectTreeModel) saveAndRefresh() tea.Cmd {
	return func() tea.Msg {
		// Check for auto-unblocking before save
		m.project.CheckAndAutoUnblock()

		// Create backup before saving for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		storage.LoadProject(m.project)
		m.buildTree()
		return TaskStatusChangedMsg{Task: nil}
	}
}

// deleteTask removes a task and its children from the project.
func (m *ProjectTreeModel) deleteTask() tea.Cmd {
	return func() tea.Msg {
		if m.deleteTarget == nil {
			return nil
		}

		task := m.deleteTarget
		m.deleteTarget = nil

		// Create backup before deleting for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Collect IDs of task and all descendants to delete
		idsToDelete := make(map[string]bool)
		var collectIDs func(t *models.Task)
		collectIDs = func(t *models.Task) {
			idsToDelete[t.ID] = true
			for _, child := range t.Children {
				collectIDs(child)
			}
		}
		collectIDs(task)

		// Filter out deleted tasks
		var remaining []*models.Task
		for _, t := range m.project.Tasks {
			if !idsToDelete[t.ID] {
				// Clear parent reference if parent was deleted
				if t.ParentID != nil && idsToDelete[*t.ParentID] {
					t.ParentID = nil
				}
				remaining = append(remaining, t)
			}
		}
		m.project.Tasks = remaining

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		storage.LoadProject(m.project)
		m.buildTree()
		return TaskDeletedMsg{Task: task}
	}
}

func (m *ProjectTreeModel) View() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("%s - Tasks", m.project.Name)
	b.WriteString(TitleStyle.Render(title))

	// Show archived badge
	if m.project.Archived {
		b.WriteString(" ")
		b.WriteString(ArchivedBadgeStyle.Render("ARCHIVED"))
		if m.project.ArchivedAt != nil {
			b.WriteString(" ")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("since %s", m.project.ArchivedAt.Format("2006-01-02"))))
		}
	}
	b.WriteString("\n")

	// Search bar
	if m.searchMode {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
	} else if m.searchQuery != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Search: \"%s\" ", m.searchQuery)))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n")
	}

	// Confirmation prompt
	if m.confirmAction != "" {
		var prompt string
		if m.confirmAction == "delete" && m.deleteTarget != nil {
			childCount := m.deleteTarget.CountChildren()
			if childCount > 0 {
				prompt = fmt.Sprintf("Delete task #%d '%s' and %d children? (y/n)", m.deleteTarget.ShortID, m.deleteTarget.Title, childCount)
			} else {
				prompt = fmt.Sprintf("Delete task #%d '%s'? (y/n)", m.deleteTarget.ShortID, m.deleteTarget.Title)
			}
		} else {
			prompt = fmt.Sprintf("Are you sure you want to %s this project? (y/n)", m.confirmAction)
		}
		b.WriteString(ConfirmStyle.Render(prompt))
		b.WriteString("\n")
	}

	// Stats
	stats := fmt.Sprintf("%d tasks, %d active, %d done",
		m.project.CountTasks(),
		m.project.CountActiveTasks(),
		m.project.CountCompletedTasks())
	b.WriteString(MutedStyle.Render(stats))
	b.WriteString("\n\n")

	visible := m.visibleNodes()

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			b.WriteString(MutedStyle.Render("No tasks yet. Add one with: tt task add <title>"))
		} else {
			b.WriteString(MutedStyle.Render("No tasks match current filters."))
		}
		b.WriteString("\n")
	} else {
		// Calculate available height for task list
		availableHeight := m.height - 8

		// Determine visible range
		startIdx := 0
		endIdx := len(visible)

		if len(visible) > availableHeight {
			// Center cursor in view
			halfView := availableHeight / 2
			startIdx = m.cursor - halfView
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + availableHeight
			if endIdx > len(visible) {
				endIdx = len(visible)
				startIdx = endIdx - availableHeight
				if startIdx < 0 {
					startIdx = 0
				}
			}
		}

		// Render visible tasks
		for i := startIdx; i < endIdx; i++ {
			node := visible[i]
			line := m.renderNode(node, i == m.cursor)
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(visible) > availableHeight {
			scrollInfo := fmt.Sprintf("Showing %d-%d of %d", startIdx+1, endIdx, len(visible))
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	// Filter and sort status
	b.WriteString("\n")
	var filters []string
	if !m.showCompleted {
		filters = append(filters, "hiding completed")
	}
	if m.filterType != "" {
		filters = append(filters, fmt.Sprintf("type: %s", m.filterType))
	}
	if m.filterStatus != "" {
		filters = append(filters, fmt.Sprintf("status: %s", m.filterStatus))
	}
	if m.filterPriority == 1 {
		filters = append(filters, "priority: high")
	}
	if m.filterTag != "" {
		filters = append(filters, fmt.Sprintf("tag: %s", m.filterTag))
	}
	if m.filterDueDate != DueDateAll {
		var dueName string
		switch m.filterDueDate {
		case DueDateOverdue:
			dueName = "overdue"
		case DueDateToday:
			dueName = "today"
		case DueDateThisWeek:
			dueName = "this week"
		case DueDateNone:
			dueName = "no due date"
		}
		filters = append(filters, fmt.Sprintf("due: %s", dueName))
	}
	if m.searchQuery != "" {
		filters = append(filters, fmt.Sprintf("search: \"%s\"", m.searchQuery))
	}

	// Build status line
	var statusParts []string
	if len(filters) > 0 {
		// Use prominent badge style when filters are active
		filterBadge := FilterBadgeStyle.Render(fmt.Sprintf("[%d filters]", len(filters)))
		filterText := strings.Join(filters, ", ")
		statusParts = append(statusParts, filterBadge+" "+filterText)
	}
	if m.sortField != SortByDefault {
		direction := "↑"
		if !m.sortAscending {
			direction = "↓"
		}
		statusParts = append(statusParts, fmt.Sprintf("Sort: %s %s", m.sortField, direction))
	}
	if len(statusParts) > 0 {
		b.WriteString(strings.Join(statusParts, " | "))
		b.WriteString("\n")
	}

	// Help hint
	b.WriteString(MutedStyle.Render("Press ? for help"))

	return b.String()
}

func (m *ProjectTreeModel) renderNode(node *TreeNode, selected bool) string {
	task := node.Task

	// Build indent
	indent := strings.Repeat(TreeIndent, node.Level)

	// Expand/collapse indicator
	expandIcon := "  "
	if len(task.Children) > 0 {
		if node.Expanded {
			expandIcon = "▼ "
		} else {
			expandIcon = "▶ "
		}
	}

	// Cursor
	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Type and status icons with colors
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	// Override status style if task has incomplete dependencies
	if m.project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	typeIcon := typeStyle.Render(task.Type.Icon())
	statusIcon := statusStyle.Render(task.Status.Icon())

	// Task ID
	idStr := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))

	// Title
	titleStyle := NormalStyle
	if selected {
		titleStyle = BoldStyle.Foreground(ColorPrimary)
	}
	if task.Status == models.StatusCancelled {
		titleStyle = CancelledStyle
	}
	title := titleStyle.Render(truncate(task.Title, m.width-node.Level*2-20))

	// Progress for parent tasks
	progress := ""
	if len(task.Children) > 0 {
		pct := task.GetProgress()
		progress = MutedStyle.Render(fmt.Sprintf(" [%.0f%%]", pct))
	}

	// Priority, overdue, and dependency indicators
	indicators := ""
	if task.Priority <= 2 {
		indicators += lipgloss.NewStyle().Foreground(ColorWarning).Render("!")
	}
	if isOverdue(task) {
		indicators += OverdueStyle.Render("⚠")
	}
	if len(task.DependsOn) > 0 {
		indicators += DependencyStyle.Render(fmt.Sprintf("→%d", len(task.DependsOn)))
	}

	return fmt.Sprintf("%s%s%s%s %s %s %s%s%s",
		cursor, indent, expandIcon, typeIcon, statusIcon, idStr, title, progress, indicators)
}

// GetHelpItems returns the help items for the project tree view.
func (m *ProjectTreeModel) GetHelpItems() []HelpItem {
	archiveDesc := "Archive project"
	if m.project.Archived {
		archiveDesc = "Unarchive project"
	}

	return []HelpItem{
		// Navigation
		{Key: "j/k", Description: "Move cursor", Category: "Navigation"},
		{Key: "Enter", Description: "View task details", Category: "Navigation"},
		{Key: "Space", Description: "Expand/collapse", Category: "Navigation"},
		{Key: "E", Description: "Expand all", Category: "Navigation"},
		{Key: "c", Description: "Collapse all", Category: "Navigation"},

		// Status Changes
		{Key: "s", Description: "Start (in progress)", Category: "Status Changes"},
		{Key: "m", Description: "Mark complete", Category: "Status Changes"},
		{Key: "b", Description: "Block task", Category: "Status Changes"},
		{Key: "x", Description: "Cancel task", Category: "Status Changes"},
		{Key: "o", Description: "Reopen (todo)", Category: "Status Changes"},

		// Type Changes
		{Key: "h", Description: "Promote type", Category: "Type Changes"},
		{Key: "l", Description: "Demote type", Category: "Type Changes"},
		{Key: "H", Description: "Cycle leaf left", Category: "Type Changes"},
		{Key: "L", Description: "Cycle leaf right", Category: "Type Changes"},

		// Task Movement
		{Key: "K", Description: "Move task up", Category: "Task Movement"},
		{Key: "J", Description: "Move task down", Category: "Task Movement"},
		{Key: "→", Description: "Indent (nest under above)", Category: "Task Movement"},
		{Key: "←", Description: "Outdent (to parent level)", Category: "Task Movement"},

		// Task Operations
		{Key: "n", Description: "Add new task", Category: "Task Operations"},
		{Key: "e", Description: "Edit task", Category: "Task Operations"},
		{Key: "ctrl+D", Description: "Set due date", Category: "Task Operations"},
		{Key: "ctrl+O", Description: "Edit in external editor", Category: "Task Operations"},
		{Key: "B", Description: "Manage dependencies", Category: "Task Operations"},
		{Key: "D", Description: "Delete task", Category: "Task Operations"},

		// Filters
		{Key: "/", Description: "Search tasks", Category: "Filters"},
		{Key: "d", Description: "Toggle completed", Category: "Filters"},
		{Key: "P", Description: "Filter by priority", Category: "Filters"},
		{Key: "t", Description: "Filter by type", Category: "Filters"},
		{Key: "S", Description: "Filter by status", Category: "Filters"},
		{Key: "T", Description: "Filter by tag", Category: "Filters"},
		{Key: "u", Description: "Filter by due date", Category: "Filters"},
		{Key: "ctrl+F", Description: "Filter summary", Category: "Filters"},

		// Sort
		{Key: "1", Description: "Sort by priority", Category: "Sort"},
		{Key: "2", Description: "Sort by due date", Category: "Sort"},
		{Key: "3", Description: "Sort by created", Category: "Sort"},
		{Key: "4", Description: "Sort by status", Category: "Sort"},
		{Key: "0", Description: "Reset sort", Category: "Sort"},

		// Project
		{Key: "A", Description: archiveDesc, Category: "Project"},

		// Other
		{Key: "r", Description: "Refresh", Category: "Other"},
		{Key: "R", Description: "View with glow", Category: "Other"},
		{Key: "z", Description: "Undo last change", Category: "Other"},
		{Key: "?", Description: "Toggle help", Category: "Other"},
	}
}
