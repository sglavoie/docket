package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

const taskWorkspaceViewMenuOverlayKey = "task-workspace-view-menu"

// TaskWorkspaceRouteChangedMsg keeps the typed route parameters synchronized
// after a layout or focus change made through the workspace View menu.
type TaskWorkspaceRouteChangedMsg struct{ Params TaskWorkspaceParams }

type taskWorkspaceViewMenuMsg struct{ Open bool }

type taskWorkspaceRowKind uint8

const (
	workspaceTaskRow taskWorkspaceRowKind = iota
	workspaceProjectHeader
	workspaceSectionHeader
	workspaceGroupHeader
)

type taskWorkspaceRow struct {
	kind        taskWorkspaceRowKind
	key         string
	label       string
	item        TaskWorkspaceItem
	depth       int
	hasChildren bool
}

// TaskWorkspaceView is the one live renderer for both app- and project-scoped
// Task Workspace routes. It owns only transient rendering concerns (folds,
// scroll, and menu/search interaction); query, organization, cursor, selection,
// zoom, and route state remain in TaskWorkspaceController.
type TaskWorkspaceView struct {
	controller *TaskWorkspaceController
	width      int
	height     int
	snapshot   TaskWorkspaceSnapshot
	rows       []taskWorkspaceRow
	scroll     int

	collapsedTasks    map[TaskIdentity]bool
	collapsedGroups   map[string]bool
	searchMode        bool
	menuOpen          bool
	menuCursor        int
	columnOffset      int
	boardColumns      []KanbanColumn
	boardActiveColumn int
	boardColumnOffset int
	boardAxis         TaskWorkspaceBoardAxis
	quickEdit         *QuickEditComponent
	quickEditProject  *models.Project
	dispatcher        *keymap.Dispatcher
	resolveProject    func(string) *models.Project
}

type taskWorkspaceOutlineMode struct{ view *TaskWorkspaceView }

func (m taskWorkspaceOutlineMode) WorkspaceLayout() TaskWorkspaceLayout { return WorkspaceOutline }
func (m taskWorkspaceOutlineMode) ApplyWorkspaceSnapshot(state *TaskWorkspaceState, snapshot TaskWorkspaceSnapshot) {
	m.view.applySnapshot(state, snapshot, WorkspaceOutline)
}

type taskWorkspaceListMode struct{ view *TaskWorkspaceView }

func (m taskWorkspaceListMode) WorkspaceLayout() TaskWorkspaceLayout { return WorkspaceList }
func (m taskWorkspaceListMode) ApplyWorkspaceSnapshot(state *TaskWorkspaceState, snapshot TaskWorkspaceSnapshot) {
	m.view.applySnapshot(state, snapshot, WorkspaceList)
}

func NewTaskWorkspaceView(controller *TaskWorkspaceController) (*TaskWorkspaceView, error) {
	if controller == nil || controller.State == nil {
		return nil, fmt.Errorf("task workspace controller is required")
	}
	view := &TaskWorkspaceView{
		controller:      controller,
		width:           80,
		height:          22,
		collapsedTasks:  make(map[TaskIdentity]bool),
		collapsedGroups: make(map[string]bool),
		quickEdit:       NewQuickEditComponent(),
	}
	if err := controller.RegisterMode(taskWorkspaceOutlineMode{view: view}); err != nil {
		return nil, err
	}
	if err := controller.RegisterMode(taskWorkspaceListMode{view: view}); err != nil {
		return nil, err
	}
	if err := controller.RegisterMode(taskWorkspaceBoardMode{view: view}); err != nil {
		return nil, err
	}
	view.rebuildDispatcher()
	return view, nil
}

func (m *TaskWorkspaceView) Init() tea.Cmd {
	if err := m.controller.Refresh(); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	return nil
}

func (m *TaskWorkspaceView) SetSize(width, height int) {
	m.width, m.height = max(width, 1), max(height, 1)
	if m.quickEdit != nil {
		input := m.quickEdit.GetInput()
		input.SetWidth(max(min(m.width-18, 60), 12))
		m.quickEdit.UpdateInput(input)
	}
	m.ensureCursorVisible()
}

func (m *TaskWorkspaceView) Width() int  { return m.width }
func (m *TaskWorkspaceView) Height() int { return m.height }

func (m *TaskWorkspaceView) ApplyWorkspaceSnapshot(state *TaskWorkspaceState, snapshot TaskWorkspaceSnapshot) {
	m.applySnapshot(state, snapshot, state.Layout)
}

func (m *TaskWorkspaceView) applySnapshot(state *TaskWorkspaceState, snapshot TaskWorkspaceSnapshot, layout TaskWorkspaceLayout) {
	m.snapshot = snapshot
	switch layout {
	case WorkspaceOutline:
		m.rows = m.buildOutlineRows(snapshot)
	case WorkspaceList:
		m.rows = m.buildListRows(snapshot)
	case WorkspaceBoard:
		m.rows = nil
		m.applyBoardSnapshot(state, snapshot)
		m.rebuildDispatcher()
		return
	default:
		m.rows = nil
	}
	order := make([]TaskIdentity, 0, len(m.rows))
	for _, row := range m.rows {
		if row.kind == workspaceTaskRow {
			order = append(order, row.item.Identity)
		}
	}
	state.applyPresentationOrder(order)
	m.rebuildDispatcher()
	m.ensureCursorVisible()
}

func (m *TaskWorkspaceView) buildOutlineRows(snapshot TaskWorkspaceSnapshot) []taskWorkspaceRow {
	projectOrder := make([]string, 0)
	byProject := make(map[string][]TaskWorkspaceItem)
	for _, item := range snapshot.Items {
		if _, exists := byProject[item.Identity.ProjectID]; !exists {
			projectOrder = append(projectOrder, item.Identity.ProjectID)
		}
		byProject[item.Identity.ProjectID] = append(byProject[item.Identity.ProjectID], item)
	}

	var rows []taskWorkspaceRow
	showProjects := m.controller.State.Scope.Kind == ScopeApp
	for _, projectID := range projectOrder {
		items := byProject[projectID]
		if len(items) == 0 {
			continue
		}
		if showProjects {
			rows = append(rows, taskWorkspaceRow{
				kind: workspaceProjectHeader, key: "project:" + projectID, label: items[0].Project.Name,
			})
		}

		sectionOrder := make([]string, 0)
		bySection := make(map[string][]TaskWorkspaceItem)
		for _, item := range items {
			section := taskSectionBucket(item.Task)
			if section == "" {
				section = "Active"
			}
			if _, exists := bySection[section]; !exists {
				sectionOrder = append(sectionOrder, section)
			}
			bySection[section] = append(bySection[section], item)
		}
		for _, section := range sectionOrder {
			rows = append(rows, taskWorkspaceRow{
				kind: workspaceSectionHeader, key: "section:" + projectID + ":" + section, label: section,
			})
			rows = append(rows, m.outlineSectionRows(bySection[section])...)
		}
	}
	return rows
}

func (m *TaskWorkspaceView) outlineSectionRows(items []TaskWorkspaceItem) []taskWorkspaceRow {
	byID := make(map[string]TaskWorkspaceItem, len(items))
	children := make(map[string][]TaskWorkspaceItem)
	for _, item := range items {
		byID[item.Identity.TaskID] = item
	}
	for _, item := range items {
		parentID := ""
		if item.Task != nil && item.Task.ParentID != nil {
			parentID = *item.Task.ParentID
		}
		if _, parentVisible := byID[parentID]; parentID == "" || !parentVisible {
			children[""] = append(children[""], item)
		} else {
			children[parentID] = append(children[parentID], item)
		}
	}

	rows := make([]taskWorkspaceRow, 0, len(items))
	visited := make(map[string]bool, len(items))
	var markHidden func(TaskWorkspaceItem)
	markHidden = func(item TaskWorkspaceItem) {
		if visited[item.Identity.TaskID] {
			return
		}
		visited[item.Identity.TaskID] = true
		for _, child := range children[item.Identity.TaskID] {
			markHidden(child)
		}
	}
	var appendTree func(TaskWorkspaceItem, int)
	appendTree = func(item TaskWorkspaceItem, depth int) {
		if visited[item.Identity.TaskID] {
			return
		}
		visited[item.Identity.TaskID] = true
		kids := children[item.Identity.TaskID]
		rows = append(rows, taskWorkspaceRow{
			kind: workspaceTaskRow, item: item, depth: depth, hasChildren: len(kids) > 0,
		})
		if m.collapsedTasks[item.Identity] {
			for _, child := range kids {
				markHidden(child)
			}
			return
		}
		for _, child := range kids {
			appendTree(child, depth+1)
		}
	}
	for _, root := range children[""] {
		appendTree(root, 0)
	}
	// Malformed/cyclic parent data must remain visible rather than disappearing.
	for _, item := range items {
		if !visited[item.Identity.TaskID] {
			appendTree(item, 0)
		}
	}
	return rows
}

func (m *TaskWorkspaceView) buildListRows(snapshot TaskWorkspaceSnapshot) []taskWorkspaceRow {
	var rows []taskWorkspaceRow
	grouped := m.controller.State.Grouping != WorkspaceGroupNone
	for _, group := range snapshot.Groups {
		if grouped {
			rows = append(rows, taskWorkspaceRow{
				kind: workspaceGroupHeader, key: group.Key, label: fmt.Sprintf("%s (%d)", group.Label, len(group.Items)),
			})
		}
		if m.collapsedGroups[group.Key] {
			continue
		}
		for _, item := range group.Items {
			rows = append(rows, taskWorkspaceRow{kind: workspaceTaskRow, item: item})
		}
	}
	return rows
}

func (m *TaskWorkspaceView) rebuildDispatcher() {
	capabilities := m.controller.State.Capabilities(m.snapshot)
	km := BuildTaskWorkspaceKeyMap(m, capabilities, m.handleAction)
	m.dispatcher = keymap.NewDispatcher(km)
}

func (m *TaskWorkspaceView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		if m.quickEdit != nil && m.quickEdit.IsActive() {
			return m, m.handleWorkspaceQuickEditKey(msg)
		}
		if m.menuOpen {
			return m, m.handleMenuKey(msg)
		}
		if m.searchMode {
			return m, m.handleSearchKey(msg)
		}
		if msg.String() == "esc" && m.controller.State.SearchQuery != "" {
			m.ClearSearch()
			return m, nil
		}
		if msg.String() == "esc" && len(m.controller.State.Selected()) > 0 {
			m.controller.State.ClearSelection()
			m.rebuildDispatcher()
			return m, nil
		}
		if m.dispatcher != nil {
			_, cmd, handled := m.dispatcher.Dispatch(msg)
			if handled {
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m *TaskWorkspaceView) handleAction(action TaskWorkspaceAction) tea.Cmd { //nolint:gocyclo
	state := m.controller.State
	switch action { //nolint:exhaustive // Planner tab actions never reach Task Workspace.
	case WorkspaceActionCursorDown:
		if state.Layout == WorkspaceBoard {
			m.moveBoardCursor(1)
		} else {
			state.MoveCursor(1)
		}
	case WorkspaceActionCursorUp:
		if state.Layout == WorkspaceBoard {
			m.moveBoardCursor(-1)
		} else {
			state.MoveCursor(-1)
		}
	case WorkspaceActionJumpFirst:
		if state.Layout == WorkspaceBoard {
			m.jumpBoardCursor(false)
		} else {
			state.MoveCursor(-len(m.rows) - 1)
		}
	case WorkspaceActionJumpLast:
		if state.Layout == WorkspaceBoard {
			m.jumpBoardCursor(true)
		} else {
			state.MoveCursor(len(m.rows) + 1)
		}
	case WorkspaceActionPageUp:
		if state.Layout == WorkspaceBoard {
			m.moveBoardCursor(-max(m.boardBodyRows(), 1))
		} else {
			state.MoveCursor(-max(m.contentHeight(), 1))
		}
	case WorkspaceActionPageDown:
		if state.Layout == WorkspaceBoard {
			m.moveBoardCursor(max(m.boardBodyRows(), 1))
		} else {
			state.MoveCursor(max(m.contentHeight(), 1))
		}
	case WorkspaceActionLayoutLeft:
		if state.Layout == WorkspaceBoard {
			m.moveBoardColumn(-1)
		} else {
			m.collapseCurrent()
		}
	case WorkspaceActionLayoutRight:
		if state.Layout == WorkspaceBoard {
			m.moveBoardColumn(1)
		} else {
			m.expandCurrent()
		}
	case WorkspaceActionNextGroup:
		if state.Layout == WorkspaceBoard {
			m.moveBoardColumn(1)
		} else {
			m.moveToGroup(1)
		}
	case WorkspaceActionPreviousGroup:
		if state.Layout == WorkspaceBoard {
			m.moveBoardColumn(-1)
		} else {
			m.moveToGroup(-1)
		}
	case WorkspaceActionZoom:
		if id, ok := state.Cursor(); ok && state.SetZoomRoot(id) {
			return m.refreshCmd()
		}
	case WorkspaceActionZoomOut:
		if _, ok := state.ZoomRoot(); ok {
			state.ClearZoom()
			return m.refreshCmd()
		}
	case WorkspaceActionToggleSidebar:
		state.SidebarVisible = !state.SidebarVisible
	case WorkspaceActionOpenDetail:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return ShowTaskDetailMsg{Task: item.Task, Project: item.Project}
		})
	case WorkspaceActionOpenPeek:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return ShowTaskDetailMsg{Task: item.Task, Project: item.Project}
		})
	case WorkspaceActionSelect:
		if id, ok := state.Cursor(); ok {
			state.ToggleSelection(id)
			m.rebuildDispatcher()
		}
	case WorkspaceActionSearch:
		m.searchMode = true
	case WorkspaceActionViewMenu:
		m.menuOpen, m.menuCursor = true, 0
		return func() tea.Msg { return taskWorkspaceViewMenuMsg{Open: true} }
	case WorkspaceActionAdd:
		if cmd := m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return CreateTaskMsg{Project: item.Project}
		}); cmd != nil {
			return cmd
		}
		var project *models.Project
		if state.Scope.Kind == ScopeProject && m.resolveProject != nil {
			project = m.resolveProject(state.Scope.ProjectID)
		}
		return func() tea.Msg { return ShowQuickCaptureMsg{Project: project} }
	case WorkspaceActionEdit:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return EditTaskMsg{Task: item.Task, Project: item.Project}
		})
	case WorkspaceActionComplete:
		return m.controller.Transition(models.StateDone)
	case WorkspaceActionDelete:
		refs := m.selectedTaskRefs()
		if len(refs) == 0 {
			return nil
		}
		origin := taskWorkspacePresentation(m.controller.State.RouteParams())
		return func() tea.Msg { return TaskDeleteRequestedMsg{Refs: refs, Origin: origin} }
	case WorkspaceActionState:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return ShowSectionPickerMsg{Task: item.Task, Project: item.Project}
		})
	case WorkspaceActionPin:
		if item, ok := m.currentItem(); ok {
			return m.controller.SetPinned(!item.Task.Pinned)
		}
	case WorkspaceActionPriority:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return ShowPriorityPickerMsg{Task: item.Task, Project: item.Project}
		})
	case WorkspaceActionDueDate:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return SetDueDateMsg{Task: item.Task, Project: item.Project, TaskRefs: m.selectedTaskRefs()}
		})
	case WorkspaceActionTags:
		refs := m.selectedTaskRefs()
		if len(refs) > 1 {
			return func() tea.Msg { return ShowBulkTagSelectorMsg{TaskRefs: refs} }
		}
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return ShowTagSelectorMsg{Task: item.Task, Project: item.Project, CurrentTags: slices.Clone(item.Task.Tags)}
		})
	case WorkspaceActionSnooze:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return SetSnoozeDateMsg{Task: item.Task, Project: item.Project, TaskRefs: m.selectedTaskRefs()}
		})
	case WorkspaceActionMove:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return ShowProjectSelectorForMoveMsg{Task: item.Task, SourceProject: item.Project}
		})
	case WorkspaceActionMoveDown:
		if state.Layout == WorkspaceBoard {
			return m.moveBoardCard(1)
		}
		return m.controller.MoveDown()
	case WorkspaceActionMoveUp:
		if state.Layout == WorkspaceBoard {
			return m.moveBoardCard(-1)
		}
		return m.controller.MoveUp()
	case WorkspaceActionPark:
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return ShowParkInputMsg{Task: item.Task, Project: item.Project}
		})
	case WorkspaceActionCopy:
		tasks := m.getMultiSelectedTasks()
		if len(tasks) > 1 {
			return func() tea.Msg { return CopyTasksTitleMsg{Tasks: tasks} }
		}
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg { return CopyTaskTitleMsg{Task: item.Task} })
	case WorkspaceActionUndo, WorkspaceActionRedo:
		// The AppModel owns durable workspace-ledger undo/redo and intercepts
		// these bindings before delegation in production.
	}
	m.ensureCursorVisible()
	return nil
}

func (m *TaskWorkspaceView) refreshCmd() tea.Cmd {
	if err := m.controller.Refresh(); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	return nil
}

func (m *TaskWorkspaceView) itemCommand(build func(TaskWorkspaceItem) tea.Msg) tea.Cmd {
	item, ok := m.currentItem()
	if !ok || build == nil {
		return nil
	}
	return func() tea.Msg { return build(item) }
}

func (m *TaskWorkspaceView) currentItem() (TaskWorkspaceItem, bool) {
	id, ok := m.controller.State.Cursor()
	if !ok {
		return TaskWorkspaceItem{}, false
	}
	return m.snapshot.Item(id)
}

func (m *TaskWorkspaceView) selectedTaskRefs() []TaskProjectRef {
	ids := m.controller.State.Selected()
	if len(ids) == 0 {
		if id, ok := m.controller.State.Cursor(); ok {
			ids = []TaskIdentity{id}
		}
	}
	refs := make([]TaskProjectRef, 0, len(ids))
	for _, id := range ids {
		if item, ok := m.snapshot.Item(id); ok {
			refs = append(refs, TaskProjectRef{Task: item.Task, Project: item.Project})
		}
	}
	return refs
}

func (m *TaskWorkspaceView) collapseCurrent() {
	if m.controller.State.Layout == WorkspaceList {
		m.columnOffset = max(m.columnOffset-1, 0)
		return
	}
	if row, ok := m.currentTaskRow(); ok && row.hasChildren {
		m.collapsedTasks[row.item.Identity] = true
		m.applySnapshot(m.controller.State, m.snapshot, WorkspaceOutline)
	}
}

func (m *TaskWorkspaceView) expandCurrent() {
	if m.controller.State.Layout == WorkspaceList {
		m.columnOffset = min(m.columnOffset+1, max(len(m.controller.State.Columns)-1, 0))
		return
	}
	if row, ok := m.currentTaskRow(); ok && row.hasChildren {
		delete(m.collapsedTasks, row.item.Identity)
		m.applySnapshot(m.controller.State, m.snapshot, WorkspaceOutline)
	}
}

func (m *TaskWorkspaceView) currentTaskRow() (taskWorkspaceRow, bool) {
	id, ok := m.controller.State.Cursor()
	if !ok {
		return taskWorkspaceRow{}, false
	}
	for _, row := range m.rows {
		if row.kind == workspaceTaskRow && row.item.Identity == id {
			return row, true
		}
	}
	return taskWorkspaceRow{}, false
}

func (m *TaskWorkspaceView) moveToGroup(delta int) {
	if delta == 0 {
		return
	}
	current, _ := m.controller.State.Cursor()
	starts := make([]TaskIdentity, 0)
	for i, row := range m.rows {
		if row.kind != workspaceProjectHeader && row.kind != workspaceSectionHeader && row.kind != workspaceGroupHeader {
			continue
		}
		for j := i + 1; j < len(m.rows); j++ {
			if m.rows[j].kind == workspaceTaskRow {
				starts = append(starts, m.rows[j].item.Identity)
				break
			}
		}
	}
	starts = slices.Compact(starts)
	if len(starts) == 0 {
		return
	}
	idx := slices.Index(starts, current)
	if idx < 0 {
		for i, id := range starts {
			if m.rowIndex(id) > m.rowIndex(current) {
				idx = i - 1
				break
			}
		}
	}
	idx = max(0, min(len(starts)-1, idx+delta))
	m.controller.State.SetCursor(starts[idx])
	m.ensureCursorVisible()
}

func (m *TaskWorkspaceView) rowIndex(id TaskIdentity) int {
	for i, row := range m.rows {
		if row.kind == workspaceTaskRow && row.item.Identity == id {
			return i
		}
	}
	return -1
}

func (m *TaskWorkspaceView) handleSearchKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.controller.State.SetSearchQuery("")
		return m.refreshCmd()
	case "enter":
		m.searchMode = false
		return nil
	case "backspace":
		query := []rune(m.controller.State.SearchQuery)
		if len(query) > 0 {
			m.controller.State.SetSearchQuery(string(query[:len(query)-1]))
			return m.refreshCmd()
		}
		return nil
	}
	text := msg.Text
	if text == "" && msg.Mod == 0 && unicode.IsPrint(msg.Code) {
		text = string(msg.Code)
	}
	if text != "" && msg.Mod == 0 {
		m.controller.State.SetSearchQuery(m.controller.State.SearchQuery + text)
		return m.refreshCmd()
	}
	return nil
}

func (m *TaskWorkspaceView) handleWorkspaceQuickEditKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.quickEdit == nil || !m.quickEdit.IsActive() {
		return nil
	}
	switch msg.String() {
	case "esc":
		m.quickEdit.Exit()
		m.quickEditProject = nil
		return nil
	case "enter":
		input := strings.TrimSpace(m.quickEdit.GetInput().Value())
		original := m.quickEdit.GetTask()
		project := m.quickEditProject
		if input == "" || original == nil || project == nil || project.GetTaskByID(original.ID) == nil {
			m.quickEdit.Exit()
			m.quickEditProject = nil
			return nil
		}
		result, err := parseQuickEdit(input, original, project)
		if err != nil {
			return nil
		}
		// Parse into isolated copies. The application service is the only
		// writer; the workspace snapshot changes after the transaction reload.
		edited := *original
		edited.Tags = slices.Clone(original.Tags)
		projectCopy := *project
		projectCopy.Tasks = []*models.Task{&edited}
		if statusErr := applyQuickEdit(&edited, &projectCopy, result); statusErr != "" {
			m.quickEdit.Exit()
			m.quickEditProject = nil
			return func() tea.Msg { return StatusMsg(statusErr) }
		}
		m.quickEdit.Exit()
		m.quickEditProject = nil
		state := edited.State()
		return m.controller.Edit(app.EditRequest{
			TaskID: edited.ID, Form: formEditFromTask(&edited), WorkflowState: &state,
		})
	default:
		inputModel := m.quickEdit.GetInput()
		inputModel, cmd := inputModel.Update(msg)
		m.quickEdit.UpdateInput(inputModel)
		preview, _ := parseQuickEditForPreview(inputModel.Value(), m.quickEdit.GetTask(), m.quickEditProject)
		m.quickEdit.SetPreview(preview)
		return cmd
	}
}

func (m *TaskWorkspaceView) workspaceMenuItems() []TaskWorkspaceMenuItem {
	items := slices.DeleteFunc(m.controller.ViewMenu(), func(item TaskWorkspaceMenuItem) bool {
		return item.Apply == nil && item.ID != "organize:advanced-filters"
	})
	groupings := []struct {
		id    string
		label string
		value TaskWorkspaceGroupBy
	}{
		{"none", "No grouping", WorkspaceGroupNone},
		{"project", "Group by project", WorkspaceGroupProject},
		{"status", "Group by status", WorkspaceGroupStatus},
		{"section", "Group by section", WorkspaceGroupSection},
		{"priority", "Group by priority", WorkspaceGroupPriority},
		{"due", "Group by due date", WorkspaceGroupDueDate},
	}
	for _, option := range groupings {
		option := option
		items = append(items, TaskWorkspaceMenuItem{
			ID: "organize:group:" + option.id, Label: option.label, Section: WorkspaceMenuOrganize,
			Active: func(state *TaskWorkspaceState) bool { return state.Grouping == option.value },
			Apply:  func(state *TaskWorkspaceState) error { return state.SetGrouping(option.value) },
		})
	}
	sorts := []struct {
		id    string
		label string
		field SortField
	}{
		{"default", "Sort by model order", SortByDefault},
		{"priority", "Sort by priority", SortByPriority},
		{"due", "Sort by due date", SortByDueDate},
		{"created", "Sort by created", SortByCreatedAt},
		{"status", "Sort by status", SortByStatus},
		{"score", "Sort by score", SortByScore},
	}
	for _, option := range sorts {
		option := option
		items = append(items, TaskWorkspaceMenuItem{
			ID: "organize:sort:" + option.id, Label: option.label, Section: WorkspaceMenuOrganize,
			Active: func(state *TaskWorkspaceState) bool { return state.Sorting.Field == option.field },
			Apply: func(state *TaskWorkspaceState) error {
				return state.SetSorting(SortState{Field: option.field, Ascending: true})
			},
		})
	}
	for _, column := range allTaskWorkspaceColumns {
		if column == WorkspaceColumnTitle {
			continue
		}
		column := column
		items = append(items, TaskWorkspaceMenuItem{
			ID: "organize:column:" + string(column), Label: "Column: " + string(column), Section: WorkspaceMenuOrganize,
			Active: func(state *TaskWorkspaceState) bool { return slices.Contains(state.Columns, column) },
			Apply: func(state *TaskWorkspaceState) error {
				columns := slices.Clone(state.Columns)
				if idx := slices.Index(columns, column); idx >= 0 {
					columns = slices.Delete(columns, idx, idx+1)
				} else {
					columns = append(columns, column)
				}
				return state.SetColumns(columns)
			},
		})
	}
	return items
}

func (m *TaskWorkspaceView) handleMenuKey(msg tea.KeyPressMsg) tea.Cmd {
	items := m.workspaceMenuItems()
	switch msg.String() {
	case "esc", "v":
		m.menuOpen = false
		return func() tea.Msg { return taskWorkspaceViewMenuMsg{Open: false} }
	case "j", "down":
		m.menuCursor = min(m.menuCursor+1, max(len(items)-1, 0))
	case "k", "up":
		m.menuCursor = max(m.menuCursor-1, 0)
	case "enter":
		if m.menuCursor < 0 || m.menuCursor >= len(items) {
			return nil
		}
		if items[m.menuCursor].ID == "organize:advanced-filters" {
			m.menuOpen = false
			source := ViewCrossProjectOutliner
			if m.controller.State.Scope.Kind == ScopeProject {
				source = ViewOutliner
			}
			return tea.Batch(
				func() tea.Msg { return taskWorkspaceViewMenuMsg{Open: false} },
				func() tea.Msg { return ShowAttrFilterPickerMsg{Source: source} },
			)
		}
		before := m.controller.State.RouteParams()
		if err := items[m.menuCursor].Apply(m.controller.State); err != nil {
			return func() tea.Msg { return ErrorMsg{Err: err} }
		}
		if err := m.controller.Refresh(); err != nil {
			return func() tea.Msg { return ErrorMsg{Err: err} }
		}
		m.menuOpen = false
		after := m.controller.State.RouteParams()
		return tea.Batch(
			func() tea.Msg { return taskWorkspaceViewMenuMsg{Open: false} },
			func() tea.Msg {
				if before != after {
					return TaskWorkspaceRouteChangedMsg{Params: after}
				}
				return nil
			},
		)
	}
	return nil
}

func (m *TaskWorkspaceView) View() tea.View {
	state := m.controller.State
	name := "Tasks"
	if state.Scope.Kind == ScopeProject {
		if item, ok := m.anyItem(); ok && item.Project != nil {
			name = item.Project.Name
		} else {
			name = "Project"
		}
	}
	layout := "Outline"
	switch state.Layout {
	case WorkspaceOutline:
	case WorkspaceList:
		layout = "List"
	case WorkspaceBoard:
		layout = "Board · " + workspaceBoardAxisLabel(state.BoardAxis)
	}
	var b strings.Builder
	b.WriteString(renderFullscreenTitle(name+" · "+layout, fmt.Sprintf("%s · %d tasks", state.Focus, len(m.snapshot.Items)), ColorPrimary))
	b.WriteString("\n")
	if breadcrumb := m.zoomBreadcrumb(); breadcrumb != "" {
		b.WriteString(MutedStyle.Render("  "+breadcrumb) + "\n")
	}
	if m.searchMode || state.SearchQuery != "" {
		cursor := ""
		if m.searchMode {
			cursor = "▌"
		}
		b.WriteString(InfoStatusStyle.Render("  / "+state.SearchQuery+cursor) + "\n")
	}
	if len(m.snapshot.Items) == 0 {
		b.WriteString("\n" + renderEmptyState("No tasks match this workspace.", "v change focus · / search · a add"))
		return tea.NewView(b.String())
	}
	if state.Layout == WorkspaceBoard {
		b.WriteString(m.renderBoard())
		return tea.NewView(b.String())
	}
	lines := m.renderRows()
	if len(lines) == 0 {
		b.WriteString("\n" + renderEmptyState("All tasks in this group are folded.", "l expand · v change view"))
		return tea.NewView(b.String())
	}
	start, end := m.visibleWindow(len(lines))
	b.WriteString(strings.Join(lines[start:end], "\n"))
	if quickEdit := m.renderWorkspaceQuickEditBar(); quickEdit != "" {
		b.WriteString("\n" + quickEdit)
	}
	return tea.NewView(b.String())
}

func (m *TaskWorkspaceView) renderWorkspaceQuickEditBar() string {
	if m.quickEdit == nil || !m.quickEdit.IsActive() {
		return ""
	}
	lines := []string{TitleStyle.Render("Quick Edit:") + " " + m.quickEdit.GetInput().View()}
	if preview := renderQuickEditPreview(m.quickEdit.GetPreview(), m.quickEdit.GetOriginal()); preview != "" {
		lines = append(lines, MutedStyle.Render(preview))
	}
	lines = append(lines, MutedStyle.Render("!1-5 priority · #tag · @date · /section · $status · enter save · esc cancel"))
	for i := range lines {
		lines[i] = truncateToVisualWidth(lines[i], max(m.width, 1))
	}
	return strings.Join(lines, "\n")
}

func (m *TaskWorkspaceView) renderRows() []string {
	lines := make([]string, 0, len(m.rows))
	cursor, _ := m.controller.State.Cursor()
	selected := make(map[TaskIdentity]bool)
	for _, id := range m.controller.State.Selected() {
		selected[id] = true
	}
	for _, row := range m.rows {
		switch row.kind {
		case workspaceProjectHeader:
			lines = append(lines, BoldStyle.Render(truncateToVisualWidth("▰ "+row.label, m.width)))
		case workspaceSectionHeader:
			lines = append(lines, MutedStyle.Render(truncateToVisualWidth("  "+row.label, m.width)))
		case workspaceGroupHeader:
			marker := "▾ "
			if m.collapsedGroups[row.key] {
				marker = "▸ "
			}
			lines = append(lines, BoldStyle.Render(truncateToVisualWidth(marker+row.label, m.width)))
		case workspaceTaskRow:
			line := m.renderTaskRow(row, cursor == row.item.Identity, selected[row.item.Identity])
			lines = append(lines, line)
		}
	}
	return lines
}

func (m *TaskWorkspaceView) renderTaskRow(row taskWorkspaceRow, current, checked bool) string {
	marker := "  "
	if current {
		marker = "› "
	}
	check := "  "
	if checked {
		check = "✓ "
	}
	prefix := ""
	if m.controller.State.Layout == WorkspaceOutline {
		prefix = strings.Repeat("  ", row.depth)
		if row.hasChildren {
			if m.collapsedTasks[row.item.Identity] {
				prefix += "▸ "
			} else {
				prefix += "▾ "
			}
		} else {
			prefix += "  "
		}
	}
	line := marker + check + prefix + row.item.Task.Title
	if m.controller.State.Layout == WorkspaceList && m.width >= 72 {
		columns := m.controller.State.Columns
		if m.columnOffset < len(columns) {
			columns = columns[m.columnOffset:]
		}
		for _, column := range columns {
			switch column { //nolint:exhaustive // Title is already rendered; Score is represented by sorting/focus metadata.
			case WorkspaceColumnProject:
				line += "  " + row.item.Project.Name
			case WorkspaceColumnStatus:
				line += "  " + row.item.Task.State().String()
			case WorkspaceColumnPriority:
				line += fmt.Sprintf("  P%d", row.item.Task.Priority)
			case WorkspaceColumnDueDate:
				if row.item.Task.DueDate != nil {
					line += "  " + row.item.Task.DueDate.Format("2006-01-02")
				}
			}
		}
	}
	line = truncateToVisualWidth(line, max(m.width, 1))
	if current {
		return SelectedTaskTitleStyle.Render(line)
	}
	if checked {
		return SelectedCheckboxStyle.Render(line)
	}
	return line
}

func (m *TaskWorkspaceView) anyItem() (TaskWorkspaceItem, bool) {
	if item, ok := m.currentItem(); ok {
		return item, true
	}
	if len(m.snapshot.Items) > 0 {
		return m.snapshot.Items[0], true
	}
	return TaskWorkspaceItem{}, false
}

func (m *TaskWorkspaceView) zoomBreadcrumb() string {
	root, ok := m.controller.State.ZoomRoot()
	if !ok {
		return ""
	}
	item, ok := m.snapshot.Item(root)
	if !ok || item.Project == nil || item.Task == nil {
		return ""
	}
	segments := []string{item.Task.Title}
	parentID := item.Task.ParentID
	seen := map[string]bool{item.Task.ID: true}
	for parentID != nil && *parentID != "" && !seen[*parentID] {
		seen[*parentID] = true
		parent := item.Project.GetTaskByID(*parentID)
		if parent == nil {
			break
		}
		segments = append([]string{parent.Title}, segments...)
		parentID = parent.ParentID
	}
	segments = append([]string{item.Project.Name}, segments...)
	return truncateToVisualWidth(strings.Join(segments, " > "), max(m.width-4, 1))
}

func (m *TaskWorkspaceView) contentHeight() int {
	chrome := 2
	if _, ok := m.controller.State.ZoomRoot(); ok {
		chrome++
	}
	if m.searchMode || m.controller.State.SearchQuery != "" {
		chrome++
	}
	if m.quickEdit != nil && m.quickEdit.IsActive() {
		chrome += 3
	}
	return max(m.height-chrome, 1)
}

func (m *TaskWorkspaceView) visibleWindow(total int) (int, int) {
	limit := min(m.contentHeight(), total)
	m.ensureCursorVisible()
	start := min(max(m.scroll, 0), max(total-limit, 0))
	return start, min(start+limit, total)
}

func (m *TaskWorkspaceView) ensureCursorVisible() {
	if m.controller.State.Layout == WorkspaceBoard {
		m.ensureBoardCursorVisible()
		return
	}
	id, ok := m.controller.State.Cursor()
	if !ok {
		m.scroll = 0
		return
	}
	idx := m.rowIndex(id)
	if idx < 0 {
		return
	}
	height := max(m.contentHeight(), 1)
	if idx < m.scroll {
		m.scroll = idx
	} else if idx >= m.scroll+height {
		m.scroll = idx - height + 1
	}
	m.scroll = max(m.scroll, 0)
}

func (m *TaskWorkspaceView) renderViewMenu(width int) string {
	if !m.menuOpen {
		return ""
	}
	items := m.workspaceMenuItems()
	if len(items) == 0 {
		return ""
	}
	start := max(0, m.menuCursor-8)
	end := min(len(items), start+17)
	var lines []string
	lastSection := TaskWorkspaceMenuSection("")
	for i := start; i < end; i++ {
		item := items[i]
		if item.Section != lastSection {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, MutedStyle.Render(string(item.Section)))
			lastSection = item.Section
		}
		marker := "  "
		if i == m.menuCursor {
			marker = "› "
		}
		active := "  "
		if item.IsActive(m.controller.State) {
			active = "✓ "
		}
		line := marker + active + item.Label
		if i == m.menuCursor {
			line = SelectedTaskTitleStyle.Render(line)
		}
		lines = append(lines, line)
	}
	innerWidth := min(max(width-8, 28), 54)
	return renderTitledBox(strings.Join(lines, "\n"), innerWidth, "View", "↑/↓ choose · enter apply · esc close", lipgloss.NewStyle().Foreground(ColorPrimary))
}

func (m *TaskWorkspaceView) getTaskRef() (*models.Task, *models.Project) {
	item, ok := m.currentItem()
	if !ok {
		return nil, nil
	}
	return item.Task, item.Project
}

func (m *TaskWorkspaceView) getCursorTask() *models.Task {
	task, _ := m.getTaskRef()
	return task
}

func (m *TaskWorkspaceView) getMultiSelectedTasks() []*models.Task {
	refs := m.selectedTaskRefs()
	if len(m.controller.State.Selected()) == 0 {
		return nil
	}
	tasks := make([]*models.Task, 0, len(refs))
	for _, ref := range refs {
		tasks = append(tasks, ref.Task)
	}
	return tasks
}

func (m *TaskWorkspaceView) clearMultiSelect() { m.controller.State.ClearSelection() }

func (m *TaskWorkspaceView) IsMultiSelectActive() bool { return len(m.controller.State.Selected()) > 0 }
func (m *TaskWorkspaceView) HasActiveSearch() bool     { return m.controller.State.SearchQuery != "" }
func (m *TaskWorkspaceView) ClearSearch() {
	m.searchMode = false
	m.controller.State.SetSearchQuery("")
	_ = m.controller.Refresh()
}
func (m *TaskWorkspaceView) ShouldCaptureEsc() bool {
	return m.menuOpen || m.searchMode || m.controller.State.SearchQuery != "" ||
		len(m.controller.State.Selected()) > 0 || (m.quickEdit != nil && m.quickEdit.IsActive())
}
func (m *TaskWorkspaceView) HasTextInput() bool {
	return m.searchMode || (m.quickEdit != nil && m.quickEdit.IsActive())
}
func (m *TaskWorkspaceView) IsFullHeight() bool { return false }
func (m *TaskWorkspaceView) ActiveFilterCount() int {
	if m.controller.State.SearchQuery != "" {
		return 1
	}
	return 0
}
func (m *TaskWorkspaceView) StatusBarLabel() string {
	return fmt.Sprintf("Task Workspace · %s", m.controller.State.Focus)
}
func (m *TaskWorkspaceView) SupportsHelpShortcut() bool { return true }

// CommandContext exposes the live workspace's typed command context so the
// palette enables the same capability-scoped grammar that direct keys and help
// advertise. The presentation capability and View are merged in so the mature
// renderer's presentation-scoped commands (e.g. Outliner task operations) stay
// reachable alongside the typed Task Workspace catalog.
func (m *TaskWorkspaceView) CommandContext() CommandContext {
	ctx := m.controller.State.CommandContext(m.snapshot)
	presentation := taskWorkspacePresentation(m.controller.State.RouteParams())
	ctx.View = presentation
	ctx.Capabilities.Add(presentationCapability(presentation))
	return ctx
}
func (m *TaskWorkspaceView) ContextualFooter() (StatusBarFooterSpec, bool) {
	left := "j/k move · enter detail · space select · X delete · / search · v view · ? help"
	if m.controller.State.Layout == WorkspaceBoard {
		left = "j/k cards · h/l columns · K/J move card · X delete · v view · ? help"
	}
	return StatusBarFooterSpec{
		Left:  left,
		Right: fmt.Sprintf("%d tasks", len(m.snapshot.Items)),
	}, true
}

func (m *TaskWorkspaceView) GetHelpItems() []HelpItem {
	bindings := TaskWorkspaceBindings(m.controller.State.Capabilities(m.snapshot))
	items := make([]HelpItem, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, HelpItem{Key: binding.Key, Description: binding.Description, Category: binding.Category})
	}
	return items
}
